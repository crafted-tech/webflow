package webflow

// ThemeMode specifies the color theme for the UI.
type ThemeMode int

const (
	ThemeSystem ThemeMode = iota // Auto-detect from OS (default)
	ThemeDark                    // Force dark mode
	ThemeLight                   // Force light mode
)

// Config holds the configuration for creating a new Flow.
type Config struct {
	Title             string                       // Window title
	Icon              []byte                       // Window icon (PNG data for titlebar/taskbar)
	Width             string                       // Window width spec: "40em", "600", "80%" (default: "40em")
	Height            string                       // Window height spec: "30em", "450", "70%" (default: "30em")
	Resizable         *bool                        // nil or true = resizable, false = fixed size
	Theme             *ThemeMode                   // nil = system (auto-detect)
	NativeTitleBar    *bool                        // nil or false = stylable titlebar, true = native system titlebar
	PrimaryColorLight string                       // HSL values for light mode, e.g., "142 70% 35%"
	PrimaryColorDark  string                       // HSL values for dark mode, e.g., "142 70% 50%"
	AppTranslations   map[string]map[string]string // App-specific translations: lang -> key -> value
}

// Option is a function that configures a Flow.
type Option func(*Config)

// WithTitle sets the window title.
func WithTitle(title string) Option {
	return func(c *Config) {
		c.Title = title
	}
}

// WithWindowIcon sets the window icon (titlebar/taskbar).
// Accepts PNG image data which will be wrapped in ICO format on Windows.
func WithWindowIcon(pngData []byte) Option {
	return func(c *Config) {
		c.Icon = pngData
	}
}

// WithSize sets the window dimensions.
// Accepts dimension specs like "40em", "600", "600px", or "80%".
func WithSize(width, height string) Option {
	return func(c *Config) {
		c.Width = width
		c.Height = height
	}
}

// WithResizable sets whether the window can be resized.
// If not called, the window is resizable by default.
func WithResizable(resizable bool) Option {
	return func(c *Config) {
		c.Resizable = &resizable
	}
}

// WithTheme sets the color theme mode.
// ThemeSystem (default): Auto-detect from OS using webframe's IsDarkMode()
// ThemeDark: Force dark mode
// ThemeLight: Force light mode
func WithTheme(mode ThemeMode) Option {
	return func(c *Config) {
		c.Theme = &mode
	}
}

// WithNativeTitleBar uses native system titlebar instead of app-drawn stylable titlebar.
// When true: Window uses native system titlebar (no frame styling)
// When false (default): Window uses stylable titlebar
// Only affects Linux (GTK3/GTK4). Ignored on Windows/macOS.
func WithNativeTitleBar(native bool) Option {
	return func(c *Config) {
		c.NativeTitleBar = &native
	}
}

// WithPrimaryColor sets custom primary color for light and dark modes.
// Colors should be HSL values without the hsl() wrapper, e.g., "200 70% 50%".
// Common colors:
//   - Green:  "142 70% 35%" (light), "142 70% 50%" (dark)
//   - Blue:   "217 91% 50%" (light), "217 91% 60%" (dark)
//   - Purple: "270 70% 50%" (light), "270 70% 60%" (dark)
func WithPrimaryColor(light, dark string) Option {
	return func(c *Config) {
		c.PrimaryColorLight = light
		c.PrimaryColorDark = dark
	}
}

// defaultConfig returns the default configuration.
func defaultConfig() Config {
	return Config{
		Title:  "Setup",
		Width:  "40em",
		Height: "30em",
	}
}
