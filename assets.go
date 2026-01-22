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

// translationsJSON contains the embedded library translations as JSON.
// This is the single source of truth for all 12 languages.
//
//go:embed assets/translations.json
var translationsJSON []byte

// iconsJSON contains the embedded SVG icons as JSON.
//
//go:embed assets/icons.json
var iconsJSON []byte
