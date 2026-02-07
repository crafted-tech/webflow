// Package webflow provides a declarative API for building wizard-like applications
// (installers, setup assistants, configuration tools, onboarding flows) using HTML rendering.
package webflow

import "github.com/crafted-tech/webframe/types"

// Navigation represents a navigation action (back, close, cancel, or custom button).
// When a Show* method returns a Navigation value, it means the user clicked a
// navigation button rather than proceeding with data.
type Navigation string

const (
	Back   Navigation = "back"
	Close  Navigation = "close"
	Cancel Navigation = "cancel"
)

// LanguageChange indicates the user changed the UI language via the language selector.
// When this is returned from ShowWelcome, the caller should rebuild the page
// to get fresh translations.
type LanguageChange struct {
	Lang string // The new language code (e.g., "en", "es", "de")
}

// IsBack returns true if the response is a Back navigation action.
func IsBack(resp any) bool {
	nav, ok := resp.(Navigation)
	return ok && nav == Back
}

// IsClose returns true if the response is a Close or Cancel navigation action.
func IsClose(resp any) bool {
	nav, ok := resp.(Navigation)
	return ok && (nav == Close || nav == Cancel)
}

// IsButton returns true if the response is a button click with the given ID.
// Works for both Navigation type (back/close) and custom buttons (map with _button key).
func IsButton(resp any, id string) bool {
	// Check Navigation type (standard buttons)
	if nav, ok := resp.(Navigation); ok {
		return string(nav) == id
	}
	// Check map type with _button key (custom inline buttons)
	if data, ok := resp.(map[string]any); ok {
		if btn, ok := data["_button"].(string); ok {
			return btn == id
		}
	}
	return false
}

// LanguageChanged returns true if the response indicates a language change.
func LanguageChanged(resp any) bool {
	_, ok := resp.(LanguageChange)
	return ok
}

// Language returns the new language code if the response is a LanguageChange,
// or an empty string otherwise.
func Language(resp any) string {
	if lc, ok := resp.(LanguageChange); ok {
		return lc.Lang
	}
	return ""
}

// IsCheckboxChecked returns true if the checkbox with the given ID is checked
// in the response data. This is used to read optional checkbox values from
// summary pages or form responses.
func IsCheckboxChecked(resp any, id string) bool {
	if data, ok := resp.(map[string]any); ok {
		if checked, ok := data[id].(bool); ok {
			return checked
		}
	}
	return false
}

// FieldType represents the type of a form field.
type FieldType int

const (
	FieldText FieldType = iota
	FieldPassword
	FieldCheckbox
	FieldSelect
	FieldFile     // Browse for file
	FieldFolder   // Browse for folder
	FieldTextArea
	FieldInfo // Read-only info/alert display (uses AlertType for styling)
)

// ButtonStyle defines the visual style for a button.
type ButtonStyle int

const (
	ButtonNormal  ButtonStyle = iota // Default button appearance
	ButtonPrimary                    // Emphasized appearance (usually Next/Install)
	ButtonDanger                     // Warning/destructive style
)

// Button represents a navigation button in the wizard.
type Button struct {
	Label    string      // Display text for the button
	ID       string      // Button identifier ("back", "next", "close", "cancel", or custom)
	Enabled  bool        // Whether the button is clickable (default true for convenience)
	Style    ButtonStyle // Visual style (Normal, Primary, Danger)
	Icon     string      // Optional icon SVG content (displayed before label)
	IconOnly bool        // If true, only show icon (label used for accessibility title)

	// Deprecated: Use Style instead. Kept for backwards compatibility.
	Primary bool // If true, button is styled as the primary action
	Danger  bool // If true, button is styled with danger/destructive styling
}

// NewButton creates a new enabled button with the given label and ID.
func NewButton(label, id string) *Button {
	return &Button{
		Label:   label,
		ID:      id,
		Enabled: true,
		Style:   ButtonNormal,
	}
}

// Disabled returns a copy of the button with Enabled set to false.
func (b *Button) Disabled() *Button {
	copy := *b
	copy.Enabled = false
	return &copy
}

// WithPrimary returns a copy of the button with Primary style.
func (b *Button) WithPrimary() *Button {
	copy := *b
	copy.Style = ButtonPrimary
	return &copy
}

// WithDanger returns a copy of the button with Danger style.
func (b *Button) WithDanger() *Button {
	copy := *b
	copy.Style = ButtonDanger
	return &copy
}

// WithIcon returns a copy of the button with the specified icon SVG.
func (b *Button) WithIcon(iconSVG string) *Button {
	copy := *b
	copy.Icon = iconSVG
	return &copy
}

// AsIconOnly returns a copy of the button that displays only the icon.
// The label is used as the button's title/tooltip for accessibility.
func (b *Button) AsIconOnly() *Button {
	copy := *b
	copy.IconOnly = true
	return &copy
}

// ButtonBar configures the navigation buttons for a page with fixed positions.
// Layout (Linux/macOS ordering): [Actions...] [Left] ... [spacing] ... [Back] [Next/Action] [Close]
// Buttons maintain stable positions across pages for consistent UX.
type ButtonBar struct {
	Back    *Button   // nil = no back button
	Next    *Button   // nil = no next button (usually primary action)
	Close   *Button   // nil = no close button
	Left    *Button   // nil = no left helper button (e.g., Help)
	Actions []*Button // Additional action buttons on the left (e.g., Copy, Save icons)
}

// WizardFirst returns a ButtonBar for the first wizard page: [Next >] [Close].
// No back button since going back is not possible.
// Button labels are translation keys - they will be translated by the frontend.
func WizardFirst() ButtonBar {
	return ButtonBar{
		Next:  NewButton(T("button.next"), ButtonNext).WithPrimary(),
		Close: NewButton(T("button.close"), ButtonClose),
	}
}

// WizardMiddle returns a ButtonBar for middle wizard pages: [Back] [Next >] [Close].
// Button labels are translation keys - they will be translated by the frontend.
func WizardMiddle() ButtonBar {
	return ButtonBar{
		Back:  NewButton(T("button.back"), ButtonBack),
		Next:  NewButton(T("button.next"), ButtonNext).WithPrimary(),
		Close: NewButton(T("button.close"), ButtonClose),
	}
}

// WizardInstall returns a ButtonBar for install confirmation: [Back] [Install] [Close].
// Button labels are translation keys - they will be translated by the frontend.
func WizardInstall() ButtonBar {
	return ButtonBar{
		Back:  NewButton(T("button.back"), ButtonBack),
		Next:  NewButton(T("button.install"), ButtonNext).WithPrimary(),
		Close: NewButton(T("button.close"), ButtonClose),
	}
}

// WizardFinish returns a ButtonBar for completion: [Finish].
// Button labels are translation keys - they will be translated by the frontend.
func WizardFinish() ButtonBar {
	return ButtonBar{
		Next: NewButton(T("button.finish"), ButtonClose).WithPrimary(),
	}
}

// WizardLicense returns a ButtonBar for license agreement: [Back] [I Agree] [Close].
// Button labels are translation keys - they will be translated by the frontend.
func WizardLicense() ButtonBar {
	return ButtonBar{
		Back:  NewButton(T("button.back"), ButtonBack),
		Next:  NewButton(T("button.iAgree"), ButtonNext).WithPrimary(),
		Close: NewButton(T("button.close"), ButtonClose),
	}
}

// WizardProgress returns a ButtonBar for progress pages: [Cancel].
// Button labels are translation keys - they will be translated by the frontend.
func WizardProgress() ButtonBar {
	return ButtonBar{
		Close: NewButton(T("button.cancel"), ButtonCancel),
	}
}

// SimpleOK returns a ButtonBar with just [OK].
// Button labels are translation keys - they will be translated by the frontend.
func SimpleOK() ButtonBar {
	return ButtonBar{
		Next: NewButton(T("button.ok"), ButtonNext).WithPrimary(),
	}
}

// SimpleClose returns a ButtonBar with just [Close].
// Button labels are translation keys - they will be translated by the frontend.
func SimpleClose() ButtonBar {
	return ButtonBar{
		Close: NewButton(T("button.close"), ButtonClose).WithPrimary(),
	}
}

// ConfirmYesNo returns a ButtonBar for confirmation: [No] [Yes].
// Button labels are translation keys - they will be translated by the frontend.
func ConfirmYesNo() ButtonBar {
	return ButtonBar{
		Back: NewButton(T("button.no"), ButtonBack),
		Next: NewButton(T("button.yes"), ButtonNext).WithPrimary(),
	}
}


// FormField represents a single input field in a form.
type FormField struct {
	ID              string    // Unique identifier for the field
	Type            FieldType // Type of input (Text, Password, Checkbox, etc.)
	Label           string    // Display label for the field
	Placeholder     string    // Placeholder text for text inputs
	Default         any       // Default value for the field
	Options         []string  // Options for Select type fields
	Required        bool      // If true, field must be filled
	Width           string    // Field width: "narrow", "medium", or "" (full, default)
	Suffix          *Button   // Optional inline button shown after the field
	AlertType       AlertType // For FieldInfo: determines styling (info, warning, error, success)
	InvalidatesForm bool      // If true, changing this field hides alerts and disables Next button
	Hidden          bool      // If true, field is initially hidden (shown when form is invalidated)
	Focus           bool      // If true, field receives focus when form is displayed
}

// Choice represents an option in a choice list.
type Choice struct {
	Label       string // Display text for the choice
	Description string // Optional description/subtitle
	Value       string // Value to return when selected
}

// MultiChoice represents a multi-selection list (checkboxes).
type MultiChoice struct {
	Choices  []Choice // Available choices
	Selected []int    // Initially selected indices (0-based)
}

// MenuItem represents a clickable item in a menu view.
type MenuItem struct {
	Title       string // Main title text (required)
	Description string // Secondary description text (optional)
	Icon        string // Icon name or SVG (optional)
}


// Page defines a wizard page with content and navigation buttons.
type Page struct {
	Title      string    // Main title displayed at the top
	Subtitle   string    // Optional subtitle/description below the title
	Icon       string    // Icon name ("info", "warning", "error", "success") or custom SVG
	Logo        []byte // Optional SVG/PNG logo data rendered above the title
	LogoWidth   int    // Logo width in pixels (0 for auto)
	LogoHeight  int    // Logo height in pixels (0 for auto)
	LogoAlign   string // Logo horizontal alignment: "left", "center", "right" (default: "center")
	CenterTitle bool   // Center the title text horizontally
	Content    any       // Content: string (message), []Choice, []FormField, or ProgressConfig
	ButtonBar  ButtonBar // Navigation buttons with fixed positions (preferred)
	Buttons    []Button  // Deprecated: use ButtonBar instead. Legacy button array.
}

// ProgressConfig configures a progress page.
type ProgressConfig struct {
	Work func(p Progress) // Function that performs the work and reports progress
}

// PageConfig holds configuration for pages that accept PageOption.
type PageConfig struct {
	ButtonBar  *ButtonBar
	Icon       string
	Subtitle   string
	Logo        []byte
	LogoWidth   int
	LogoHeight  int
	LogoAlign   string
	CenterTitle bool
}

// PageOption configures a page.
type PageOption func(*PageConfig)

// WithButtonBar sets the button bar configuration for the page.
func WithButtonBar(bb ButtonBar) PageOption {
	return func(c *PageConfig) {
		c.ButtonBar = &bb
	}
}

// WithIcon sets the icon for the page.
func WithIcon(icon string) PageOption {
	return func(c *PageConfig) {
		c.Icon = icon
	}
}

// WithSubtitle sets the subtitle for the page.
func WithSubtitle(subtitle string) PageOption {
	return func(c *PageConfig) {
		c.Subtitle = subtitle
	}
}

// WithCenterTitle centers the page title horizontally.
func WithCenterTitle() PageOption {
	return func(c *PageConfig) {
		c.CenterTitle = true
	}
}

// WithLogo sets a logo image (SVG or PNG bytes) to display above the page title.
// Pass 0 for width or height to scale proportionally from the other dimension.
// If both are 0, height defaults to 48px.
func WithLogo(logo []byte, width, height int) PageOption {
	return func(c *PageConfig) {
		c.Logo = logo
		c.LogoWidth = width
		c.LogoHeight = height
	}
}

// WithLogoAlign sets the horizontal alignment of the logo: "left", "center", or "right".
// Default is "center".
func WithLogoAlign(align string) PageOption {
	return func(c *PageConfig) {
		c.LogoAlign = align
	}
}

// Progress interface for updating progress during long-running operations.
type Progress interface {
	// Update sets the current progress percentage (0-100) and status message.
	Update(percent float64, status string)
	// Cancelled returns true if the user has requested cancellation.
	Cancelled() bool
}

// LogStyle defines the visual style for log lines.
type LogStyle int

const (
	LogNormal  LogStyle = iota // Default text
	LogSuccess                 // Green, for success messages
	LogWarning                 // Yellow, for warnings
	LogError                   // Red, for errors
	LogDim                     // Gray/muted, for less important info
)

// LogWriter provides methods for writing to a live log/console view.
type LogWriter interface {
	// WriteLine appends a line to the scrolling view.
	WriteLine(text string)

	// WriteLineStyled appends a styled line to the scrolling view.
	WriteLineStyled(text string, style LogStyle)

	// Clear removes all content from the log.
	Clear()

	// SetStatus updates the status line at the bottom.
	SetStatus(status string)

	// Cancelled returns true if the user requested cancellation.
	Cancelled() bool
}

// LogConfig configures a log view page.
type LogConfig struct {
	Work func(log LogWriter) // Function that performs the work and writes to the log
}

// FileStatus represents the status of a file operation.
type FileStatus int

const (
	FilePending    FileStatus = iota // Not yet processed
	FileInProgress                   // Currently being processed
	FileComplete                     // Successfully processed
	FileSkipped                      // Skipped
	FileFailed                       // Failed to process
)

// FileList provides methods for showing file operation progress.
type FileList interface {
	// AddFile adds a file to the list with the given status.
	AddFile(path string, status FileStatus)

	// UpdateFile updates the status of an existing file.
	UpdateFile(path string, status FileStatus)

	// SetCurrentFile highlights the currently processing file.
	SetCurrentFile(path string)

	// SetProgress updates the overall progress (e.g., "5 of 100").
	SetProgress(current, total int)

	// SetStatus updates the overall status text.
	SetStatus(status string)

	// Cancelled returns true if the user requested cancellation.
	Cancelled() bool
}

// FileListConfig configures a file progress page.
type FileListConfig struct {
	Work func(files FileList) // Function that performs the work and updates the file list
}

// ReviewConfig configures a review/text viewer page.
type ReviewConfig struct {
	Content  string // Text content to display
	OnCopy   func() // Callback when Copy is clicked
	OnSave   func() // Callback when Save is clicked
	Subtitle string // Optional subtitle (e.g., file path)
}

// WelcomeConfig configures a welcome page with optional logo and language selector.
type WelcomeConfig struct {
	Logo             []byte // Optional SVG/PNG logo data
	LogoHeight       int    // Logo height in pixels (0 for default: 64)
	Title            string // Main title
	Message          string // Welcome message (can include continue instructions if desired)
	LanguageSelector bool   // Show language selector
}

// LicenseConfig configures a license agreement page.
type LicenseConfig struct {
	Title   string // Page title (e.g., "License Agreement")
	Label   string // Instruction text above the license
	Content string // License text content
}

// ConfirmCheckboxConfig configures a confirmation dialog with a required checkbox.
type ConfirmCheckboxConfig struct {
	Title          string // Dialog title
	Message        string // Main message text
	CheckboxLabel  string // Label for the required checkbox
	WarningMessage string // Optional warning message (shown in yellow/orange)
}

// AlertType defines the type of alert (used for both inline alerts and alert dialogs).
type AlertType string

const (
	AlertNone    AlertType = ""        // Default: render as key-value (for summary items)
	AlertInfo    AlertType = "info"    // Blue info box
	AlertWarning AlertType = "warning" // Yellow/amber warning box
	AlertError   AlertType = "error"   // Red error box
	AlertSuccess AlertType = "success" // Green success box
)

// AlertConfig configures an alert dialog with icon inline with title.
type AlertConfig struct {
	Type    AlertType // Alert type (determines color and icon)
	Title   string    // Alert title (shown inline with icon)
	Message string    // Alert message (shown below title)
}

// SummaryItem represents a single key-value pair in a summary display.
// Used for "Ready to Install" pages where labels need translation but values are literal.
// If AlertType is set, the item renders as an alert box with icon instead of key-value.
type SummaryItem struct {
	Label     string    // Label text (use T() for translation keys); unused for alerts
	Value     string    // Literal value (rendered as-is); for alerts: the message
	AlertType AlertType // If set, render as alert box with icon instead of key-value
}

// SummaryCheckbox represents an acknowledgment checkbox in a summary display.
// Used for downgrade/reinstall confirmations that require user acknowledgment.
type SummaryCheckbox struct {
	ID       string // Identifier for the checkbox (e.g., "downgrade", "reinstall")
	Label    string // Checkbox label text
	Required bool   // If true, Install button disabled until checked
	Warning  string // Optional warning text shown above checkbox (yellow box)
	Checked  bool   // Initial checked state (default: false)
}

// SummaryConfig configures a summary/review display with labeled key-value pairs.
// The frontend translates labels (if they have translation prefix) while values
// are rendered as literal text.
type SummaryConfig struct {
	Items      []SummaryItem     // Key-value pairs to display
	Checkboxes []SummaryCheckbox // Optional acknowledgment checkboxes
}

// Dialog types re-exported from webframe/types for convenience.
// These are used with OpenFile, OpenFiles, SaveFile, and PickFolder methods.
type (
	FileFilter   = types.FileFilter
	DialogOption = types.DialogOption
)

// Dialog option functions re-exported from webframe/types.
var (
	DialogTitle       = types.WithTitle       // Set dialog title
	DialogDefaultDir  = types.WithDefaultDir  // Set starting directory
	DialogDefaultName = types.WithDefaultName // Set default filename (SaveFile only)
	DialogFilters     = types.WithFilters     // Set file type filters
	DialogFilterIndex = types.WithFilterIndex // Set default filter index (0-based)
	Filter            = types.Filter          // Create a FileFilter: Filter("Images", "*.png", "*.jpg")
)

// Built-in button IDs
const (
	ButtonBack   = "back"
	ButtonNext   = "next"
	ButtonClose  = "close"
	ButtonCancel = "cancel"
)

// Standard button presets
var (
	BackButton   = Button{Label: "Back", ID: ButtonBack}
	NextButton   = Button{Label: "Next", ID: ButtonNext, Primary: true}
	CloseButton  = Button{Label: "Close", ID: ButtonClose}
	CancelButton = Button{Label: "Cancel", ID: ButtonCancel}
	FinishButton = Button{Label: "Finish", ID: ButtonClose, Primary: true}
)
