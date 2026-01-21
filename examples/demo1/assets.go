package main

import (
	_ "embed"
	"encoding/json"
)

//go:embed assets/logo.svg
var logoSVG []byte

//go:embed assets/license.txt
var licenseText string

//go:embed assets/translations.json
var translationsJSON []byte

// loadTranslations parses the embedded translations JSON.
func loadTranslations() map[string]map[string]string {
	var translations map[string]map[string]string
	if err := json.Unmarshal(translationsJSON, &translations); err != nil {
		// Return empty map on error
		return make(map[string]map[string]string)
	}
	return translations
}
