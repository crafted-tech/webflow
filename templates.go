package webflow

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"strings"
)

// selectChevron is the SVG chevron icon for select dropdowns.
// Uses currentColor to inherit text color from CSS.
const selectChevron = `<svg class="select-chevron" xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 12 12" aria-hidden="true"><path fill="currentColor" d="M3 4L6 8L9 4z"/></svg>`

// renderPage generates the complete HTML for a flow page.
// Translation is performed immediately by T()/TF() - no frontend translation needed.
// Call SetLanguage() before calling this function to ensure correct language.
func renderPage(page Page, darkMode bool, primaryLight, primaryDark string) string {
	// T() and TF() translate strings immediately using the package-level currentLanguage.
	// The frontend still needs i18n.js for the language selector to display language names.

	var buf bytes.Buffer

	theme := "light"
	if darkMode {
		theme = "dark"
	}

	// Build CSS with optional color overrides
	css := cssContent
	if primaryLight != "" || primaryDark != "" {
		var colorCSS strings.Builder
		colorCSS.WriteString("\n:root {")
		if primaryLight != "" {
			colorCSS.WriteString("\n    --primary: " + primaryLight + ";")
			colorCSS.WriteString("\n    --ring: " + primaryLight + ";")
		}
		colorCSS.WriteString("\n}")
		if primaryDark != "" {
			colorCSS.WriteString("\n[data-theme=\"dark\"] {")
			colorCSS.WriteString("\n    --primary: " + primaryDark + ";")
			colorCSS.WriteString("\n    --ring: " + primaryDark + ";")
			colorCSS.WriteString("\n}")
		}
		css += colorCSS.String()
	}

	buf.WriteString(`<!DOCTYPE html>
<html lang="en" data-theme="` + theme + `">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>` + css + `</style>
</head>
<body>
    <div class="flow-container">
`)

	// Header
	buf.WriteString(`        <div class="flow-header">
`)
	if page.Icon != "" {
		buf.WriteString(renderIcon(page.Icon))
	}
	if page.Title != "" {
		buf.WriteString(`            <h1 class="flow-title">` + html.EscapeString(page.Title) + `</h1>
`)
	}
	if page.Subtitle != "" {
		buf.WriteString(`            <p class="flow-subtitle">` + html.EscapeString(page.Subtitle) + `</p>
`)
	}
	buf.WriteString(`        </div>
`)

	// Content
	contentHTML, needsPassthrough := renderContent(page.Content)
	contentClass := "flow-content"
	if needsPassthrough {
		contentClass += " flow-content-passthrough"
	}
	buf.WriteString(`        <div class="` + contentClass + `">
`)
	buf.WriteString(contentHTML)
	buf.WriteString(`        </div>
`)

	// Footer with buttons - prefer ButtonBar over legacy Buttons array
	buf.WriteString(renderButtonBar(page))

	buf.WriteString(`    </div>
    <script>` + jsContent + `</script>
</body>
</html>`)

	return buf.String()
}

// renderIcon renders an icon based on the icon name or SVG content.
func renderIcon(icon string) string {
	var svg string
	iconClass := ""

	switch icon {
	case "info":
		iconClass = "icon-info"
		svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/></svg>`
	case "warning":
		iconClass = "icon-warning"
		svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2v-4h2v4z"/></svg>`
	case "error":
		iconClass = "icon-error"
		svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/></svg>`
	case "success":
		iconClass = "icon-success"
		svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/></svg>`
	default:
		// Assume it's custom SVG content
		if strings.HasPrefix(icon, "<svg") {
			svg = icon
		} else {
			return ""
		}
	}

	return fmt.Sprintf(`            <div class="flow-icon %s">%s</div>
`, iconClass, svg)
}

// renderContent renders the page content based on its type.
// Returns (html, needsPassthrough) where needsPassthrough indicates the content
// handles its own scrolling and the parent should use overflow:hidden.
func renderContent(content any) (string, bool) {
	if content == nil {
		return "", false
	}

	switch c := content.(type) {
	case string:
		return renderMessage(c), false
	case []Choice:
		return renderChoiceList(c), false
	case MultiChoice:
		return renderMultiChoiceList(c), false
	case []MenuItem:
		return renderMenuList(c), false
	case []FormField:
		return renderForm(c), false
	case ProgressConfig:
		return renderProgress(), false
	case LogConfig:
		return renderLogView(), true
	case FileListConfig:
		return renderFileListView(), true
	case ReviewConfig:
		return renderReviewView(c), true
	case WelcomeConfig:
		return renderWelcomeView(c), false
	case LicenseConfig:
		return renderLicenseView(c), true
	case ConfirmCheckboxConfig:
		return renderConfirmCheckboxView(c), false
	case SummaryConfig:
		return renderSummaryView(c), false
	default:
		return "", false
	}
}

// renderMessage renders a simple text message.
func renderMessage(message string) string {
	return `            <p class="flow-message">` + html.EscapeString(message) + `</p>
`
}

// renderChoiceList renders a list of selectable choices (radio buttons).
func renderChoiceList(choices []Choice) string {
	var buf bytes.Buffer
	buf.WriteString(`            <div class="choice-list">
`)
	for i, choice := range choices {
		checked := ""
		autofocus := ""
		if i == 0 {
			checked = " checked"
			autofocus = " autofocus"
		}
		value := choice.Value
		if value == "" {
			value = choice.Label
		}
		inputID := fmt.Sprintf("choice-%d", i)
		buf.WriteString(fmt.Sprintf(`                <label class="choice-item" for="%s">
                    <input type="radio" id="%s" name="choice" value="%s" data-index="%d"%s%s>
                    <span class="choice-radio"></span>
                    <span class="choice-content">
                        <span class="choice-label">%s</span>
`, inputID, inputID, html.EscapeString(value), i, checked, autofocus, html.EscapeString(choice.Label)))
		if choice.Description != "" {
			buf.WriteString(fmt.Sprintf(`                        <span class="choice-description">%s</span>
`, html.EscapeString(choice.Description)))
		}
		buf.WriteString(`                    </span>
                </label>
`)
	}
	buf.WriteString(`            </div>
`)
	return buf.String()
}

// renderMultiChoiceList renders a list of checkboxes for multi-selection.
func renderMultiChoiceList(mc MultiChoice) string {
	// Build a set of selected indices for quick lookup
	selectedSet := make(map[int]bool)
	for _, idx := range mc.Selected {
		selectedSet[idx] = true
	}

	var buf bytes.Buffer
	buf.WriteString(`            <div class="choice-list choice-list-multi">
`)
	for i, choice := range mc.Choices {
		checked := ""
		if selectedSet[i] {
			checked = " checked"
		}
		autofocus := ""
		if i == 0 {
			autofocus = " autofocus"
		}
		value := choice.Value
		if value == "" {
			value = choice.Label
		}
		inputID := fmt.Sprintf("choice-%d", i)
		buf.WriteString(fmt.Sprintf(`                <label class="choice-item" for="%s">
                    <input type="checkbox" id="%s" name="choice-%d" value="%s" data-index="%d"%s%s>
                    <span class="choice-checkbox"></span>
                    <span class="choice-content">
                        <span class="choice-label">%s</span>
`, inputID, inputID, i, html.EscapeString(value), i, checked, autofocus, html.EscapeString(choice.Label)))
		if choice.Description != "" {
			buf.WriteString(fmt.Sprintf(`                        <span class="choice-description">%s</span>
`, html.EscapeString(choice.Description)))
		}
		buf.WriteString(`                    </span>
                </label>
`)
	}
	buf.WriteString(`            </div>
`)
	return buf.String()
}

// renderMenuList renders a list of clickable menu items.
func renderMenuList(items []MenuItem) string {
	var buf bytes.Buffer
	buf.WriteString(`            <div class="menu-list">
`)
	for i, item := range items {
		buf.WriteString(fmt.Sprintf(`                <button type="button" class="menu-item" data-index="%d">
`, i))
		if item.Icon != "" {
			buf.WriteString(fmt.Sprintf(`                    <div class="menu-icon">%s</div>
`, renderMenuIcon(item.Icon)))
		}
		buf.WriteString(`                    <div class="menu-content">
`)
		buf.WriteString(fmt.Sprintf(`                        <div class="menu-title">%s</div>
`, html.EscapeString(item.Title)))
		if item.Description != "" {
			buf.WriteString(fmt.Sprintf(`                        <div class="menu-description">%s</div>
`, html.EscapeString(item.Description)))
		}
		buf.WriteString(`                    </div>
                </button>
`)
	}
	buf.WriteString(`            </div>
`)
	return buf.String()
}

// renderMenuIcon renders an icon for menu items.
func renderMenuIcon(icon string) string {
	// Check for built-in icon names
	switch icon {
	case "info":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/></svg>`
	case "warning":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2v-4h2v4z"/></svg>`
	case "error":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/></svg>`
	case "success":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/></svg>`
	case "settings":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M19.14,12.94c0.04-0.3,0.06-0.61,0.06-0.94c0-0.32-0.02-0.64-0.07-0.94l2.03-1.58c0.18-0.14,0.23-0.41,0.12-0.61 l-1.92-3.32c-0.12-0.22-0.37-0.29-0.59-0.22l-2.39,0.96c-0.5-0.38-1.03-0.7-1.62-0.94L14.4,2.81c-0.04-0.24-0.24-0.41-0.48-0.41 h-3.84c-0.24,0-0.43,0.17-0.47,0.41L9.25,5.35C8.66,5.59,8.12,5.92,7.63,6.29L5.24,5.33c-0.22-0.08-0.47,0-0.59,0.22L2.74,8.87 C2.62,9.08,2.66,9.34,2.86,9.48l2.03,1.58C4.84,11.36,4.8,11.69,4.8,12s0.02,0.64,0.07,0.94l-2.03,1.58 c-0.18,0.14-0.23,0.41-0.12,0.61l1.92,3.32c0.12,0.22,0.37,0.29,0.59,0.22l2.39-0.96c0.5,0.38,1.03,0.7,1.62,0.94l0.36,2.54 c0.05,0.24,0.24,0.41,0.48,0.41h3.84c0.24,0,0.44-0.17,0.47-0.41l0.36-2.54c0.59-0.24,1.13-0.56,1.62-0.94l2.39,0.96 c0.22,0.08,0.47,0,0.59-0.22l1.92-3.32c0.12-0.22,0.07-0.47-0.12-0.61L19.14,12.94z M12,15.6c-1.98,0-3.6-1.62-3.6-3.6 s1.62-3.6,3.6-3.6s3.6,1.62,3.6,3.6S13.98,15.6,12,15.6z"/></svg>`
	case "folder":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/></svg>`
	case "file":
		return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M14 2H6c-1.1 0-1.99.9-1.99 2L4 20c0 1.1.89 2 1.99 2H18c1.1 0 2-.9 2-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z"/></svg>`
	default:
		// Assume it's custom SVG content
		if strings.HasPrefix(icon, "<svg") {
			return icon
		}
		return ""
	}
}

// renderForm renders a form with input fields.
func renderForm(fields []FormField) string {
	var buf bytes.Buffer
	buf.WriteString(`            <form class="flow-form">
`)
	for _, field := range fields {
		buf.WriteString(renderFormField(field))
	}
	buf.WriteString(`            </form>
`)
	return buf.String()
}

// renderFormField renders a single form field.
func renderFormField(field FormField) string {
	var buf bytes.Buffer

	switch field.Type {
	case FieldText, FieldPassword, FieldPath:
		inputType := "text"
		if field.Type == FieldPassword {
			inputType = "password"
		}

		buf.WriteString(fmt.Sprintf(`                <div class="form-group">
                    <label class="form-label" for="%s">%s</label>
`, html.EscapeString(field.ID), html.EscapeString(field.Label)))

		if field.Type == FieldPath {
			buf.WriteString(`                    <div class="form-path-group">
`)
		}

		defaultVal := ""
		if field.Default != nil {
			defaultVal = fmt.Sprintf("%v", field.Default)
		}

		required := ""
		if field.Required {
			required = " required"
		}

		placeholder := ""
		if field.Placeholder != "" {
			placeholder = fmt.Sprintf(` placeholder="%s"`, html.EscapeString(field.Placeholder))
		}

		buf.WriteString(fmt.Sprintf(`                    <input type="%s" id="%s" class="form-input" value="%s"%s%s>
`, inputType, html.EscapeString(field.ID), html.EscapeString(defaultVal), placeholder, required))

		if field.Type == FieldPath {
			buf.WriteString(fmt.Sprintf(`                        <button type="button" class="btn btn-default" onclick="window.browseFolder('%s')">Browse</button>
                    </div>
`, html.EscapeString(field.ID)))
		}

		buf.WriteString(`                </div>
`)

	case FieldTextArea:
		defaultVal := ""
		if field.Default != nil {
			defaultVal = fmt.Sprintf("%v", field.Default)
		}

		required := ""
		if field.Required {
			required = " required"
		}

		placeholder := ""
		if field.Placeholder != "" {
			placeholder = fmt.Sprintf(` placeholder="%s"`, html.EscapeString(field.Placeholder))
		}

		buf.WriteString(fmt.Sprintf(`                <div class="form-group">
                    <label class="form-label" for="%s">%s</label>
                    <textarea id="%s" class="form-input form-textarea"%s%s>%s</textarea>
                </div>
`, html.EscapeString(field.ID), html.EscapeString(field.Label), html.EscapeString(field.ID), placeholder, required, html.EscapeString(defaultVal)))

	case FieldCheckbox:
		checked := ""
		if field.Default == true {
			checked = " checked"
		}

		buf.WriteString(fmt.Sprintf(`                <div class="form-group">
                    <div class="form-checkbox-group">
                        <input type="checkbox" id="%s" class="form-checkbox"%s>
                        <label class="form-label" for="%s">%s</label>
                    </div>
                </div>
`, html.EscapeString(field.ID), checked, html.EscapeString(field.ID), html.EscapeString(field.Label)))

	case FieldSelect:
		buf.WriteString(fmt.Sprintf(`                <div class="form-group-inline">
                    <label class="form-label" for="%s">%s</label>
                    <div class="select-wrapper">
                        <select id="%s" class="form-input">
`, html.EscapeString(field.ID), html.EscapeString(field.Label), html.EscapeString(field.ID)))

		defaultVal := ""
		if field.Default != nil {
			defaultVal = fmt.Sprintf("%v", field.Default)
		}

		for _, opt := range field.Options {
			selected := ""
			if opt == defaultVal {
				selected = " selected"
			}
			buf.WriteString(fmt.Sprintf(`                            <option value="%s"%s>%s</option>
`, html.EscapeString(opt), selected, html.EscapeString(opt)))
		}

		buf.WriteString(`                        </select>
                        ` + selectChevron + `
                    </div>
                </div>
`)
	}

	return buf.String()
}

// renderProgress renders a progress bar.
func renderProgress() string {
	return `            <div class="progress-container">
                <div class="progress-bar-wrapper">
                    <div class="progress-bar" style="width: 0%"></div>
                </div>
                <p class="progress-status">Starting...</p>
            </div>
`
}

// renderLogView renders a live log/console view.
func renderLogView() string {
	return `            <div class="log-container">
                <div class="log-content" id="log-content"></div>
                <div class="log-status" id="log-status"></div>
            </div>
`
}

// renderFileListView renders a file progress list view.
func renderFileListView() string {
	return `            <div class="filelist-container">
                <div class="filelist-progress" id="filelist-progress"></div>
                <div class="filelist-content" id="filelist-content"></div>
                <div class="filelist-status" id="filelist-status"></div>
            </div>
`
}

// renderReviewView renders a text review/viewer.
// Copy/Save buttons are rendered in the ButtonBar, not here.
func renderReviewView(cfg ReviewConfig) string {
	var buf bytes.Buffer
	buf.WriteString(`            <div class="review-container">
`)
	if cfg.Subtitle != "" {
		buf.WriteString(fmt.Sprintf(`                <div class="review-subtitle">%s</div>
`, html.EscapeString(cfg.Subtitle)))
	}
	buf.WriteString(fmt.Sprintf(`                <div class="review-content">%s</div>
            </div>
`, html.EscapeString(cfg.Content)))
	return buf.String()
}

// renderWelcomeView renders a welcome page with optional logo and language selector.
func renderWelcomeView(cfg WelcomeConfig) string {
	var buf bytes.Buffer

	buf.WriteString(`            <div class="welcome-container">
`)
	// Logo
	if len(cfg.Logo) > 0 {
		logoHeight := cfg.LogoHeight
		if logoHeight == 0 {
			logoHeight = 64
		}
		// Check if it's SVG or PNG based on content
		logoData := string(cfg.Logo)
		if strings.HasPrefix(logoData, "<svg") || strings.HasPrefix(logoData, "<?xml") {
			// SVG - render inline
			buf.WriteString(fmt.Sprintf(`                <div class="welcome-logo" style="height: %dpx;">%s</div>
`, logoHeight, logoData))
		} else {
			// Binary data (PNG/etc) - use data URI
			// For simplicity, assume PNG
			encoded := "data:image/png;base64," + encodeBase64(cfg.Logo)
			buf.WriteString(fmt.Sprintf(`                <div class="welcome-logo"><img src="%s" alt="Logo" style="height: %dpx;"></div>
`, encoded, logoHeight))
		}
	}

	// Title
	if cfg.Title != "" {
		// Don't HTML-escape translation strings - they contain control characters
		// and JSON that need to be parsed by JavaScript as-is
		buf.WriteString(fmt.Sprintf(`                <h2 class="welcome-title">%s</h2>
`, cfg.Title))
	}

	// Message
	if cfg.Message != "" {
		// Don't HTML-escape translation strings - they contain control characters
		// and JSON that need to be parsed by JavaScript as-is
		// Convert newlines to <br> for display
		formattedMsg := strings.ReplaceAll(cfg.Message, "\n", "<br>")
		buf.WriteString(fmt.Sprintf(`                <p class="welcome-message">%s</p>
`, formattedMsg))
	}

	// Language selector
	if cfg.LanguageSelector {
		buf.WriteString(`                <div class="welcome-language">
                    <label class="form-label" for="language-select">` + T("welcome.languageLabel") + `</label>
                    <div class="select-wrapper">
                        <select id="language-select" class="form-input" onchange="window.changeLanguage(this.value)">
`)
		// Render language options (backend provides full list)
		langMu.RLock()
		currLang := currentLanguage
		langMu.RUnlock()
		for _, lang := range GetAvailableLanguages() {
			selected := ""
			if lang.Code == currLang {
				selected = " selected"
			}
			buf.WriteString(fmt.Sprintf(`                            <option value="%s"%s>%s</option>
`, html.EscapeString(lang.Code), selected, html.EscapeString(lang.Name)))
		}
		buf.WriteString(`                        </select>
                        ` + selectChevron + `
                    </div>
                </div>
`)
	}

	buf.WriteString(`            </div>
`)
	return buf.String()
}

// renderLicenseView renders a license agreement page.
func renderLicenseView(cfg LicenseConfig) string {
	var buf bytes.Buffer

	// Top label
	if cfg.Label != "" {
		buf.WriteString(fmt.Sprintf(`            <p class="license-label">%s</p>
`, html.EscapeString(cfg.Label)))
	}

	// License content in a bordered scrollable area
	buf.WriteString(fmt.Sprintf(`            <div class="license-content">%s</div>
`, html.EscapeString(cfg.Content)))

	// Bottom instruction label (button text is embedded in translation)
	instruction := T("license.instruction")
	if instruction != "" {
		buf.WriteString(fmt.Sprintf(`            <p class="license-instruction">%s</p>
`, html.EscapeString(instruction)))
	}

	return buf.String()
}

// renderConfirmCheckboxView renders a confirmation dialog with a required checkbox.
func renderConfirmCheckboxView(cfg ConfirmCheckboxConfig) string {
	var buf bytes.Buffer

	// Message
	if cfg.Message != "" {
		escapedMsg := html.EscapeString(cfg.Message)
		formattedMsg := strings.ReplaceAll(escapedMsg, "\n", "<br>")
		buf.WriteString(fmt.Sprintf(`            <p class="flow-message">%s</p>
`, formattedMsg))
	}

	// Warning message (if any)
	if cfg.WarningMessage != "" {
		icon := GetIcon("warning")
		escapedWarn := html.EscapeString(cfg.WarningMessage)
		formattedWarn := strings.ReplaceAll(escapedWarn, "\n", "<br>")
		buf.WriteString(fmt.Sprintf(`            <div class="summary-alert summary-alert-warning">
                <span class="summary-alert-icon">%s</span>
                <span class="summary-alert-text">%s</span>
            </div>
`, icon, formattedWarn))
	}

	// Required checkbox
	if cfg.CheckboxLabel != "" {
		buf.WriteString(fmt.Sprintf(`            <div class="form-group">
                <div class="form-checkbox-group">
                    <input type="checkbox" id="_confirm_checkbox" class="form-checkbox" onchange="window.updateConfirmButton(this.checked)">
                    <label class="form-label" for="_confirm_checkbox">%s</label>
                </div>
            </div>
`, html.EscapeString(cfg.CheckboxLabel)))
	}

	return buf.String()
}

// renderSummaryView renders a summary with labeled key-value pairs and optional checkboxes.
// Labels can contain translation keys (with \x01 prefix) which the frontend will translate.
// Values are rendered as literal text.
// Items with AlertType set are rendered as alert boxes with icons.
func renderSummaryView(cfg SummaryConfig) string {
	var buf bytes.Buffer

	// Separate regular items from alert items
	var regularItems []SummaryItem
	var alertItems []SummaryItem
	for _, item := range cfg.Items {
		if item.AlertType != "" {
			alertItems = append(alertItems, item)
		} else {
			regularItems = append(regularItems, item)
		}
	}

	// Render regular key-value pairs
	if len(regularItems) > 0 {
		buf.WriteString(`            <dl class="summary-list">
`)
		for _, item := range regularItems {
			// Handle multiline values (convert newlines to <br>)
			escapedValue := html.EscapeString(item.Value)
			formattedValue := strings.ReplaceAll(escapedValue, "\n", "<br>")
			buf.WriteString(fmt.Sprintf(`                <dt>%s</dt>
                <dd>%s</dd>
`, html.EscapeString(item.Label), formattedValue))
		}
		buf.WriteString(`            </dl>
`)
	}

	// Render alert items
	for _, item := range alertItems {
		icon := GetIcon(string(item.AlertType))
		escapedValue := html.EscapeString(item.Value)
		formattedValue := strings.ReplaceAll(escapedValue, "\n", "<br>")
		buf.WriteString(fmt.Sprintf(`            <div class="summary-alert summary-alert-%s">
                <span class="summary-alert-icon">%s</span>
                <span class="summary-alert-text">%s</span>
            </div>
`, item.AlertType, icon, formattedValue))
	}

	// Render checkboxes if any
	if len(cfg.Checkboxes) > 0 {
		// Track whether any checkboxes are required (for button disabling)
		hasRequired := false
		for _, cb := range cfg.Checkboxes {
			if cb.Required {
				hasRequired = true
				break
			}
		}

		buf.WriteString(`            <div class="summary-checkboxes">
`)
		for _, cb := range cfg.Checkboxes {
			// Warning box (if present)
			if cb.Warning != "" {
				icon := GetIcon("warning")
				escapedWarn := html.EscapeString(cb.Warning)
				formattedWarn := strings.ReplaceAll(escapedWarn, "\n", "<br>")
				buf.WriteString(fmt.Sprintf(`                <div class="summary-alert summary-alert-warning">
                    <span class="summary-alert-icon">%s</span>
                    <span class="summary-alert-text">%s</span>
                </div>
`, icon, formattedWarn))
			}

			// Checkbox with data attributes for JS
			requiredAttr := ""
			if cb.Required {
				requiredAttr = ` data-required="true"`
			}
			buf.WriteString(fmt.Sprintf(`                <div class="form-group">
                    <div class="form-checkbox-group">
                        <input type="checkbox" id="%s" class="form-checkbox summary-checkbox"%s onchange="window.updateSummaryCheckboxes()">
                        <label class="form-label" for="%s">%s</label>
                    </div>
                </div>
`, html.EscapeString(cb.ID), requiredAttr, html.EscapeString(cb.ID), html.EscapeString(cb.Label)))
		}
		buf.WriteString(`            </div>
`)

		// Add data attribute to enable JS tracking if any required checkboxes
		if hasRequired {
			buf.WriteString(`            <script>window._summaryHasRequiredCheckboxes = true;</script>
`)
		}
	}

	return buf.String()
}

// encodeBase64 encodes bytes to base64 string.
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// renderButtonBar renders the button bar with fixed positions.
// Layout (Linux/macOS): [Actions...] [Left] ... [spacer] ... [Back] [Next] [Close]
func renderButtonBar(page Page) string {
	bb := page.ButtonBar

	// Check if ButtonBar is empty (all nil) - fall back to legacy Buttons
	hasButtonBar := bb.Left != nil || bb.Back != nil || bb.Next != nil || bb.Close != nil || len(bb.Actions) > 0
	if !hasButtonBar && len(page.Buttons) > 0 {
		// Legacy mode: render buttons array
		var buf bytes.Buffer
		buf.WriteString(`        <div class="flow-footer">
`)
		for _, btn := range page.Buttons {
			buf.WriteString(renderButton(&btn))
		}
		buf.WriteString(`        </div>
`)
		return buf.String()
	}

	if !hasButtonBar {
		return "" // No buttons at all
	}

	var buf bytes.Buffer
	buf.WriteString(`        <div class="flow-footer">
`)

	// Action buttons (e.g., Copy, Save icons)
	for _, btn := range bb.Actions {
		buf.WriteString(renderButton(btn))
	}

	// Left button (helper, e.g., Help)
	if bb.Left != nil {
		buf.WriteString(renderButton(bb.Left))
	}

	// Spacer to push remaining buttons to the right
	buf.WriteString(`            <div class="button-spacer"></div>
`)

	// Back button
	if bb.Back != nil {
		buf.WriteString(renderButton(bb.Back))
	}

	// Next/primary action button
	if bb.Next != nil {
		buf.WriteString(renderButton(bb.Next))
	}

	// Close button
	if bb.Close != nil {
		buf.WriteString(renderButton(bb.Close))
	}

	buf.WriteString(`        </div>
`)
	return buf.String()
}

// renderButton renders a single button element.
func renderButton(btn *Button) string {
	if btn == nil {
		return ""
	}

	btnClass := "btn"

	// Check new Style field first, then fall back to deprecated Primary/Danger
	switch btn.Style {
	case ButtonPrimary:
		btnClass += " btn-primary"
	case ButtonDanger:
		btnClass += " btn-destructive"
	default:
		// Check deprecated fields for backwards compatibility
		if btn.Primary {
			btnClass += " btn-primary"
		} else if btn.Danger {
			btnClass += " btn-destructive"
		} else {
			btnClass += " btn-default"
		}
	}

	// Icon-only button style
	if btn.IconOnly {
		btnClass += " btn-icon"
	}

	// Handle disabled state
	disabled := ""
	if !btn.Enabled {
		btnClass += " btn-disabled"
		disabled = " disabled"
	}

	// Build button content
	var content string
	if btn.Icon != "" {
		if btn.IconOnly {
			// Icon only - label becomes title for accessibility
			content = fmt.Sprintf(`<span class="btn-icon-wrap">%s</span>`, btn.Icon)
			return fmt.Sprintf(`            <button type="button" class="%s" data-button="%s" title="%s"%s>%s</button>
`, btnClass, html.EscapeString(btn.ID), html.EscapeString(btn.Label), disabled, content)
		}
		// Icon + label
		content = fmt.Sprintf(`<span class="btn-icon-wrap">%s</span><span>%s</span>`, btn.Icon, html.EscapeString(btn.Label))
	} else {
		content = html.EscapeString(btn.Label)
	}

	return fmt.Sprintf(`            <button type="button" class="%s" data-button="%s"%s>%s</button>
`, btnClass, html.EscapeString(btn.ID), disabled, content)
}
