// Package webflow provides a wizard-style UI framework using HTML/WebView.
// This file contains the i18n support for backend-based translations.
package webflow

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// Translation markers - control characters for translation keys
const (
	// TranslationPrefix marks a string as a translation key (SOH - Start of Heading)
	TranslationPrefix = "\x01"
	// ArgSeparator separates key from args in translation strings (STX - Start of Text)
	ArgSeparator = "\x02"
)

// libraryTranslations contains built-in translations for 12 languages.
// Loaded from embedded JSON at init time.
var libraryTranslations map[string]map[string]string

func init() {
	if err := json.Unmarshal(translationsJSON, &libraryTranslations); err != nil {
		panic("webflow: failed to parse embedded translations: " + err.Error())
	}
}

// Package-level language state for immediate translation in T() and TF().
// This is set before rendering a page and used by T()/TF() to translate immediately.
var (
	currentLanguage        = "en"
	currentAppTranslations map[string]map[string]string
	langMu                 sync.RWMutex
)

// SetLanguage sets the current language and app translations for T() and TF().
// This must be called before rendering a page to ensure translations are correct.
func SetLanguage(lang string, appTrans map[string]map[string]string) {
	langMu.Lock()
	currentLanguage = lang
	currentAppTranslations = appTrans
	langMu.Unlock()
}

// T translates a string immediately using the current language.
// Call SetLanguage() before rendering to ensure correct translations.
//
// Example:
//
//	button := Button{Label: T("button.next")} // Will be translated to "Next", "Weiter", etc.
func T(key string) string {
	langMu.RLock()
	lang := currentLanguage
	appTrans := currentAppTranslations
	langMu.RUnlock()

	return lookupTranslation(key, lang, appTrans)
}

// TF translates a key with format arguments, substituting placeholders immediately.
// Call SetLanguage() before rendering to ensure correct translations.
//
// Example:
//
//	title := TF("welcome.title", "Unison Auditor")
//	// "Welcome to the {0} Setup Wizard" → "Welcome to the Unison Auditor Setup Wizard"
func TF(key string, args ...any) string {
	langMu.RLock()
	lang := currentLanguage
	appTrans := currentAppTranslations
	langMu.RUnlock()

	template := lookupTranslation(key, lang, appTrans)

	// Substitute {0}, {1}, etc. with args
	for i, arg := range args {
		placeholder := fmt.Sprintf("{%d}", i)
		template = strings.ReplaceAll(template, placeholder, fmt.Sprint(arg))
	}
	return template
}

// TranslateString translates a string that may contain translation markers.
// If the string starts with TranslationPrefix (\x01), it's parsed as a translation key
// with optional arguments. Otherwise, the string is returned as-is.
//
// The lookup order is:
//  1. appTranslations[lang][key] - app-specific translation for current language
//  2. appTranslations["en"][key] - app-specific English fallback
//  3. libraryTranslations[lang][key] - built-in translation for current language
//  4. libraryTranslations["en"][key] - built-in English fallback
//  5. key itself - if no translation found
func TranslateString(s string, lang string, appTrans map[string]map[string]string) string {
	if s == "" || !strings.HasPrefix(s, TranslationPrefix) {
		return s // Literal text, return as-is
	}

	// Strip prefix and parse key + optional args
	content := s[1:] // Remove \x01
	var key string
	var args []any

	sepIndex := strings.Index(content, ArgSeparator)
	if sepIndex >= 0 {
		key = content[:sepIndex]
		argsJSON := content[sepIndex+1:]
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			// Ignore parse errors, just use empty args
			args = nil
		}
	} else {
		key = content
	}

	// Get translation template with fallback chain
	template := lookupTranslation(key, lang, appTrans)

	// Recursively translate any args that are themselves translation keys
	for i, arg := range args {
		if str, ok := arg.(string); ok && strings.HasPrefix(str, TranslationPrefix) {
			args[i] = TranslateString(str, lang, appTrans)
		}
	}

	// Substitute placeholders: {0}, {1}, etc.
	result := template
	for i, arg := range args {
		placeholder := "{" + string(rune('0'+i)) + "}"
		argStr := ""
		switch v := arg.(type) {
		case string:
			argStr = v
		default:
			// Convert non-string args to string via JSON
			if b, err := json.Marshal(v); err == nil {
				argStr = string(b)
			}
		}
		result = strings.ReplaceAll(result, placeholder, argStr)
	}

	return result
}

// lookupTranslation finds the translation for a key with fallback chain.
// Order: appTrans[lang] -> appTrans["en"] -> libraryTranslations[lang] -> libraryTranslations["en"] -> key
func lookupTranslation(key, lang string, appTrans map[string]map[string]string) string {
	// Try app translations for current language first
	if appTrans != nil {
		if langMap, ok := appTrans[lang]; ok {
			if val, ok := langMap[key]; ok {
				return val
			}
		}
		// Fallback to app's English translation
		if lang != "en" {
			if langMap, ok := appTrans["en"]; ok {
				if val, ok := langMap[key]; ok {
					return val
				}
			}
		}
	}

	// Try library translations for current language
	if langMap, ok := libraryTranslations[lang]; ok {
		if val, ok := langMap[key]; ok {
			return val
		}
	}

	// Fallback to library's English translation
	if lang != "en" {
		if langMap, ok := libraryTranslations["en"]; ok {
			if val, ok := langMap[key]; ok {
				return val
			}
		}
	}

	// Return key itself as last resort
	return key
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

// getLibraryTranslationsJS returns JavaScript code that defines library translations.
// This is injected into the HTML page for frontend translation support.
func getLibraryTranslationsJS() string {
	js, err := json.Marshal(libraryTranslations)
	if err != nil {
		return "const libraryTranslations = {};"
	}
	return "const libraryTranslations = " + string(js) + ";"
}
