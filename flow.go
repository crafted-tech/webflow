package webflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/crafted-tech/webframe"
	"github.com/crafted-tech/webframe/types"
)

// Fallback frame colors when GetHeaderBarColor is not available
var (
	darkFrameColorFallback          = types.RGBA{R: 0x1C, G: 0x1F, B: 0x26, A: 0xFF}
	lightFrameColorFallback         = types.RGBA{R: 0xF4, G: 0xF5, B: 0xF7, A: 0xFF}
	darkBackdropFrameColorFallback  = types.RGBA{R: 0x18, G: 0x1A, B: 0x20, A: 0xFF}
	lightBackdropFrameColorFallback = types.RGBA{R: 0xE8, G: 0xE9, B: 0xEB, A: 0xFF}
)

// debugLog writes to a crash log for debugging
func debugLog(msg string) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	logDir := filepath.Join(cacheDir, "UnisonWebView", "logs")
	os.MkdirAll(logDir, 0755)
	f, err := os.OpenFile(filepath.Join(logDir, "crash.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[FLOW] %s\n", msg)
}

// Flow manages the wizard UI, displaying pages and collecting user responses.
type Flow struct {
	wv         types.WebFrame
	config     Config
	responseCh chan messageResponse
	darkMode   bool
	mu         sync.Mutex
	quitOnMsg  bool // Whether to quit the event loop when a message is received

	// Progress control
	progressCancelled atomic.Bool
}

// messageResponse represents a message received from JavaScript.
type messageResponse struct {
	Type   string         `json:"type"`
	Button string         `json:"button"`
	Data   map[string]any `json:"data"`
}

// New creates a new Flow with the given options.
// The Flow manages a window for displaying wizard-like UIs.
func New(opts ...Option) (*Flow, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	f := &Flow{
		config:     cfg,
		responseCh: make(chan messageResponse, 1),
	}

	// Create webview
	resizable := cfg.Resizable == nil || *cfg.Resizable // nil or true = resizable
	nativeTitleBar := cfg.NativeTitleBar != nil && *cfg.NativeTitleBar // nil or false = stylable titlebar
	wvConfig := types.Config{
		Title:          cfg.Title,
		Width:          cfg.Width,
		Height:         cfg.Height,
		Resizable:      resizable,
		NativeTitleBar: nativeTitleBar,
		StartHidden:    true,
		OnClose: func() {
			// Send a close message when window X button is clicked
			select {
			case f.responseCh <- messageResponse{Type: "window_close", Button: "close"}:
			default:
			}
		},
	}

	wv, err := webframe.New(wvConfig)
	if err != nil {
		return nil, err
	}

	f.wv = wv

	// Determine dark mode based on theme setting
	switch {
	case cfg.Theme == nil, *cfg.Theme == ThemeSystem:
		// Auto-detect from OS
		osIsDark := wv.IsDarkMode()
		f.darkMode = osIsDark
		fmt.Printf("[webflow] Theme: system (auto-detect), wv.IsDarkMode()=%v, using darkMode=%v\n", osIsDark, f.darkMode)
	case *cfg.Theme == ThemeDark:
		f.darkMode = true
		fmt.Printf("[webflow] Theme: forced dark, using darkMode=%v\n", f.darkMode)
	case *cfg.Theme == ThemeLight:
		f.darkMode = false
		fmt.Printf("[webflow] Theme: forced light, using darkMode=%v\n", f.darkMode)
	}

	// Set initial frame appearance using system headerbar colors (active and backdrop)
	frameColor := wv.GetHeaderBarColor()
	backdropFrameColor := wv.GetBackdropHeaderBarColor()
	fmt.Printf("[webflow] Setting frame appearance to system colors (active=#%02x%02x%02x, backdrop=#%02x%02x%02x)\n",
		frameColor.R, frameColor.G, frameColor.B, backdropFrameColor.R, backdropFrameColor.G, backdropFrameColor.B)
	wv.SetFrameAppearance(types.FrameAppearance{TitleBar: frameColor, BackdropTitleBar: backdropFrameColor})

	// Register for OS theme changes (only when using system theme)
	if cfg.Theme == nil || *cfg.Theme == ThemeSystem {
		wv.OnThemeChange(func(isDark bool) {
			fmt.Printf("[webflow] OS theme changed: isDark=%v\n", isDark)
			f.darkMode = isDark

			// Update window frame decorations using system colors (active and backdrop)
			newFrameColor := f.wv.GetHeaderBarColor()
			newBackdropFrameColor := f.wv.GetBackdropHeaderBarColor()
			fmt.Printf("[webflow] Updating frame to system colors (active=#%02x%02x%02x, backdrop=#%02x%02x%02x)\n",
				newFrameColor.R, newFrameColor.G, newFrameColor.B, newBackdropFrameColor.R, newBackdropFrameColor.G, newBackdropFrameColor.B)
			f.wv.SetFrameAppearance(types.FrameAppearance{TitleBar: newFrameColor, BackdropTitleBar: newBackdropFrameColor})

			// Update CSS class instantly
			if f.darkMode {
				f.wv.EvaluateScript(`document.documentElement.setAttribute('data-theme', 'dark')`)
			} else {
				f.wv.EvaluateScript(`document.documentElement.setAttribute('data-theme', 'light')`)
			}
		})
	}

	// Set up message handler
	wv.AddMessageHandler(func(message string) {
		var resp messageResponse
		if err := json.Unmarshal([]byte(message), &resp); err != nil {
			return
		}

		// Handle special message types that don't trigger page navigation
		if resp.Type == "page_ready" {
			// Page has loaded, focus the WebView content
			if focuser, ok := f.wv.(webviewFocuser); ok {
				focuser.FocusWebView()
			}
			return
		}

		if resp.Type == "browse_folder" {
			debugLog("browse_folder message received")
			debugLog(fmt.Sprintf("f.wv type: %T", f.wv))
			// Check if webview supports folder browsing before calling handler
			browser, ok := f.wv.(folderBrowser)
			debugLog(fmt.Sprintf("folderBrowser assertion: ok=%v, browser=%v", ok, browser != nil))
			if !ok {
				debugLog("folderBrowser interface NOT supported")
				return
			}
			debugLog("calling handleBrowseFolder")
			f.handleBrowseFolder(resp)
			debugLog("handleBrowseFolder returned")
			return
		}

		if resp.Type == "toggle_theme" {
			f.darkMode = !f.darkMode
			fmt.Printf("[webflow] Theme toggled via Shift+F5, new darkMode=%v\n", f.darkMode)

			// Update CSS class instantly (no re-render needed)
			// Use EvaluateScriptAsync to avoid deadlock when called from message handler
			if f.darkMode {
				f.wv.EvaluateScriptAsync(`document.documentElement.setAttribute('data-theme', 'dark')`)
			} else {
				f.wv.EvaluateScriptAsync(`document.documentElement.setAttribute('data-theme', 'light')`)
			}

			// Update window frame decorations using fallback colors for manual toggle
			// (we can't query system color since we're forcing a non-system theme)
			if f.darkMode {
				f.wv.SetFrameAppearance(types.FrameAppearance{
					TitleBar:         darkFrameColorFallback,
					BackdropTitleBar: darkBackdropFrameColorFallback,
				})
			} else {
				f.wv.SetFrameAppearance(types.FrameAppearance{
					TitleBar:         lightFrameColorFallback,
					BackdropTitleBar: lightBackdropFrameColorFallback,
				})
			}
			return
		}

		select {
		case f.responseCh <- resp:
			debugLog(fmt.Sprintf("message handler: sent to responseCh, button=%s", resp.Button))
			// If we should quit on message, do so
			f.mu.Lock()
			shouldQuit := f.quitOnMsg
			f.mu.Unlock()
			debugLog(fmt.Sprintf("message handler: shouldQuit=%v", shouldQuit))
			if shouldQuit {
				debugLog("message handler: calling Quit()")
				f.wv.Quit()
				debugLog("message handler: Quit() returned")
			}
		default:
			debugLog("message handler: channel full, dropping message")
		}
	})

	return f, nil
}

// Close releases the Flow's resources and closes the window.
func (f *Flow) Close() {
	if f.wv != nil {
		f.wv.Destroy()
	}
}

// Run starts the event loop. This must be called after all Show* methods complete
// if you want to keep the window open.
func (f *Flow) Run() {
	f.wv.Run()
}

// ShowPage displays a custom page and waits for user interaction.
// This is the core building block - all other Show* methods use it internally.
func (f *Flow) ShowPage(page Page) Response {
	html := renderPage(page, f.darkMode)

	f.wv.LoadHTML(html)
	f.wv.Show()

	// Enable quit on message
	f.mu.Lock()
	f.quitOnMsg = true
	f.mu.Unlock()

	// Run event loop until message received
	f.wv.Run()

	// Disable quit on message
	f.mu.Lock()
	f.quitOnMsg = false
	f.mu.Unlock()

	// Get response from channel
	msg := <-f.responseCh

	return Response{
		Button: msg.Button,
		Data:   msg.Data,
	}
}

// applyPageConfig creates a Page with the given config options applied.
func applyPageConfig(title string, content any, opts []PageOption) Page {
	cfg := PageConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	page := Page{
		Title:    title,
		Content:  content,
		Icon:     cfg.Icon,
		Subtitle: cfg.Subtitle,
	}

	if cfg.ButtonBar != nil {
		page.ButtonBar = *cfg.ButtonBar
	}

	return page
}

// ShowMessage displays a simple message with configurable buttons.
// Use WithButtonBar option to set navigation buttons.
// Default is SimpleOK() if no ButtonBar is provided.
func (f *Flow) ShowMessage(title, message string, opts ...PageOption) (Response, ButtonResult) {
	// Apply default ButtonBar if none provided
	hasButtonBar := false
	for _, opt := range opts {
		cfg := PageConfig{}
		opt(&cfg)
		if cfg.ButtonBar != nil {
			hasButtonBar = true
			break
		}
	}
	if !hasButtonBar {
		opts = append(opts, WithButtonBar(SimpleOK()))
	}

	page := applyPageConfig(title, message, opts)
	resp := f.ShowPage(page)
	return resp, resp.ToButtonResult()
}

// ShowChoice displays a list of options for single selection.
// Use WithButtonBar option to set navigation buttons.
// Default is WizardMiddle() if no ButtonBar is provided.
// Returns the selected index (0-based), Response, and ButtonResult.
func (f *Flow) ShowChoice(title string, options []string, opts ...PageOption) (int, Response, ButtonResult) {
	// Apply default ButtonBar if none provided
	hasButtonBar := false
	for _, opt := range opts {
		cfg := PageConfig{}
		opt(&cfg)
		if cfg.ButtonBar != nil {
			hasButtonBar = true
			break
		}
	}
	if !hasButtonBar {
		opts = append(opts, WithButtonBar(WizardMiddle()))
	}

	choices := make([]Choice, len(options))
	for i, opt := range options {
		choices[i] = Choice{Label: opt}
	}

	page := applyPageConfig(title, choices, opts)
	resp := f.ShowPage(page)

	// Extract selected index from response data
	selectedIdx := 0
	if idx, ok := resp.Data["_selected_index"]; ok {
		if idxFloat, ok := idx.(float64); ok {
			selectedIdx = int(idxFloat)
		}
	}

	return selectedIdx, resp, resp.ToButtonResult()
}

// ShowChoices displays a list of Choice structs for single selection.
// This provides more control over the display than ShowChoice.
// Use WithButtonBar option to set navigation buttons.
// Default is WizardMiddle() if no ButtonBar is provided.
func (f *Flow) ShowChoices(title string, choices []Choice, opts ...PageOption) (int, Response, ButtonResult) {
	// Apply default ButtonBar if none provided
	hasButtonBar := false
	for _, opt := range opts {
		cfg := PageConfig{}
		opt(&cfg)
		if cfg.ButtonBar != nil {
			hasButtonBar = true
			break
		}
	}
	if !hasButtonBar {
		opts = append(opts, WithButtonBar(WizardMiddle()))
	}

	page := applyPageConfig(title, choices, opts)
	resp := f.ShowPage(page)

	// Extract selected index from response data
	selectedIdx := 0
	if idx, ok := resp.Data["_selected_index"]; ok {
		if idxFloat, ok := idx.(float64); ok {
			selectedIdx = int(idxFloat)
		}
	}

	return selectedIdx, resp, resp.ToButtonResult()
}

// ShowForm displays a form with multiple input fields.
// Use WithButtonBar option to set navigation buttons.
// Default is WizardMiddle() if no ButtonBar is provided.
// Returns the field values as a map, Response, and ButtonResult.
func (f *Flow) ShowForm(title string, fields []FormField, opts ...PageOption) (map[string]any, Response, ButtonResult) {
	// Apply default ButtonBar if none provided
	hasButtonBar := false
	for _, opt := range opts {
		cfg := PageConfig{}
		opt(&cfg)
		if cfg.ButtonBar != nil {
			hasButtonBar = true
			break
		}
	}
	if !hasButtonBar {
		opts = append(opts, WithButtonBar(WizardMiddle()))
	}

	page := applyPageConfig(title, fields, opts)
	resp := f.ShowPage(page)

	// Extract form values from response data
	values := make(map[string]any)
	for _, field := range fields {
		if v, ok := resp.Data[field.ID]; ok {
			values[field.ID] = v
		}
	}

	return values, resp, resp.ToButtonResult()
}

// ShowConfirm displays a Yes/No confirmation dialog.
// Returns true if the user clicked Yes, false otherwise.
func (f *Flow) ShowConfirm(title, message string) bool {
	_, result := f.ShowMessage(title, message, WithButtonBar(ConfirmYesNo()))
	return result == ButtonResultNext
}

// ShowError displays an error message with an OK button and error icon.
func (f *Flow) ShowError(title, message string) {
	f.ShowMessage(title, message, WithIcon("error"), WithButtonBar(SimpleOK()))
}

// ShowTextInput displays a single text input dialog.
// Returns the entered text and the ButtonResult.
func (f *Flow) ShowTextInput(title, label, defaultValue string, opts ...PageOption) (string, ButtonResult) {
	// Apply default ButtonBar if none provided
	hasButtonBar := false
	for _, opt := range opts {
		cfg := PageConfig{}
		opt(&cfg)
		if cfg.ButtonBar != nil {
			hasButtonBar = true
			break
		}
	}
	if !hasButtonBar {
		opts = append(opts, WithButtonBar(SimpleOK()))
	}

	fields := []FormField{
		{
			ID:      "_text_input",
			Type:    FieldText,
			Label:   label,
			Default: defaultValue,
		},
	}

	page := applyPageConfig(title, fields, opts)
	resp := f.ShowPage(page)

	text := ""
	if v, ok := resp.Data["_text_input"].(string); ok {
		text = v
	}

	return text, resp.ToButtonResult()
}

// ShowMultiChoice displays a multi-selection list (checkboxes).
// Use WithButtonBar option to set navigation buttons.
// Default is WizardMiddle() if no ButtonBar is provided.
// Returns the selected indices, Response, and ButtonResult.
func (f *Flow) ShowMultiChoice(title string, options []string, opts ...PageOption) ([]int, Response, ButtonResult) {
	// Apply default ButtonBar if none provided
	hasButtonBar := false
	for _, opt := range opts {
		cfg := PageConfig{}
		opt(&cfg)
		if cfg.ButtonBar != nil {
			hasButtonBar = true
			break
		}
	}
	if !hasButtonBar {
		opts = append(opts, WithButtonBar(WizardMiddle()))
	}

	choices := make([]Choice, len(options))
	for i, opt := range options {
		choices[i] = Choice{Label: opt}
	}

	mc := MultiChoice{Choices: choices}
	page := applyPageConfig(title, mc, opts)
	resp := f.ShowPage(page)

	// Extract selected indices from response data
	var selectedIndices []int
	if indices, ok := resp.Data["_selected_indices"].([]any); ok {
		for _, idx := range indices {
			if idxFloat, ok := idx.(float64); ok {
				selectedIndices = append(selectedIndices, int(idxFloat))
			}
		}
	}

	return selectedIndices, resp, resp.ToButtonResult()
}

// ShowMenu displays a menu with clickable items.
// When the user clicks an item, the method returns immediately with the item index.
// Use WithButtonBar option to set navigation buttons.
// Returns the selected item index (-1 if cancelled via buttons), Response, and ButtonResult.
func (f *Flow) ShowMenu(title string, items []MenuItem, opts ...PageOption) (int, Response, ButtonResult) {
	// Apply default ButtonBar if none provided
	hasButtonBar := false
	for _, opt := range opts {
		cfg := PageConfig{}
		opt(&cfg)
		if cfg.ButtonBar != nil {
			hasButtonBar = true
			break
		}
	}
	if !hasButtonBar {
		opts = append(opts, WithButtonBar(ButtonBar{
			Close: NewButton("Close", ButtonClose),
		}))
	}

	page := applyPageConfig(title, items, opts)
	resp := f.ShowPage(page)

	// Check if a menu item was clicked
	selectedIdx := -1
	if resp.Button == "menu_item" {
		if idx, ok := resp.Data["_selected_index"]; ok {
			if idxFloat, ok := idx.(float64); ok {
				selectedIdx = int(idxFloat)
			}
		}
	}

	return selectedIdx, resp, resp.ToButtonResult()
}

// ShowLog displays a live log view and runs the work function.
// The work function receives a LogWriter interface to write log lines.
// This method blocks until the work is complete or cancelled.
func (f *Flow) ShowLog(title string, work func(log LogWriter)) {
	debugLog("ShowLog: starting")
	f.progressCancelled.Store(false)

	page := Page{
		Title:     title,
		Content:   LogConfig{Work: work},
		ButtonBar: WizardProgress(),
	}

	html := renderPage(page, f.darkMode)
	f.wv.LoadHTML(html)
	f.wv.Show()

	// Create log writer
	logWriter := &logWriterImpl{
		flow: f,
	}

	// Track whether work completed
	workDone := make(chan struct{})

	// Run work in goroutine
	go func() {
		debugLog("ShowLog: work goroutine starting")
		work(logWriter)
		debugLog("ShowLog: work goroutine finished")
		close(workDone)
		if !f.progressCancelled.Load() {
			debugLog("ShowLog: work goroutine calling Quit")
			f.wv.Quit()
		}
	}()

	// Enable quit on message (for cancel button)
	f.mu.Lock()
	f.quitOnMsg = true
	f.mu.Unlock()

	// Run event loop until work completes or cancel is clicked
	f.wv.Run()

	// Disable quit on message
	f.mu.Lock()
	f.quitOnMsg = false
	f.mu.Unlock()

	// Check if cancelled
	select {
	case msg := <-f.responseCh:
		if msg.Button == ButtonCancel {
			f.progressCancelled.Store(true)
		}
	default:
	}
}

// logWriterImpl implements the LogWriter interface.
type logWriterImpl struct {
	flow *Flow
}

func (l *logWriterImpl) WriteLine(text string) {
	l.WriteLineStyled(text, LogNormal)
}

func (l *logWriterImpl) WriteLineStyled(text string, style LogStyle) {
	styleClass := ""
	switch style {
	case LogSuccess:
		styleClass = "log-success"
	case LogWarning:
		styleClass = "log-warning"
	case LogError:
		styleClass = "log-error"
	case LogDim:
		styleClass = "log-dim"
	}

	script := `window.logWriteLine(` + jsonString(text) + `, ` + jsonString(styleClass) + `);`

	if async, ok := l.flow.wv.(asyncScriptEvaluator); ok {
		async.EvaluateScriptAsync(script)
	} else {
		l.flow.wv.EvaluateScript(script)
	}
}

func (l *logWriterImpl) Clear() {
	script := `window.logClear();`

	if async, ok := l.flow.wv.(asyncScriptEvaluator); ok {
		async.EvaluateScriptAsync(script)
	} else {
		l.flow.wv.EvaluateScript(script)
	}
}

func (l *logWriterImpl) SetStatus(status string) {
	script := `window.logSetStatus(` + jsonString(status) + `);`

	if async, ok := l.flow.wv.(asyncScriptEvaluator); ok {
		async.EvaluateScriptAsync(script)
	} else {
		l.flow.wv.EvaluateScript(script)
	}
}

func (l *logWriterImpl) Cancelled() bool {
	return l.flow.progressCancelled.Load()
}

// ShowFileProgress displays a file list progress view and runs the work function.
// The work function receives a FileList interface to add/update files.
// This method blocks until the work is complete or cancelled.
func (f *Flow) ShowFileProgress(title string, work func(files FileList)) {
	debugLog("ShowFileProgress: starting")
	f.progressCancelled.Store(false)

	page := Page{
		Title:     title,
		Content:   FileListConfig{Work: work},
		ButtonBar: WizardProgress(),
	}

	html := renderPage(page, f.darkMode)
	f.wv.LoadHTML(html)
	f.wv.Show()

	// Create file list handler
	fileList := &fileListImpl{
		flow: f,
	}

	// Track whether work completed
	workDone := make(chan struct{})

	// Run work in goroutine
	go func() {
		debugLog("ShowFileProgress: work goroutine starting")
		work(fileList)
		debugLog("ShowFileProgress: work goroutine finished")
		close(workDone)
		if !f.progressCancelled.Load() {
			debugLog("ShowFileProgress: work goroutine calling Quit")
			f.wv.Quit()
		}
	}()

	// Enable quit on message (for cancel button)
	f.mu.Lock()
	f.quitOnMsg = true
	f.mu.Unlock()

	// Run event loop until work completes or cancel is clicked
	f.wv.Run()

	// Disable quit on message
	f.mu.Lock()
	f.quitOnMsg = false
	f.mu.Unlock()

	// Check if cancelled
	select {
	case msg := <-f.responseCh:
		if msg.Button == ButtonCancel {
			f.progressCancelled.Store(true)
		}
	default:
	}
}

// fileListImpl implements the FileList interface.
type fileListImpl struct {
	flow *Flow
}

// SVG icons for file status
const (
	fileIconPending    = `<svg viewBox="0 0 16 16" fill="currentColor"><circle cx="8" cy="8" r="3"/></svg>`
	fileIconInProgress = `<svg viewBox="0 0 16 16" fill="currentColor"><circle cx="8" cy="8" r="6" fill="none" stroke="currentColor" stroke-width="2"/></svg>`
	fileIconComplete   = `<svg viewBox="0 0 16 16" fill="currentColor"><path d="M13.78 4.22a.75.75 0 010 1.06l-7.25 7.25a.75.75 0 01-1.06 0L2.22 9.28a.75.75 0 011.06-1.06L6 10.94l6.72-6.72a.75.75 0 011.06 0z"/></svg>`
	fileIconSkipped    = `<svg viewBox="0 0 16 16" fill="currentColor"><path d="M8 0a8 8 0 100 16A8 8 0 008 0zM1.5 8a6.5 6.5 0 1113 0 6.5 6.5 0 01-13 0z"/></svg>`
	fileIconFailed     = `<svg viewBox="0 0 16 16" fill="currentColor"><path d="M3.72 3.72a.75.75 0 011.06 0L8 6.94l3.22-3.22a.75.75 0 111.06 1.06L9.06 8l3.22 3.22a.75.75 0 11-1.06 1.06L8 9.06l-3.22 3.22a.75.75 0 01-1.06-1.06L6.94 8 3.72 4.78a.75.75 0 010-1.06z"/></svg>`
)

func (fl *fileListImpl) getStatusInfo(status FileStatus) (string, string) {
	switch status {
	case FilePending:
		return "pending", fileIconPending
	case FileInProgress:
		return "in-progress", fileIconInProgress
	case FileComplete:
		return "complete", fileIconComplete
	case FileSkipped:
		return "skipped", fileIconSkipped
	case FileFailed:
		return "failed", fileIconFailed
	default:
		return "pending", fileIconPending
	}
}

func (fl *fileListImpl) AddFile(path string, status FileStatus) {
	statusClass, iconSvg := fl.getStatusInfo(status)
	script := `window.fileListAddFile(` + jsonString(path) + `, ` + jsonString(statusClass) + `, ` + jsonString(iconSvg) + `);`

	if async, ok := fl.flow.wv.(asyncScriptEvaluator); ok {
		async.EvaluateScriptAsync(script)
	} else {
		fl.flow.wv.EvaluateScript(script)
	}
}

func (fl *fileListImpl) UpdateFile(path string, status FileStatus) {
	statusClass, iconSvg := fl.getStatusInfo(status)
	script := `window.fileListUpdateFile(` + jsonString(path) + `, ` + jsonString(statusClass) + `, ` + jsonString(iconSvg) + `);`

	if async, ok := fl.flow.wv.(asyncScriptEvaluator); ok {
		async.EvaluateScriptAsync(script)
	} else {
		fl.flow.wv.EvaluateScript(script)
	}
}

func (fl *fileListImpl) SetCurrentFile(path string) {
	script := `window.fileListSetCurrent(` + jsonString(path) + `);`

	if async, ok := fl.flow.wv.(asyncScriptEvaluator); ok {
		async.EvaluateScriptAsync(script)
	} else {
		fl.flow.wv.EvaluateScript(script)
	}
}

func (fl *fileListImpl) SetProgress(current, total int) {
	text := fmt.Sprintf("%d of %d files", current, total)
	script := `window.fileListSetProgress(` + jsonString(text) + `);`

	if async, ok := fl.flow.wv.(asyncScriptEvaluator); ok {
		async.EvaluateScriptAsync(script)
	} else {
		fl.flow.wv.EvaluateScript(script)
	}
}

func (fl *fileListImpl) SetStatus(status string) {
	script := `window.fileListSetStatus(` + jsonString(status) + `);`

	if async, ok := fl.flow.wv.(asyncScriptEvaluator); ok {
		async.EvaluateScriptAsync(script)
	} else {
		fl.flow.wv.EvaluateScript(script)
	}
}

func (fl *fileListImpl) Cancelled() bool {
	return fl.flow.progressCancelled.Load()
}

// ShowReview displays text content in a scrollable view with Copy and Save buttons.
// Useful for viewing logs, error details, or reports.
// The onCopy callback is invoked when user clicks Copy (view stays open).
// The onSave callback is invoked when user clicks Save (view stays open).
// Returns when user closes the dialog.
func (f *Flow) ShowReview(title, content string, onCopy func(), opts ...PageOption) {
	f.showReviewInternal(title, content, onCopy, nil, opts...)
}

// ShowReviewWithSave displays text content with Copy and Save buttons.
// Both callbacks are invoked while the view stays open.
func (f *Flow) ShowReviewWithSave(title, content string, onCopy, onSave func(), opts ...PageOption) {
	f.showReviewInternal(title, content, onCopy, onSave, opts...)
}

func (f *Flow) showReviewInternal(title, content string, onCopy, onSave func(), opts ...PageOption) {
	// Extract subtitle from options
	subtitle := ""
	var userButtonBar *ButtonBar
	for _, opt := range opts {
		cfg := PageConfig{}
		opt(&cfg)
		if cfg.Subtitle != "" {
			subtitle = cfg.Subtitle
		}
		if cfg.ButtonBar != nil {
			userButtonBar = cfg.ButtonBar
		}
	}

	// Build action buttons for copy/save
	var actions []*Button
	copyBtn := NewButton("Copy to Clipboard", "review_copy").
		WithIcon(IconCopy).
		AsIconOnly()
	actions = append(actions, copyBtn)

	if onSave != nil {
		saveBtn := NewButton("Save to File", "review_save").
			WithIcon(IconDownload).
			AsIconOnly()
		actions = append(actions, saveBtn)
	}

	// Use provided ButtonBar or default to SimpleClose
	var buttonBar ButtonBar
	if userButtonBar != nil {
		buttonBar = *userButtonBar
	} else {
		buttonBar = SimpleClose()
	}
	buttonBar.Actions = actions

	reviewCfg := ReviewConfig{
		Content:  content,
		OnCopy:   onCopy,
		OnSave:   onSave,
		Subtitle: subtitle,
	}

	page := Page{
		Title:     title,
		Content:   reviewCfg,
		ButtonBar: buttonBar,
	}

	// Render page once
	html := renderPage(page, f.darkMode)
	f.wv.LoadHTML(html)
	f.wv.Show()

	// Event loop - don't re-render on copy/save to preserve animations
	for {
		// Enable quit on message
		f.mu.Lock()
		f.quitOnMsg = true
		f.mu.Unlock()

		// Run event loop
		f.wv.Run()

		// Disable quit on message
		f.mu.Lock()
		f.quitOnMsg = false
		f.mu.Unlock()

		// Handle response
		select {
		case msg := <-f.responseCh:
			switch msg.Button {
			case "review_copy":
				if onCopy != nil {
					onCopy()
				}
				continue // Stay in dialog, wait for more messages
			case "review_save":
				// Show native save file dialog using saveFile helper (supports both new and legacy interfaces)
				path, ok := saveFile(f.wv, "Save As", "log.txt",
					types.FileFilter{Name: "Text Files", Patterns: []string{"*.txt"}},
					types.FileFilter{Name: "All Files", Patterns: []string{"*.*"}},
				)
				if ok && path != "" {
					// Write the content to the file
					if err := os.WriteFile(path, []byte(content), 0644); err == nil {
						if onSave != nil {
							onSave()
						}
					}
				}
				continue // Stay in dialog, wait for more messages
			default:
				return // Close or other button - exit
			}
		default:
			return
		}
	}
}

// ShowProgress displays a progress bar and executes the provided work function.
// The work function receives a Progress interface to report progress.
// This method blocks until the work is complete or cancelled.
// Returns true if the work completed normally, false if cancelled.
func (f *Flow) ShowProgress(title string, work func(p Progress)) bool {
	debugLog("ShowProgress: starting")
	f.progressCancelled.Store(false)

	page := Page{
		Title:     title,
		Content:   ProgressConfig{Work: work},
		ButtonBar: WizardProgress(),
	}

	html := renderPage(page, f.darkMode)
	f.wv.LoadHTML(html)
	f.wv.Show()

	// Create progress reporter
	progress := &progressImpl{
		flow: f,
	}

	// Track whether work completed
	workDone := make(chan struct{})

	// Run work in goroutine
	go func() {
		debugLog("ShowProgress: work goroutine starting")
		work(progress)
		debugLog("ShowProgress: work goroutine finished, closing workDone")
		close(workDone)
		// Only quit the event loop if we weren't cancelled
		// (if cancelled, the message handler already called Quit)
		if !f.progressCancelled.Load() {
			debugLog("ShowProgress: work goroutine calling Quit (not cancelled)")
			f.wv.Quit()
			debugLog("ShowProgress: work goroutine Quit returned")
		} else {
			debugLog("ShowProgress: work goroutine skipping Quit (was cancelled)")
		}
	}()

	// Enable quit on message (for cancel button)
	f.mu.Lock()
	f.quitOnMsg = true
	f.mu.Unlock()

	// Run event loop until work completes or cancel is clicked
	debugLog("ShowProgress: calling Run()")
	f.wv.Run()
	debugLog("ShowProgress: Run() returned")

	// Disable quit on message
	f.mu.Lock()
	f.quitOnMsg = false
	f.mu.Unlock()

	// Check if cancelled
	debugLog("ShowProgress: checking responseCh")
	select {
	case msg := <-f.responseCh:
		debugLog(fmt.Sprintf("ShowProgress: got message from responseCh: %+v", msg))
		if msg.Button == ButtonCancel {
			debugLog("ShowProgress: setting progressCancelled")
			f.progressCancelled.Store(true)
			// Don't wait for work to finish - the message loop has exited
			// and waiting would freeze the UI. The work goroutine will
			// check Cancelled() and clean up on its own.
			debugLog("ShowProgress: returning false (cancelled)")
			return false
		}
	default:
		debugLog("ShowProgress: work completed normally (no message in responseCh)")
	}
	debugLog("ShowProgress: returning true (completed)")
	return true
}

// progressImpl implements the Progress interface.
type progressImpl struct {
	flow *Flow
}

// asyncScriptEvaluator is an optional interface for non-blocking script execution.
// The Windows webview implementation provides this for cross-thread safety.
type asyncScriptEvaluator interface {
	EvaluateScriptAsync(script string)
}

func (p *progressImpl) Update(percent float64, status string) {
	// Clamp percent to 0-100
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	// Update progress bar via JavaScript
	script := `window.updateProgress(` + formatFloat(percent) + `, ` + jsonString(status) + `);`

	// Use async script execution if available (required for cross-thread safety on Windows)
	if async, ok := p.flow.wv.(asyncScriptEvaluator); ok {
		async.EvaluateScriptAsync(script)
	} else {
		p.flow.wv.EvaluateScript(script)
	}
}

func (p *progressImpl) Cancelled() bool {
	return p.flow.progressCancelled.Load()
}

// Helper functions
func formatFloat(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// webviewFocuser is an optional interface for focusing the WebView content.
// The Windows webview implementation provides this.
type webviewFocuser interface {
	FocusWebView()
}

// folderBrowser is an optional interface for native folder selection dialogs.
// Deprecated: Use types.Dialogs interface instead.
type folderBrowser interface {
	BrowseFolder(title string) string
}

// fileSaver is an optional interface for native file save dialogs.
// Deprecated: Use types.Dialogs interface instead.
type fileSaver interface {
	SaveFile(title, defaultName, filter string) string
}

// pickFolder uses the new Dialogs interface if available, otherwise falls back to legacy folderBrowser.
func pickFolder(wv types.WebFrame, title string) (string, bool) {
	// Try the new Dialogs interface first
	if d, ok := wv.(types.Dialogs); ok {
		return d.PickFolder(types.WithTitle(title))
	}
	// Fall back to legacy interface
	if b, ok := wv.(folderBrowser); ok {
		path := b.BrowseFolder(title)
		return path, path != ""
	}
	return "", false
}

// saveFile uses the new Dialogs interface if available, otherwise falls back to legacy fileSaver.
func saveFile(wv types.WebFrame, title, defaultName string, filters ...types.FileFilter) (string, bool) {
	// Try the new Dialogs interface first
	if d, ok := wv.(types.Dialogs); ok {
		return d.SaveFile(
			types.WithTitle(title),
			types.WithDefaultName(defaultName),
			types.WithFilters(filters...),
		)
	}
	// Fall back to legacy interface
	if s, ok := wv.(fileSaver); ok {
		// Convert filters to legacy format (simplified)
		filter := ""
		path := s.SaveFile(title, defaultName, filter)
		return path, path != ""
	}
	return "", false
}

// handleBrowseFolder handles a browse_folder message from JavaScript.
// It shows a native folder selection dialog and updates the input field with the result.
func (f *Flow) handleBrowseFolder(resp messageResponse) {
	debugLog(fmt.Sprintf("handleBrowseFolder: resp.Data=%v", resp.Data))

	// Get the target input ID from the message data
	targetID, _ := resp.Data["target"].(string)
	debugLog(fmt.Sprintf("handleBrowseFolder: targetID=%q", targetID))
	if targetID == "" {
		debugLog("handleBrowseFolder: targetID is empty, returning")
		return
	}

	// Get optional title
	title := "Select Folder"
	if t, ok := resp.Data["title"].(string); ok && t != "" {
		title = t
	}
	debugLog(fmt.Sprintf("handleBrowseFolder: title=%q", title))

	// Show the folder dialog using pickFolder helper (supports both new and legacy interfaces)
	debugLog("handleBrowseFolder: calling pickFolder")
	path, ok := pickFolder(f.wv, title)
	debugLog(fmt.Sprintf("handleBrowseFolder: pickFolder returned path=%q, ok=%v", path, ok))
	if !ok || path == "" {
		debugLog("handleBrowseFolder: path is empty (cancelled or not supported)")
		return // User cancelled or not supported
	}

	// Update the input field with the selected path
	script := `document.getElementById(` + jsonString(targetID) + `).value = ` + jsonString(path) + `;`
	debugLog(fmt.Sprintf("handleBrowseFolder: executing script: %s", script))

	// Use async script execution if available
	if async, ok := f.wv.(asyncScriptEvaluator); ok {
		async.EvaluateScriptAsync(script)
	} else {
		f.wv.EvaluateScript(script)
	}
}
