package webflow

import _ "embed"

// CSS contains the embedded stylesheet for the flow UI.
//
//go:embed assets/style.css
var cssContent string

// JS contains the embedded JavaScript runtime for the flow UI.
//
//go:embed assets/runtime.js
var jsContent string
