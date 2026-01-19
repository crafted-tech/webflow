package webflow

// Config holds the configuration for creating a new Flow.
type Config struct {
	Title     string // Window title
	Width     string // Window width spec: "40em", "600", "80%" (default: "40em")
	Height    string // Window height spec: "30em", "450", "70%" (default: "30em")
	Resizable *bool  // nil or true = resizable, false = fixed size
	DarkMode  *bool  // nil = auto-detect, true = dark, false = light
}

// Option is a function that configures a Flow.
type Option func(*Config)

// WithTitle sets the window title.
func WithTitle(title string) Option {
	return func(c *Config) {
		c.Title = title
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

// WithDarkMode forces dark or light mode.
// If not called, the system preference is auto-detected.
func WithDarkMode(dark bool) Option {
	return func(c *Config) {
		c.DarkMode = &dark
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
