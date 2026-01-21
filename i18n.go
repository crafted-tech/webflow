// Package webflow provides a wizard-style UI framework using HTML/WebView.
// This file contains the i18n support for frontend-based translations.
package webflow

import "encoding/json"

// Translation markers - control characters for frontend translation
const (
	// TranslationPrefix marks a string as a translation key (SOH - Start of Heading)
	TranslationPrefix = "\x01"
	// ArgSeparator separates key from args in translation strings (STX - Start of Text)
	ArgSeparator = "\x02"
)

// T marks a string as a translation key.
// The frontend will look up this key in the translations dictionary.
//
// Example:
//
//	button := Button{Label: T("button.next")} // Will be translated to "Next", "Weiter", etc.
func T(key string) string {
	return TranslationPrefix + key
}

// TF marks a translation key with format arguments for placeholder substitution.
// The frontend will look up the key and replace {0}, {1}, etc. with the provided arguments.
//
// Example:
//
//	title := TF("welcome.title", "Unison Auditor")
//	// "Welcome to the {0} Setup Wizard" → "Welcome to the Unison Auditor Setup Wizard"
func TF(key string, args ...any) string {
	if len(args) == 0 {
		return T(key)
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		// Fallback to key only if JSON encoding fails
		return T(key)
	}
	return TranslationPrefix + key + ArgSeparator + string(argsJSON)
}

// appTranslations stores application-specific translations set via WithAppTranslations.
// These are merged with library translations on the frontend.
var appTranslations map[string]map[string]string

// WithAppTranslations sets application-specific translations that are merged
// with the library's built-in translations. App translations take precedence.
//
// Example:
//
//	flow, _ := webflow.New(
//	    webflow.WithAppTranslations(map[string]map[string]string{
//	        "en": {"welcome.title": "Welcome to My App", "app.custom": "Custom string"},
//	        "de": {"welcome.title": "Willkommen bei Meiner App", "app.custom": "Eigene Zeichenkette"},
//	    }),
//	)
func WithAppTranslations(translations map[string]map[string]string) Option {
	return func(c *Config) {
		c.AppTranslations = translations
		appTranslations = translations // Also store in global for template access
	}
}

// getAppTranslationsJS returns JavaScript code that defines app translations.
// This is injected into the HTML page and merged with library translations.
func getAppTranslationsJS() string {
	if appTranslations == nil || len(appTranslations) == 0 {
		return "const appTranslations = {};"
	}
	js, err := json.Marshal(appTranslations)
	if err != nil {
		return "const appTranslations = {};"
	}
	return "const appTranslations = " + string(js) + ";"
}
