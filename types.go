// Package webflow provides a declarative API for building wizard-like applications
// (installers, setup assistants, configuration tools, onboarding flows) using HTML rendering.
package webflow

// FieldType represents the type of a form field.
type FieldType int

const (
	FieldText FieldType = iota
	FieldPassword
	FieldCheckbox
	FieldSelect
	FieldPath
	FieldTextArea
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

// ButtonResult represents which button the user clicked.
type ButtonResult int

const (
	ButtonResultClose ButtonResult = iota // Close clicked or window closed
	ButtonResultBack                      // Back clicked
	ButtonResultNext                      // Next/OK/Install clicked
	ButtonResultLeft                      // Left helper clicked
)

// WizardFirst returns a ButtonBar for the first wizard page: [Next >] [Close].
// No back button since going back is not possible.
func WizardFirst() ButtonBar {
	return ButtonBar{
		Next:  NewButton("Next", ButtonNext).WithPrimary(),
		Close: NewButton("Close", ButtonClose),
	}
}

// WizardMiddle returns a ButtonBar for middle wizard pages: [Back] [Next >] [Close].
func WizardMiddle() ButtonBar {
	return ButtonBar{
		Back:  NewButton("Back", ButtonBack),
		Next:  NewButton("Next", ButtonNext).WithPrimary(),
		Close: NewButton("Close", ButtonClose),
	}
}

// WizardInstall returns a ButtonBar for install confirmation: [Back] [Install] [Close].
func WizardInstall() ButtonBar {
	return ButtonBar{
		Back:  NewButton("Back", ButtonBack),
		Next:  NewButton("Install", ButtonNext).WithPrimary(),
		Close: NewButton("Close", ButtonClose),
	}
}

// WizardFinish returns a ButtonBar for completion: [Finish].
func WizardFinish() ButtonBar {
	return ButtonBar{
		Next: NewButton("Finish", ButtonClose).WithPrimary(),
	}
}

// WizardLicense returns a ButtonBar for license agreement: [Back] [I Agree] [Close].
func WizardLicense() ButtonBar {
	return ButtonBar{
		Back:  NewButton("Back", ButtonBack),
		Next:  NewButton("I Agree", ButtonNext).WithPrimary(),
		Close: NewButton("Close", ButtonClose),
	}
}

// WizardProgress returns a ButtonBar for progress pages: [Cancel].
func WizardProgress() ButtonBar {
	return ButtonBar{
		Close: NewButton("Cancel", ButtonCancel),
	}
}

// SimpleOK returns a ButtonBar with just [OK].
func SimpleOK() ButtonBar {
	return ButtonBar{
		Next: NewButton("OK", ButtonNext).WithPrimary(),
	}
}

// SimpleClose returns a ButtonBar with just [Close].
func SimpleClose() ButtonBar {
	return ButtonBar{
		Close: NewButton("Close", ButtonClose).WithPrimary(),
	}
}

// ConfirmYesNo returns a ButtonBar for confirmation: [No] [Yes].
func ConfirmYesNo() ButtonBar {
	return ButtonBar{
		Back: NewButton("No", ButtonBack),
		Next: NewButton("Yes", ButtonNext).WithPrimary(),
	}
}

// Response holds the result of a user interaction with a flow page.
type Response struct {
	Button string         // Which button was clicked
	Data   map[string]any // Form data, selections, etc.
}

// ToButtonResult converts the Response's Button string to a ButtonResult.
// This maps button IDs to the corresponding result:
//   - "back" -> ButtonResultBack
//   - "next" -> ButtonResultNext
//   - "left" -> ButtonResultLeft
//   - "close", "cancel", or window close -> ButtonResultClose
func (r Response) ToButtonResult() ButtonResult {
	switch r.Button {
	case ButtonBack:
		return ButtonResultBack
	case ButtonNext:
		return ButtonResultNext
	case "left":
		return ButtonResultLeft
	default:
		return ButtonResultClose
	}
}

// FormField represents a single input field in a form.
type FormField struct {
	ID          string    // Unique identifier for the field
	Type        FieldType // Type of input (Text, Password, Checkbox, etc.)
	Label       string    // Display label for the field
	Placeholder string    // Placeholder text for text inputs
	Default     any       // Default value for the field
	Options     []string  // Options for Select type fields
	Required    bool      // If true, field must be filled
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
	Title     string    // Main title displayed at the top
	Subtitle  string    // Optional subtitle/description below the title
	Icon      string    // Icon name ("info", "warning", "error", "success") or custom SVG
	Content   any       // Content: string (message), []Choice, []FormField, or ProgressConfig
	ButtonBar ButtonBar // Navigation buttons with fixed positions (preferred)
	Buttons   []Button  // Deprecated: use ButtonBar instead. Legacy button array.
}

// ProgressConfig configures a progress page.
type ProgressConfig struct {
	Work func(p Progress) // Function that performs the work and reports progress
}

// PageConfig holds configuration for pages that accept PageOption.
type PageConfig struct {
	ButtonBar *ButtonBar
	Icon      string
	Subtitle  string
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
