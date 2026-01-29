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
	wv                types.WebFrame
	config            Config
	responseCh        chan messageResponse
	darkMode          bool
	mu                sync.Mutex
	quitOnMsg         bool // Whether to quit the event loop when a message is received
	primaryColorLight string
	primaryColorDark  string
	language          string // Current language code (e.g., "en", "es", "de")

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
		config:            cfg,
		responseCh:        make(chan messageResponse, 1),
		primaryColorLight: cfg.PrimaryColorLight,
		primaryColorDark:  cfg.PrimaryColorDark,
		language:          "en", // Default language
	}

	// Create webview
	resizable := cfg.Resizable == nil || *cfg.Resizable                // nil or true = resizable
	nativeTitleBar := cfg.NativeTitleBar != nil && *cfg.NativeTitleBar // nil or false = stylable titlebar
	wvConfig := types.Config{
		Title:          cfg.Title,
		Icon:           cfg.Icon,
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
		f.darkMode = wv.IsDarkMode()
	case *cfg.Theme == ThemeDark:
		f.darkMode = true
	case *cfg.Theme == ThemeLight:
		f.darkMode = false
	}

	// Set initial frame appearance using system headerbar colors
	frameColor := wv.GetHeaderBarColor()
	backdropFrameColor := wv.GetBackdropHeaderBarColor()
	wv.SetFrameAppearance(types.FrameAppearance{TitleBar: frameColor, BackdropTitleBar: backdropFrameColor})

	// Register for OS theme changes (only when using system theme)
	if cfg.Theme == nil || *cfg.Theme == ThemeSystem {
		wv.OnThemeChange(func(isDark bool) {
			f.darkMode = isDark

			// Update window frame decorations using system colors
			newFrameColor := f.wv.GetHeaderBarColor()
			newBackdropFrameColor := f.wv.GetBackdropHeaderBarColor()
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

		if resp.Type == "browse_path" {
			debugLog("browse_path message received")
			f.handleBrowsePath(resp)
			return
		}

		if resp.Type == "toggle_theme" {
			f.darkMode = !f.darkMode

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

		if resp.Type == "change_language" {
			if lang, ok := resp.Data["language"].(string); ok {
				f.mu.Lock()
				f.language = lang
				f.mu.Unlock()
				SetLanguage(lang, f.config.AppTranslations) // Update global state so T()/TF() use new language immediately

				// Notify app of language change (for persisting preference)
				if f.config.OnLanguageChange != nil {
					f.config.OnLanguageChange(lang)
				}

				// Send response so ShowPage returns and caller can rebuild page
				select {
				case f.responseCh <- messageResponse{
					Type:   "change_language",
					Button: "",
					Data:   map[string]any{"_language_changed": true},
				}:
					f.mu.Lock()
					shouldQuit := f.quitOnMsg
					f.mu.Unlock()
					if shouldQuit {
						f.wv.Quit()
					}
				default:
				}
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

	// Auto-initialize translations if app translations were provided
	if cfg.AppTranslations != nil {
		lang := cfg.InitialLanguage
		if lang == "" {
			lang = "en"
		}
		SetLanguage(lang, cfg.AppTranslations)
		f.language = lang
	}

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

// showPageInternal displays a page and returns the raw messageResponse.
// This is used internally by Show* methods to get the raw response.
func (f *Flow) showPageInternal(page Page) messageResponse {
	f.mu.Lock()
	lang := f.language
	f.mu.Unlock()

	// Set language for T()/TF() to translate immediately
	SetLanguage(lang, f.config.AppTranslations)
	html := renderPage(page, f.darkMode, f.primaryColorLight, f.primaryColorDark)

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
	return <-f.responseCh
}

// ShowPage displays a custom page and waits for user interaction.
// This is the core building block for custom pages.
//
// Returns:
//   - Navigation (Back/Close/Cancel) if user clicked a navigation button
//   - LanguageChange if user changed the language
//   - nil if user clicked Next/OK (proceeds with no data)
//   - map[string]any if the page has form data
func (f *Flow) ShowPage(page Page) any {
	msg := f.showPageInternal(page)

	// Check for language change
	if msg.Data != nil {
		if changed, _ := msg.Data["_language_changed"].(bool); changed {
			return LanguageChange{Lang: f.language}
		}
	}

	switch msg.Button {
	case ButtonBack:
		return Back
	case ButtonClose, ButtonCancel, "":
		if msg.Button == "" && msg.Type == "window_close" {
			return Close
		}
		if msg.Button == "" {
			// Empty button with data means proceed
			if msg.Data != nil {
				return msg.Data
			}
			return nil
		}
		return Close
	case ButtonNext:
		if len(msg.Data) > 0 {
			return msg.Data
		}
		return nil
	default:
		// Custom button - return as Navigation with the button ID
		return Navigation(msg.Button)
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

// ShowMessage displays a simple message or structured content with configurable buttons.
// The content parameter can be a string (for simple text) or other content types like
// SummaryConfig (for key-value summaries). Use WithButtonBar option to set navigation buttons.
// Default is SimpleOK() if no ButtonBar is provided.
//
// Returns:
//   - nil if user clicked Next/OK (without form data)
//   - map[string]any if user clicked Next/OK (with form data including checkboxes)
//   - Navigation (Back/Close/Cancel or custom button ID) for navigation
func (f *Flow) ShowMessage(title string, content any, opts ...PageOption) any {
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

	page := applyPageConfig(title, content, opts)
	msg := f.showPageInternal(page)

	switch msg.Button {
	case ButtonBack:
		return Back
	case ButtonClose, ButtonCancel, "":
		if msg.Button == "" && msg.Type != "window_close" {
			// Next/OK - return data if available
			if len(msg.Data) > 0 {
				return msg.Data
			}
			return nil
		}
		return Close
	case ButtonNext:
		// Return data if available (for checkboxes, etc.)
		if len(msg.Data) > 0 {
			return msg.Data
		}
		return nil
	default:
		return Navigation(msg.Button)
	}
}

// ShowChoice displays a list of Choice structs for single selection.
// Choices can have optional descriptions.
// Use WithButtonBar option to set navigation buttons.
// Default is WizardMiddle() if no ButtonBar is provided.
//
// Returns:
//   - int (selected index, 0-based) if user clicked Next
//   - Navigation (Back/Close/Cancel) for navigation
func (f *Flow) ShowChoice(title string, choices []Choice, opts ...PageOption) any {
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
	msg := f.showPageInternal(page)

	switch msg.Button {
	case ButtonBack:
		return Back
	case ButtonClose, ButtonCancel, "":
		if msg.Button == "" && msg.Type != "window_close" && msg.Data != nil {
			if idx, ok := msg.Data["_selected_index"].(float64); ok {
				return int(idx)
			}
			return 0
		}
		return Close
	case ButtonNext:
		if idx, ok := msg.Data["_selected_index"].(float64); ok {
			return int(idx)
		}
		return 0
	default:
		return Navigation(msg.Button)
	}
}

// ShowForm displays a form with multiple input fields.
// Use WithButtonBar option to set navigation buttons.
// Default is WizardMiddle() if no ButtonBar is provided.
//
// Returns:
//   - map[string]any with form field values (keyed by field ID) if user clicked Next
//   - Navigation (Back/Close/Cancel) for navigation
func (f *Flow) ShowForm(title string, fields []FormField, opts ...PageOption) any {
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
	msg := f.showPageInternal(page)

	switch msg.Button {
	case ButtonBack:
		return Back
	case ButtonClose, ButtonCancel, "":
		if msg.Button == "" && msg.Type != "window_close" && msg.Data != nil {
			return msg.Data
		}
		return Close
	case ButtonNext:
		if msg.Data == nil {
			return make(map[string]any)
		}
		return msg.Data
	default:
		return Navigation(msg.Button)
	}
}

// ShowConfirm displays a Yes/No confirmation dialog.
//
// Returns:
//   - true if user clicked Yes
//   - false if user clicked No
//   - Navigation (Close) if window was closed
func (f *Flow) ShowConfirm(title, message string) any {
	page := applyPageConfig(title, message, []PageOption{WithButtonBar(ConfirmYesNo())})
	msg := f.showPageInternal(page)

	switch msg.Button {
	case ButtonBack: // No button uses Back ID in ConfirmYesNo
		return false
	case ButtonNext: // Yes button uses Next ID
		return true
	case ButtonClose, ButtonCancel, "":
		if msg.Button == "" && msg.Type != "window_close" {
			return true // Default to Yes for unexpected empty button
		}
		return Close
	default:
		return Navigation(msg.Button)
	}
}

// ShowAlert displays an alert dialog with icon inline with title.
// The alert type determines the color scheme and icon (info, warning, error, success).
func (f *Flow) ShowAlert(alertType AlertType, title, message string, opts ...PageOption) {
	cfg := AlertConfig{
		Type:    alertType,
		Title:   title,
		Message: message,
	}

	// Apply default ButtonBar if none provided
	hasButtonBar := false
	for _, opt := range opts {
		pcfg := PageConfig{}
		opt(&pcfg)
		if pcfg.ButtonBar != nil {
			hasButtonBar = true
			break
		}
	}
	if !hasButtonBar {
		opts = append(opts, WithButtonBar(SimpleOK()))
	}

	f.ShowMessage("", cfg, opts...)
}

// ShowAlertInfo displays an info alert (blue).
func (f *Flow) ShowAlertInfo(title, message string) {
	f.ShowAlert(AlertInfo, title, message)
}

// ShowAlertWarning displays a warning alert (yellow/amber).
func (f *Flow) ShowAlertWarning(title, message string) {
	f.ShowAlert(AlertWarning, title, message)
}

// ShowAlertError displays an error alert (red).
func (f *Flow) ShowAlertError(title, message string) {
	f.ShowAlert(AlertError, title, message)
}

// ShowAlertSuccess displays a success alert (green).
func (f *Flow) ShowAlertSuccess(title, message string) {
	f.ShowAlert(AlertSuccess, title, message)
}

// ShowError displays an error message with an OK button and error icon.
// Deprecated: Use ShowAlertError instead for consistent styling.
func (f *Flow) ShowError(title, message string) {
	f.ShowAlertError(title, message)
}

// ShowErrorDetails displays an error message with OK and optional Details buttons.
// If detailsContent is provided, a Details button is shown that opens a log viewer
// with Copy and optional Save functionality.
func (f *Flow) ShowErrorDetails(title, message, detailsContent string, onCopy func(), onSave ...func()) {
	if detailsContent == "" {
		// No details - just show simple error
		f.ShowError(title, message)
		return
	}

	// Create button bar with Details button
	detailsBtn := NewButton(T("button.details"), "details")
	buttonBar := ButtonBar{
		Left:  detailsBtn,
		Close: NewButton(T("button.ok"), ButtonClose).WithPrimary(),
	}

	page := Page{
		Content:   AlertConfig{Type: AlertError, Title: title, Message: message},
		ButtonBar: buttonBar,
	}

	f.mu.Lock()
	lang := f.language
	f.mu.Unlock()
	SetLanguage(lang, f.config.AppTranslations)
	html := renderPage(page, f.darkMode, f.primaryColorLight, f.primaryColorDark)
	f.wv.LoadHTML(html)
	f.wv.Show()

	// Event loop
	for {
		f.mu.Lock()
		f.quitOnMsg = true
		f.mu.Unlock()

		f.wv.Run()

		f.mu.Lock()
		f.quitOnMsg = false
		f.mu.Unlock()

		select {
		case msg := <-f.responseCh:
			if msg.Button == "details" {
				// Show details in review dialog with Copy and optional Save
				if len(onSave) > 0 && onSave[0] != nil {
					f.ShowReviewWithSave(T("log.title"), detailsContent, onCopy, onSave[0])
				} else {
					f.ShowReview(T("log.title"), detailsContent, onCopy)
				}
				// Re-render and continue showing error
				f.wv.LoadHTML(html)
				continue
			}
			return // OK/Close clicked
		default:
			return
		}
	}
}

// ShowWelcome displays a welcome page with optional logo and language selector.
//
// Returns:
//   - nil if user clicked Next
//   - LanguageChange if user changed the language (caller should rebuild page)
//   - Navigation (Close) if window was closed
func (f *Flow) ShowWelcome(cfg WelcomeConfig, opts ...PageOption) any {
	// Apply default ButtonBar if none provided
	hasButtonBar := false
	for _, opt := range opts {
		pcfg := PageConfig{}
		opt(&pcfg)
		if pcfg.ButtonBar != nil {
			hasButtonBar = true
			break
		}
	}
	if !hasButtonBar {
		opts = append(opts, WithButtonBar(WizardFirst()))
	}

	page := applyPageConfig("", cfg, opts)
	msg := f.showPageInternal(page)

	// Check for language change
	if msg.Data != nil {
		if changed, _ := msg.Data["_language_changed"].(bool); changed {
			return LanguageChange{Lang: f.language}
		}
	}

	switch msg.Button {
	case ButtonNext:
		return nil
	case ButtonClose, ButtonCancel, "":
		if msg.Button == "" && msg.Type != "window_close" {
			return nil
		}
		return Close
	default:
		return Navigation(msg.Button)
	}
}

// ShowLicense displays a license agreement page.
//
// Returns:
//   - true if user clicked "I Agree"
//   - Navigation (Back/Close) for navigation
func (f *Flow) ShowLicense(cfg LicenseConfig, opts ...PageOption) any {
	// Apply default ButtonBar if none provided
	hasButtonBar := false
	for _, opt := range opts {
		pcfg := PageConfig{}
		opt(&pcfg)
		if pcfg.ButtonBar != nil {
			hasButtonBar = true
			break
		}
	}
	if !hasButtonBar {
		opts = append(opts, WithButtonBar(WizardLicense()))
	}

	page := applyPageConfig(cfg.Title, cfg, opts)
	msg := f.showPageInternal(page)

	switch msg.Button {
	case ButtonBack:
		return Back
	case ButtonNext:
		return true
	case ButtonClose, ButtonCancel, "":
		if msg.Button == "" && msg.Type != "window_close" {
			return true
		}
		return Close
	default:
		return Navigation(msg.Button)
	}
}

// ShowConfirmWithCheckbox displays a confirmation dialog with a required checkbox.
// The Next/Install button is disabled until the checkbox is checked.
//
// Returns:
//   - true if user confirmed (checked the box and clicked Next/Install)
//   - Navigation (Back/Close) for navigation
func (f *Flow) ShowConfirmWithCheckbox(cfg ConfirmCheckboxConfig, opts ...PageOption) any {
	// Apply default ButtonBar if none provided
	hasButtonBar := false
	for _, opt := range opts {
		pcfg := PageConfig{}
		opt(&pcfg)
		if pcfg.ButtonBar != nil {
			hasButtonBar = true
			break
		}
	}
	if !hasButtonBar {
		// Default to Install button bar with disabled Next
		bb := WizardInstall()
		bb.Next = bb.Next.Disabled() // Start disabled
		opts = append(opts, WithButtonBar(bb))
	}

	page := applyPageConfig(cfg.Title, cfg, opts)
	msg := f.showPageInternal(page)

	switch msg.Button {
	case ButtonBack:
		return Back
	case ButtonNext:
		return true
	case ButtonClose, ButtonCancel, "":
		if msg.Button == "" && msg.Type != "window_close" {
			return true
		}
		return Close
	default:
		return Navigation(msg.Button)
	}
}

// OpenFile shows a native file open dialog for selecting a single file.
// Returns the path and true if a file was selected, empty string and false if cancelled.
func (f *Flow) OpenFile(opts ...DialogOption) (string, bool) {
	if d, ok := f.wv.(types.Dialogs); ok {
		return d.OpenFile(opts...)
	}
	return "", false
}

// OpenFiles shows a native file open dialog for selecting multiple files.
// Returns the paths and true if files were selected, nil and false if cancelled.
func (f *Flow) OpenFiles(opts ...DialogOption) ([]string, bool) {
	if d, ok := f.wv.(types.Dialogs); ok {
		return d.OpenFiles(opts...)
	}
	return nil, false
}

// SaveFile shows a native file save dialog.
// Returns the path and true if a location was selected, empty string and false if cancelled.
func (f *Flow) SaveFile(opts ...DialogOption) (string, bool) {
	if d, ok := f.wv.(types.Dialogs); ok {
		return d.SaveFile(opts...)
	}
	return "", false
}

// PickFolder shows a native folder selection dialog.
// Returns the path and true if a folder was selected, empty string and false if cancelled.
func (f *Flow) PickFolder(opts ...DialogOption) (string, bool) {
	if d, ok := f.wv.(types.Dialogs); ok {
		return d.PickFolder(opts...)
	}
	return "", false
}

// ShowTextInput displays a single text input dialog.
//
// Returns:
//   - string (the entered text) if user clicked OK/Next
//   - Navigation (Back/Close) for navigation
func (f *Flow) ShowTextInput(title, label, defaultValue string, opts ...PageOption) any {
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
	msg := f.showPageInternal(page)

	switch msg.Button {
	case ButtonBack:
		return Back
	case ButtonClose, ButtonCancel, "":
		if msg.Button == "" && msg.Type != "window_close" && msg.Data != nil {
			if v, ok := msg.Data["_text_input"].(string); ok {
				return v
			}
			return ""
		}
		return Close
	case ButtonNext:
		if msg.Data != nil {
			if v, ok := msg.Data["_text_input"].(string); ok {
				return v
			}
		}
		return ""
	default:
		return Navigation(msg.Button)
	}
}

// ShowMultiChoice displays a multi-selection list (checkboxes).
// Choices can have optional descriptions.
// Use WithButtonBar option to set navigation buttons.
// Default is WizardMiddle() if no ButtonBar is provided.
//
// Returns:
//   - []int (selected indices, 0-based) if user clicked Next
//   - Navigation (Back/Close) for navigation
func (f *Flow) ShowMultiChoice(title string, choices []Choice, opts ...PageOption) any {
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

	mc := MultiChoice{Choices: choices}
	page := applyPageConfig(title, mc, opts)
	msg := f.showPageInternal(page)

	// Helper to extract indices from response data
	extractIndices := func(data map[string]any) []int {
		if data == nil {
			return nil
		}
		indices, ok := data["_selected_indices"].([]any)
		if !ok {
			return nil
		}
		result := make([]int, 0, len(indices))
		for _, idx := range indices {
			if idxFloat, ok := idx.(float64); ok {
				result = append(result, int(idxFloat))
			}
		}
		return result
	}

	switch msg.Button {
	case ButtonBack:
		return Back
	case ButtonClose, ButtonCancel, "":
		if msg.Button == "" && msg.Type != "window_close" && msg.Data != nil {
			return extractIndices(msg.Data)
		}
		return Close
	case ButtonNext:
		return extractIndices(msg.Data)
	default:
		return Navigation(msg.Button)
	}
}

// ShowMenu displays a menu with clickable items.
// When the user clicks an item, the method returns immediately with the item index.
// Use WithButtonBar option to set navigation buttons.
//
// Returns:
//   - int (selected item index, 0-based) if user clicked an item
//   - Navigation (Close) if user closed without selecting
func (f *Flow) ShowMenu(title string, items []MenuItem, opts ...PageOption) any {
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
	msg := f.showPageInternal(page)

	switch msg.Button {
	case "menu_item":
		if idx, ok := msg.Data["_selected_index"].(float64); ok {
			return int(idx)
		}
		return 0
	case ButtonClose, ButtonCancel, "":
		if msg.Button == "" && msg.Type != "window_close" && msg.Data != nil {
			if idx, ok := msg.Data["_selected_index"].(float64); ok {
				return int(idx)
			}
		}
		return Close
	default:
		return Navigation(msg.Button)
	}
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

	f.mu.Lock()
	lang := f.language
	f.mu.Unlock()
	SetLanguage(lang, f.config.AppTranslations)
	html := renderPage(page, f.darkMode, f.primaryColorLight, f.primaryColorDark)
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

	f.mu.Lock()
	lang := f.language
	f.mu.Unlock()
	SetLanguage(lang, f.config.AppTranslations)
	html := renderPage(page, f.darkMode, f.primaryColorLight, f.primaryColorDark)
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
	f.mu.Lock()
	lang := f.language
	f.mu.Unlock()
	SetLanguage(lang, f.config.AppTranslations)
	html := renderPage(page, f.darkMode, f.primaryColorLight, f.primaryColorDark)
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
				// Show native save file dialog
				path, ok := f.SaveFile(
					DialogTitle("Save As"),
					DialogDefaultName("log.txt"),
					DialogFilters(
						FileFilter{Name: "Text Files", Patterns: []string{"*.txt"}},
						FileFilter{Name: "All Files", Patterns: []string{"*.*"}},
					),
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
//
// Returns:
//   - nil if work completed successfully
//   - Navigation (Cancel/Close) if user cancelled
func (f *Flow) ShowProgress(title string, work func(p Progress)) any {
	debugLog("ShowProgress: starting")
	f.progressCancelled.Store(false)

	page := Page{
		Title:     title,
		Content:   ProgressConfig{Work: work},
		ButtonBar: WizardProgress(),
	}

	f.mu.Lock()
	lang := f.language
	f.mu.Unlock()
	SetLanguage(lang, f.config.AppTranslations)
	html := renderPage(page, f.darkMode, f.primaryColorLight, f.primaryColorDark)
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
			debugLog("ShowProgress: returning cancelled response")
			return Cancel
		}
	default:
		debugLog("ShowProgress: work completed normally (no message in responseCh)")
	}
	debugLog("ShowProgress: returning completed response")
	return nil
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

// handleBrowsePath handles a browse_path message from JavaScript.
// It shows a native file or folder selection dialog and updates the input field with the result.
func (f *Flow) handleBrowsePath(resp messageResponse) {
	debugLog(fmt.Sprintf("handleBrowsePath: resp.Data=%v", resp.Data))

	// Get the target input ID from the message data
	targetID, _ := resp.Data["target"].(string)
	if targetID == "" {
		debugLog("handleBrowsePath: targetID is empty, returning")
		return
	}

	// Get browse mode (file or folder)
	mode, _ := resp.Data["mode"].(string)
	if mode == "" {
		mode = "folder" // Default to folder for backward compatibility
	}

	// Get optional title
	title := "Select"
	if mode == "folder" {
		title = "Select Folder"
	} else {
		title = "Select File"
	}
	if t, ok := resp.Data["title"].(string); ok && t != "" {
		title = t
	}

	// Check if dialogs are supported
	d, ok := f.wv.(types.Dialogs)
	if !ok {
		debugLog("handleBrowsePath: dialogs not supported")
		return
	}

	// Show the appropriate dialog
	var path string
	if mode == "folder" {
		path, ok = d.PickFolder(types.WithTitle(title))
	} else {
		path, ok = d.OpenFile(types.WithTitle(title))
	}

	if !ok || path == "" {
		debugLog("handleBrowsePath: path is empty (cancelled)")
		return
	}

	// Update the input field with the selected path
	script := `document.getElementById(` + jsonString(targetID) + `).value = ` + jsonString(path) + `;`

	// Use async script execution if available
	if async, ok := f.wv.(asyncScriptEvaluator); ok {
		async.EvaluateScriptAsync(script)
	} else {
		f.wv.EvaluateScript(script)
	}
}
