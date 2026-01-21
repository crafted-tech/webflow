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

// i18nJS contains the embedded i18n/translation system for the flow UI.
// This must be loaded before runtime.js in the HTML.
//
//go:embed assets/i18n.js
var i18nJSContent string
