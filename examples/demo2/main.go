// Demo2 - Component Gallery showcasing all webflow UI elements.
// Unlike demo1 (linear installer flow), demo2 is menu-driven to explore each component independently.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/crafted-tech/webflow"
)

func main() {
	f, err := webflow.New(
		webflow.WithTitle("Component Gallery"),
		webflow.WithSize("50em", "38em"),
		webflow.WithResizable(true),
	)
	if err != nil {
		log.Fatalf("Failed to create flow: %v", err)
	}
	defer f.Close()

	// Main loop - return to menu after each demo
	for {
		idx := showMainMenu(f)
		switch idx {
		case 0:
			demoPageTypes(f)
		case 1:
			demoSelections(f)
		case 2:
			demoForms(f)
		case 3:
			demoProgress(f)
		case 4:
			demoAlerts(f)
		case 5:
			demoButtons(f)
		case 6:
			demoDialogs(f)
		case -1:
			return // Exit
		}
	}
}

func showMainMenu(f *webflow.Flow) int {
	resp := f.ShowMenu("Component Gallery", []webflow.MenuItem{
		{Title: "Page Types", Description: "Message, Welcome, License, Confirm dialogs", Icon: "file"},
		{Title: "Selection", Description: "Choice, Choices, MultiChoice, Menu", Icon: "folder"},
		{Title: "Forms", Description: "All field types: Text, Password, Checkbox, Select, Path, TextArea", Icon: "settings"},
		{Title: "Progress Views", Description: "Progress bar, File progress, Log viewer", Icon: "info"},
		{Title: "Alerts & Summaries", Description: "All alert types: Info, Warning, Error, Success", Icon: "warning"},
		{Title: "Buttons & Icons", Description: "Styles, states, presets, icon buttons", Icon: "success"},
		{Title: "Dialogs", Description: "Error, ErrorDetails, TextInput, FilePicker, Review", Icon: "error"},
	}, webflow.WithButtonBar(webflow.ButtonBar{
		Close: webflow.NewButton("Exit", webflow.ButtonClose),
	}))

	if webflow.IsClose(resp) {
		return -1
	}
	if idx, ok := resp.(int); ok {
		return idx
	}
	return -1
}

// =============================================================================
// Page Types Demo
// =============================================================================

func demoPageTypes(f *webflow.Flow) {
	type step int
	const (
		stepMenu step = iota
		stepMessage
		stepWelcome
		stepLicense
		stepConfirm
		stepConfirmCheckbox
	)

	current := stepMenu
	for {
		switch current {
		case stepMenu:
			resp := f.ShowMenu("Page Types", []webflow.MenuItem{
				{Title: "ShowMessage", Description: "Simple message with icon and subtitle", Icon: "info"},
				{Title: "ShowWelcome", Description: "Welcome page with logo", Icon: "success"},
				{Title: "ShowLicense", Description: "Scrollable license text with agreement", Icon: "file"},
				{Title: "ShowConfirm", Description: "Yes/No confirmation dialog", Icon: "warning"},
				{Title: "ShowConfirmWithCheckbox", Description: "Confirmation with required checkbox", Icon: "error"},
			}, webflow.WithButtonBar(webflow.ButtonBar{
				Back:  webflow.NewButton("Back", webflow.ButtonBack),
				Close: webflow.NewButton("Close", webflow.ButtonClose),
			}))
			if webflow.IsClose(resp) {
				os.Exit(0)
			}
			if webflow.IsBack(resp) {
				return
			}
			if idx, ok := resp.(int); ok {
				current = step(idx + 1)
			}

		case stepMessage:
			f.ShowMessage(
				"ShowMessage Demo",
				"This demonstrates the ShowMessage method with an info icon and subtitle.\n\nUse this for simple informational displays.",
				webflow.WithIcon("info"),
				webflow.WithSubtitle("This is a subtitle"),
				webflow.WithButtonBar(webflow.SimpleOK()),
			)
			current = stepMenu

		case stepWelcome:
			f.ShowWelcome(webflow.WelcomeConfig{
				Logo:             logoSVG,
				LogoHeight:       64,
				Title:            "Welcome to Component Gallery",
				Message:          "This demo showcases all webflow UI components.\n\nClick Next to continue exploring.",
				LanguageSelector: false,
			})
			current = stepMenu

		case stepLicense:
			f.ShowLicense(webflow.LicenseConfig{
				Title:   "License Agreement",
				Label:   "Please read and accept the license agreement:",
				Content: sampleLicenseText,
			}, webflow.WithButtonBar(webflow.ButtonBar{
				Back:  webflow.NewButton("Back", webflow.ButtonBack),
				Next:  webflow.NewButton("I Agree", webflow.ButtonNext).WithPrimary(),
				Close: webflow.NewButton("Close", webflow.ButtonClose),
			}))
			current = stepMenu

		case stepConfirm:
			result := f.ShowConfirm(
				"Confirm Action",
				"Do you want to proceed with this action?\n\nThis demonstrates the ShowConfirm method.",
			)
			if b, ok := result.(bool); ok {
				if b {
					f.ShowMessage("Confirmed", "You clicked Yes!", webflow.WithIcon("success"))
				} else {
					f.ShowMessage("Declined", "You clicked No.", webflow.WithIcon("info"))
				}
			}
			current = stepMenu

		case stepConfirmCheckbox:
			f.ShowConfirmWithCheckbox(webflow.ConfirmCheckboxConfig{
				Title:          "Important Confirmation",
				Message:        "This action requires explicit acknowledgment before proceeding.",
				CheckboxLabel:  "I understand and accept the terms",
				WarningMessage: "This action cannot be undone. Please make sure you understand the implications.",
			})
			current = stepMenu
		}
	}
}

// =============================================================================
// Selection Demo
// =============================================================================

func demoSelections(f *webflow.Flow) {
	type step int
	const (
		stepMenu step = iota
		stepChoice
		stepMultiChoice
		stepMenuDemo
	)

	current := stepMenu
	for {
		switch current {
		case stepMenu:
			resp := f.ShowMenu("Selection Components", []webflow.MenuItem{
				{Title: "ShowChoice", Description: "Single selection with optional descriptions", Icon: "folder"},
				{Title: "ShowMultiChoice", Description: "Multi-selection with optional descriptions", Icon: "success"},
				{Title: "ShowMenu", Description: "Clickable menu items with icons", Icon: "settings"},
			}, webflow.WithButtonBar(webflow.ButtonBar{
				Back:  webflow.NewButton("Back", webflow.ButtonBack),
				Close: webflow.NewButton("Close", webflow.ButtonClose),
			}))
			if webflow.IsClose(resp) {
				os.Exit(0)
			}
			if webflow.IsBack(resp) {
				return
			}
			if idx, ok := resp.(int); ok {
				current = step(idx + 1)
			}

		case stepChoice:
			resp := f.ShowChoice(
				"Select a Plan",
				[]webflow.Choice{
					{Label: "Standard", Description: "Basic features, suitable for individuals"},
					{Label: "Professional", Description: "Advanced features, team collaboration"},
					{Label: "Enterprise", Description: "Full features, dedicated support"},
				},
				webflow.WithSubtitle("Selection with descriptions"),
				webflow.WithButtonBar(webflow.WizardMiddle()),
			)
			if !webflow.IsBack(resp) && !webflow.IsClose(resp) {
				if idx, ok := resp.(int); ok {
					plans := []string{"Standard", "Professional", "Enterprise"}
					f.ShowMessage("Plan Selected", fmt.Sprintf("You selected: %s", plans[idx]), webflow.WithIcon("success"))
				}
			}
			current = stepMenu

		case stepMultiChoice:
			resp := f.ShowMultiChoice(
				"Select Components",
				[]webflow.Choice{
					{Label: "Core Framework", Description: "Required base components"},
					{Label: "Documentation", Description: "User guides and API reference"},
					{Label: "Example Projects", Description: "Sample code and tutorials"},
					{Label: "Developer Tools", Description: "Build tools and debugging utilities"},
					{Label: "Desktop Integration", Description: "Native OS features and shortcuts"},
				},
				webflow.WithSubtitle("Select multiple items (with descriptions)"),
				webflow.WithButtonBar(webflow.WizardMiddle()),
			)
			if !webflow.IsBack(resp) && !webflow.IsClose(resp) {
				if indices, ok := resp.([]int); ok {
					f.ShowMessage("Components Selected", fmt.Sprintf("You selected %d component(s)", len(indices)), webflow.WithIcon("success"))
				}
			}
			current = stepMenu

		case stepMenuDemo:
			resp := f.ShowMenu("Settings Menu", []webflow.MenuItem{
				{Title: "Preferences", Description: "Configure application settings", Icon: "settings"},
				{Title: "File Management", Description: "Organize your files and folders", Icon: "folder"},
				{Title: "Help & Support", Description: "Get help and documentation", Icon: "info"},
				{Title: "About", Description: "Version and license information", Icon: "warning"},
			}, webflow.WithSubtitle("Click an item to select it"), webflow.WithButtonBar(webflow.ButtonBar{
				Back:  webflow.NewButton("Back", webflow.ButtonBack),
				Close: webflow.NewButton("Close", webflow.ButtonClose),
			}))
			if !webflow.IsBack(resp) && !webflow.IsClose(resp) {
				if idx, ok := resp.(int); ok {
					f.ShowMessage("Menu Item Selected", fmt.Sprintf("You selected menu item %d", idx+1), webflow.WithIcon("success"))
				}
			}
			current = stepMenu
		}
	}
}

// =============================================================================
// Forms Demo
// =============================================================================

func demoForms(f *webflow.Flow) {
	f.ShowMessage(
		"Form Components",
		"This demo shows all 6 form field types:\n\n- Text Input\n- Password Input\n- Checkbox\n- Select Dropdown\n- Path Picker\n- Text Area\n\nClick OK to see the form.",
		webflow.WithIcon("info"),
		webflow.WithButtonBar(webflow.SimpleOK()),
	)

	resp := f.ShowForm(
		"Complete Form Demo",
		[]webflow.FormField{
			{
				ID:          "text",
				Type:        webflow.FieldText,
				Label:       "Text Input",
				Placeholder: "Enter some text...",
				Default:     "",
			},
			{
				ID:          "password",
				Type:        webflow.FieldPassword,
				Label:       "Password",
				Placeholder: "Enter password...",
			},
			{
				ID:      "checkbox",
				Type:    webflow.FieldCheckbox,
				Label:   "Enable notifications",
				Default: true,
			},
			{
				ID:      "select",
				Type:    webflow.FieldSelect,
				Label:   "Select Option",
				Options: []string{"Option 1", "Option 2", "Option 3"},
				Default: "Option 1",
			},
			{
				ID:      "path",
				Type:    webflow.FieldPath,
				Label:   "Installation Path",
				Default: "",
			},
			{
				ID:          "textarea",
				Type:        webflow.FieldTextArea,
				Label:       "Comments",
				Placeholder: "Enter additional comments...",
				Default:     "",
			},
		},
		webflow.WithSubtitle("All field types in one form"),
		webflow.WithButtonBar(webflow.ButtonBar{
			Back:  webflow.NewButton("Back", webflow.ButtonBack),
			Next:  webflow.NewButton("Submit", webflow.ButtonNext).WithPrimary(),
			Close: webflow.NewButton("Close", webflow.ButtonClose),
		}),
	)

	if !webflow.IsBack(resp) && !webflow.IsClose(resp) {
		if data, ok := resp.(map[string]any); ok {
			// Show result summary
			var items []webflow.SummaryItem
			for k, v := range data {
				items = append(items, webflow.SummaryItem{
					Label: k,
					Value: fmt.Sprintf("%v", v),
				})
			}
			f.ShowMessage(
				"Form Submitted",
				webflow.SummaryConfig{Items: items},
				webflow.WithIcon("success"),
				webflow.WithButtonBar(webflow.SimpleOK()),
			)
		}
	}
}

// =============================================================================
// Progress Demo
// =============================================================================

func demoProgress(f *webflow.Flow) {
	type step int
	const (
		stepMenu step = iota
		stepProgress
		stepFileProgress
		stepLog
	)

	current := stepMenu
	for {
		switch current {
		case stepMenu:
			resp := f.ShowMenu("Progress Components", []webflow.MenuItem{
				{Title: "ShowProgress", Description: "Animated progress bar with cancellation", Icon: "info"},
				{Title: "ShowFileProgress", Description: "File list with status icons", Icon: "folder"},
				{Title: "ShowLog", Description: "Live log with all LogStyles", Icon: "file"},
			}, webflow.WithButtonBar(webflow.ButtonBar{
				Back:  webflow.NewButton("Back", webflow.ButtonBack),
				Close: webflow.NewButton("Close", webflow.ButtonClose),
			}))
			if webflow.IsClose(resp) {
				os.Exit(0)
			}
			if webflow.IsBack(resp) {
				return
			}
			if idx, ok := resp.(int); ok {
				current = step(idx + 1)
			}

		case stepProgress:
			resp := f.ShowProgress("Processing...", func(p webflow.Progress) {
				steps := []struct {
					percent float64
					status  string
				}{
					{0, "Initializing..."},
					{20, "Loading configuration..."},
					{40, "Processing data..."},
					{60, "Validating results..."},
					{80, "Finalizing..."},
					{100, "Complete!"},
				}

				for _, s := range steps {
					if p.Cancelled() {
						return
					}
					p.Update(s.percent, s.status)
					time.Sleep(500 * time.Millisecond)
				}
			})
			if webflow.IsClose(resp) {
				f.ShowMessage("Cancelled", "The operation was cancelled by the user.", webflow.WithIcon("warning"))
			} else {
				f.ShowMessage("Complete", "The operation completed successfully!", webflow.WithIcon("success"))
			}
			current = stepMenu

		case stepFileProgress:
			f.ShowFileProgress("Copying Files", func(files webflow.FileList) {
				demoFiles := []string{
					"src/main.go",
					"src/utils/helper.go",
					"config/settings.json",
					"docs/README.md",
					"tests/main_test.go",
				}

				for i, file := range demoFiles {
					if files.Cancelled() {
						return
					}

					files.SetProgress(i+1, len(demoFiles))
					files.AddFile(file, webflow.FileInProgress)
					files.SetCurrentFile(file)
					files.SetStatus(fmt.Sprintf("Copying: %s", file))

					time.Sleep(400 * time.Millisecond)

					// Randomly succeed or skip
					if i == 2 {
						files.UpdateFile(file, webflow.FileSkipped)
					} else {
						files.UpdateFile(file, webflow.FileComplete)
					}
				}

				files.SetStatus("All files copied successfully!")
				time.Sleep(500 * time.Millisecond)
			})
			current = stepMenu

		case stepLog:
			f.ShowLog("Installation Log", func(log webflow.LogWriter) {
				log.SetStatus("Running installation...")

				log.WriteLine("Starting installation process...")
				time.Sleep(300 * time.Millisecond)

				log.WriteLineStyled("[SUCCESS] Configuration loaded", webflow.LogSuccess)
				time.Sleep(300 * time.Millisecond)

				log.WriteLineStyled("[WARNING] Optional component not found, skipping", webflow.LogWarning)
				time.Sleep(300 * time.Millisecond)

				log.WriteLineStyled("[ERROR] Failed to create backup (non-critical)", webflow.LogError)
				time.Sleep(300 * time.Millisecond)

				log.WriteLineStyled("Note: This is a dim/muted message", webflow.LogDim)
				time.Sleep(300 * time.Millisecond)

				for i := 1; i <= 5; i++ {
					if log.Cancelled() {
						log.WriteLineStyled("Operation cancelled by user", webflow.LogWarning)
						return
					}
					log.WriteLine(fmt.Sprintf("Processing step %d of 5...", i))
					time.Sleep(400 * time.Millisecond)
				}

				log.WriteLineStyled("[SUCCESS] Installation completed!", webflow.LogSuccess)
				log.SetStatus("Done")
				time.Sleep(500 * time.Millisecond)
			})
			current = stepMenu
		}
	}
}

// =============================================================================
// Alerts Demo
// =============================================================================

func demoAlerts(f *webflow.Flow) {
	// Show all alert types in a SummaryConfig
	f.ShowMessage(
		"Alerts & Summary Items",
		webflow.SummaryConfig{
			Items: []webflow.SummaryItem{
				{Label: "Regular Item", Value: "This is a standard key-value pair"},
				{Value: "This is an AlertInfo box (blue) - for informational messages", AlertType: webflow.AlertInfo},
				{Value: "This is an AlertWarning box (yellow) - for warnings", AlertType: webflow.AlertWarning},
				{Value: "This is an AlertError box (red) - for errors", AlertType: webflow.AlertError},
				{Value: "This is an AlertSuccess box (green) - for success messages", AlertType: webflow.AlertSuccess},
			},
		},
		webflow.WithSubtitle("All AlertType values in SummaryConfig"),
		webflow.WithButtonBar(webflow.SimpleOK()),
	)
}

// =============================================================================
// Buttons Demo
// =============================================================================

func demoButtons(f *webflow.Flow) {
	type step int
	const (
		stepMenu step = iota
		stepStyles
		stepPresets
		stepIcons
	)

	current := stepMenu
	for {
		switch current {
		case stepMenu:
			resp := f.ShowMenu("Button Components", []webflow.MenuItem{
				{Title: "Button Styles", Description: "Normal, Primary, Danger, Disabled", Icon: "settings"},
				{Title: "ButtonBar Presets", Description: "WizardFirst, WizardMiddle, etc.", Icon: "folder"},
				{Title: "Icon Buttons", Description: "Buttons with icons, icon-only variants", Icon: "file"},
			}, webflow.WithButtonBar(webflow.ButtonBar{
				Back:  webflow.NewButton("Back", webflow.ButtonBack),
				Close: webflow.NewButton("Close", webflow.ButtonClose),
			}))
			if webflow.IsClose(resp) {
				os.Exit(0)
			}
			if webflow.IsBack(resp) {
				return
			}
			if idx, ok := resp.(int); ok {
				current = step(idx + 1)
			}

		case stepStyles:
			// Demonstrate button styles: Normal, Primary, Danger, Disabled
			f.ShowMessage(
				"Button Styles",
				"This page demonstrates all button styles:\n\n- Normal (Back button)\n- Primary (emphasized action)\n- Danger (destructive action)\n- Disabled (non-interactive)",
				webflow.WithSubtitle("Normal, Primary, Danger, and Disabled states"),
				webflow.WithButtonBar(webflow.ButtonBar{
					Back:  webflow.NewButton("Normal", webflow.ButtonBack),
					Next:  webflow.NewButton("Primary", webflow.ButtonNext).WithPrimary(),
					Close: webflow.NewButton("Danger", webflow.ButtonClose).WithDanger(),
					Left:  webflow.NewButton("Disabled", "disabled").Disabled(),
				}),
			)
			current = stepMenu

		case stepPresets:
			// Show each ButtonBar preset in sequence
			presets := []struct {
				name string
				bar  webflow.ButtonBar
			}{
				{"WizardFirst()", webflow.WizardFirst()},
				{"WizardMiddle()", webflow.WizardMiddle()},
				{"WizardInstall()", webflow.WizardInstall()},
				{"WizardFinish()", webflow.WizardFinish()},
				{"WizardLicense()", webflow.WizardLicense()},
				{"WizardProgress()", webflow.WizardProgress()},
				{"SimpleOK()", webflow.SimpleOK()},
				{"SimpleClose()", webflow.SimpleClose()},
				{"ConfirmYesNo()", webflow.ConfirmYesNo()},
			}

			for _, preset := range presets {
				f.ShowMessage(
					"ButtonBar Presets",
					fmt.Sprintf("Currently showing: %s", preset.name),
					webflow.WithSubtitle(preset.name),
					webflow.WithButtonBar(preset.bar),
				)
			}
			current = stepMenu

		case stepIcons:
			// Demonstrate icon buttons
			f.ShowMessage(
				"Icon Buttons",
				"This page shows icon buttons in the Actions area.\n\nThe left side has Copy, Download, and Info icon buttons.\n\nUse WithIcon() and AsIconOnly() to create these.",
				webflow.WithSubtitle("Actions with icon-only buttons"),
				webflow.WithButtonBar(webflow.ButtonBar{
					Actions: []*webflow.Button{
						webflow.NewButton("Copy", "copy").WithIcon(webflow.IconCopy).AsIconOnly(),
						webflow.NewButton("Download", "download").WithIcon(webflow.IconDownload).AsIconOnly(),
						webflow.NewButton("Info", "info").WithIcon(webflow.IconInfo).AsIconOnly(),
					},
					Back:  webflow.NewButton("Back", webflow.ButtonBack),
					Close: webflow.NewButton("Close", webflow.ButtonClose),
				}),
			)
			current = stepMenu
		}
	}
}

// =============================================================================
// Dialogs Demo
// =============================================================================

func demoDialogs(f *webflow.Flow) {
	type step int
	const (
		stepMenu step = iota
		stepError
		stepErrorDetails
		stepTextInput
		stepFileSave
		stepReview
	)

	current := stepMenu
	for {
		switch current {
		case stepMenu:
			resp := f.ShowMenu("Dialog Components", []webflow.MenuItem{
				{Title: "ShowError", Description: "Simple error message display", Icon: "error"},
				{Title: "ShowErrorDetails", Description: "Error with expandable details", Icon: "warning"},
				{Title: "ShowTextInput", Description: "Single text input dialog", Icon: "file"},
				{Title: "ShowFileSavePicker", Description: "Native file save dialog", Icon: "folder"},
				{Title: "ShowReview", Description: "Text viewer with Copy/Save buttons", Icon: "info"},
			}, webflow.WithButtonBar(webflow.ButtonBar{
				Back:  webflow.NewButton("Back", webflow.ButtonBack),
				Close: webflow.NewButton("Close", webflow.ButtonClose),
			}))
			if webflow.IsClose(resp) {
				os.Exit(0)
			}
			if webflow.IsBack(resp) {
				return
			}
			if idx, ok := resp.(int); ok {
				current = step(idx + 1)
			}

		case stepError:
			f.ShowError("Error Occurred", "An error occurred while processing your request.\n\nThis demonstrates the ShowError method which displays a simple error message with an OK button.")
			current = stepMenu

		case stepErrorDetails:
			f.ShowErrorDetails(
				"Error with Details",
				"An error occurred. Click Details to view the full error log.",
				sampleErrorLog,
				func() { log.Println("Copied error details to clipboard") },
				func() { log.Println("Saved error details to file") },
			)
			current = stepMenu

		case stepTextInput:
			resp := f.ShowTextInput(
				"Enter Text",
				"Enter your name:",
				"default value",
				webflow.WithSubtitle("Single text input dialog"),
				webflow.WithButtonBar(webflow.WizardMiddle()),
			)
			if !webflow.IsBack(resp) && !webflow.IsClose(resp) {
				if text, ok := resp.(string); ok {
					f.ShowMessage("Text Entered", fmt.Sprintf("You entered: %s", text), webflow.WithIcon("success"))
				}
			}
			current = stepMenu

		case stepFileSave:
			path, ok := f.ShowFileSavePicker(
				"Save File",
				"example.txt",
				webflow.FileFilter{Name: "Text Files", Patterns: []string{"*.txt"}},
				webflow.FileFilter{Name: "All Files", Patterns: []string{"*.*"}},
			)
			if ok && path != "" {
				f.ShowMessage("File Selected", fmt.Sprintf("You selected: %s", path), webflow.WithIcon("success"))
			} else {
				f.ShowMessage("Cancelled", "No file was selected.", webflow.WithIcon("info"))
			}
			current = stepMenu

		case stepReview:
			f.ShowReviewWithSave(
				"Configuration Review",
				sampleReviewText,
				func() { log.Println("Copied review content") },
				func() { log.Println("Saved review content") },
				webflow.WithSubtitle("Sample configuration file"),
			)
			current = stepMenu
		}
	}
}

// Sample content
const sampleLicenseText = `MIT License

Copyright (c) 2024 Component Gallery Demo

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.`

const sampleErrorLog = `Error Stack Trace
================
2024-01-15 14:32:17 ERROR [main.go:42] Failed to connect to database
2024-01-15 14:32:17 ERROR [db.go:156] Connection refused: localhost:5432
2024-01-15 14:32:17 DEBUG [retry.go:23] Retry attempt 1 of 3
2024-01-15 14:32:18 DEBUG [retry.go:23] Retry attempt 2 of 3
2024-01-15 14:32:19 DEBUG [retry.go:23] Retry attempt 3 of 3
2024-01-15 14:32:20 ERROR [retry.go:30] All retry attempts exhausted
2024-01-15 14:32:20 FATAL [main.go:45] Application terminated

Stack trace:
  at Database.Connect (db.go:156)
  at RetryHandler.Execute (retry.go:30)
  at main.Initialize (main.go:42)
  at main.main (main.go:12)`

const sampleReviewText = `Configuration Review
====================

Application: Component Gallery Demo
Version: 1.0.0
Build: 2024.01.15

Settings:
  - Theme: System Default
  - Language: English
  - Auto-update: Enabled
  - Telemetry: Disabled

Installed Components:
  - Core Framework (required)
  - UI Components (required)
  - Documentation (optional)
  - Example Projects (optional)

Installation Path: C:\Program Files\Demo

This configuration file can be copied or saved for your records.`
