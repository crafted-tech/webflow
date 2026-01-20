// Demo installer showcasing all webflow/flow features.
// This demonstrates a complete installation wizard with:
// - Welcome, License, Choice, MultiChoice pages
// - Text input, Form, Directory picker
// - Progress bar, File list progress, Log view
// - Review/text viewer with copy
// - Back button navigation
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/crafted-tech/webflow"
)

const licenseText = `MIT License

Copyright (c) 2026 Example Corp

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
SOFTWARE.
`

func main() {
	// Create a new flow wizard
	f, err := webflow.New(
		webflow.WithTitle("Demo Installer"),
		webflow.WithSize("45em", "35em"),
		webflow.WithResizable(false),
		//webflow.WithNativeTitleBar(true),
	)
	if err != nil {
		log.Fatalf("Failed to create flow: %v", err)
	}
	defer f.Close()

	// Installation state
	homeDir, _ := os.UserHomeDir()
	defaultPath := filepath.Join(homeDir, "DemoApp")
	installPath := defaultPath
	userName := ""
	installType := 0
	var selectedComponents []int
	var config map[string]any

	// State machine for back button navigation
	type installerStep int
	const (
		stepWelcome installerStep = iota
		stepLicense
		stepInstallType
		stepComponents
		stepUserInfo
		stepDirectory
		stepConfig
		stepReady
		stepInstall
	)

	step := stepWelcome

	for step < stepInstall {
		switch step {
		case stepWelcome:
			// Welcome page
			_, result := f.ShowMessage(
				"Welcome to Demo Application",
				"This wizard will guide you through the installation of Demo Application.\n\n"+
					"Demo Application is a sample installer that showcases all the features\n"+
					"of the webflow/flow package.\n\n"+
					"Click Next to continue.",
				webflow.WithButtonBar(webflow.WizardFirst()),
			)
			if result == webflow.ButtonResultClose {
				return
			}
			step = stepLicense

		case stepLicense:
			// License agreement
			_, result := f.ShowMessage(
				"License Agreement",
				licenseText,
				webflow.WithButtonBar(webflow.WizardLicense()),
			)
			switch result {
			case webflow.ButtonResultBack:
				step = stepWelcome
				continue
			case webflow.ButtonResultClose:
				return
			}
			step = stepInstallType

		case stepInstallType:
			// Installation type selection
			idx, _, result := f.ShowChoice(
				"Installation Type",
				[]string{
					"Full Installation (Recommended)",
					"Minimal Installation",
					"Custom Installation",
				},
				webflow.WithButtonBar(webflow.WizardMiddle()),
			)
			switch result {
			case webflow.ButtonResultBack:
				step = stepLicense
				continue
			case webflow.ButtonResultClose:
				return
			}
			installType = idx
			if installType == 2 {
				step = stepComponents
			} else {
				step = stepUserInfo
			}

		case stepComponents:
			// Custom component selection (multi-choice)
			components, _, result := f.ShowMultiChoice(
				"Select Components",
				[]string{
					"Core Application",
					"Documentation",
					"Example Projects",
					"Developer Tools",
					"Desktop Integration",
				},
				webflow.WithButtonBar(webflow.WizardMiddle()),
			)
			switch result {
			case webflow.ButtonResultBack:
				step = stepInstallType
				continue
			case webflow.ButtonResultClose:
				return
			}
			if len(components) == 0 {
				_, noCompResult := f.ShowMessage(
					"No Components Selected",
					"Please select at least one component to install.",
					webflow.WithButtonBar(webflow.ButtonBar{
						Back:  webflow.NewButton("Back", webflow.ButtonBack),
						Close: webflow.NewButton("Close", webflow.ButtonClose),
					}),
				)
				switch noCompResult {
				case webflow.ButtonResultBack:
					continue // Stay on component selection
				case webflow.ButtonResultClose:
					return
				}
			}
			selectedComponents = components
			step = stepUserInfo

		case stepUserInfo:
			// Get user name
			name, result := f.ShowTextInput(
				"User Information",
				"Enter your name (for personalization):",
				userName,
				webflow.WithButtonBar(webflow.WizardMiddle()),
			)
			switch result {
			case webflow.ButtonResultBack:
				if installType == 2 {
					step = stepComponents
				} else {
					step = stepInstallType
				}
				continue
			case webflow.ButtonResultClose:
				return
			}
			userName = name
			if userName == "" {
				userName = "User"
			}
			step = stepDirectory

		case stepDirectory:
			// Installation directory
			dirResult, _, result := f.ShowForm(
				"Choose Install Location",
				[]webflow.FormField{
					{
						ID:      "install_path",
						Type:    webflow.FieldPath,
						Label:   "Destination Folder:",
						Default: installPath,
					},
				},
				webflow.WithButtonBar(webflow.WizardMiddle()),
			)
			switch result {
			case webflow.ButtonResultBack:
				step = stepUserInfo
				continue
			case webflow.ButtonResultClose:
				return
			}
			if path, ok := dirResult["install_path"].(string); ok && path != "" {
				installPath = path
			}
			step = stepConfig

		case stepConfig:
			// Configuration form
			cfg, _, result := f.ShowForm(
				"Configuration",
				[]webflow.FormField{
					{
						ID:      "create_shortcut",
						Type:    webflow.FieldCheckbox,
						Label:   "Create desktop shortcut",
						Default: true,
					},
					{
						ID:      "start_menu",
						Type:    webflow.FieldCheckbox,
						Label:   "Add to start menu",
						Default: true,
					},
					{
						ID:      "auto_update",
						Type:    webflow.FieldSelect,
						Label:   "Automatic updates:",
						Options: []string{"Disabled", "Check only", "Auto-install"},
						Default: "Check only",
					},
				},
				webflow.WithButtonBar(webflow.WizardMiddle()),
			)
			switch result {
			case webflow.ButtonResultBack:
				step = stepDirectory
				continue
			case webflow.ButtonResultClose:
				return
			}
			config = cfg
			step = stepReady

		case stepReady:
			// Confirmation before installation
			installTypeNames := []string{"Full", "Minimal", "Custom"}
			summary := "Ready to install with the following settings:\n\n" +
				"Type: " + installTypeNames[installType] + "\n" +
				"Location: " + installPath + "\n" +
				"User: " + userName

			// Add component info for custom installation
			if installType == 2 && len(selectedComponents) > 0 {
				componentNames := []string{"Core Application", "Documentation", "Example Projects", "Developer Tools", "Desktop Integration"}
				summary += "\nComponents: "
				for i, idx := range selectedComponents {
					if i > 0 {
						summary += ", "
					}
					summary += componentNames[idx]
				}
			}

			_, result := f.ShowMessage(
				"Ready to Install",
				summary,
				webflow.WithButtonBar(webflow.WizardInstall()),
			)
			switch result {
			case webflow.ButtonResultBack:
				step = stepConfig
			case webflow.ButtonResultClose:
				f.ShowMessage("Installation Cancelled",
					"Installation was cancelled by user.",
					webflow.WithButtonBar(webflow.SimpleClose()))
				return
			case webflow.ButtonResultNext:
				step = stepInstall
			}
		}
	}

	// Show installation progress
	completed := f.ShowProgress("Installing Demo Application", func(p webflow.Progress) {
		steps := []struct {
			progress float64
			status   string
			duration time.Duration
		}{
			{5, "Creating directories...", 500 * time.Millisecond},
			{15, "Creating directories...", 300 * time.Millisecond},
			{30, "Copying files...", 800 * time.Millisecond},
			{50, "Copying files...", 1000 * time.Millisecond},
			{70, "Setting permissions...", 600 * time.Millisecond},
			{85, "Creating shortcuts...", 400 * time.Millisecond},
			{95, "Completing installation...", 300 * time.Millisecond},
			{100, "Installation complete!", 200 * time.Millisecond},
		}

		for _, s := range steps {
			if p.Cancelled() {
				return
			}
			p.Update(s.progress, s.status)
			time.Sleep(s.duration)
		}
	})

	if !completed {
		f.ShowMessage("Installation Cancelled",
			"The installation was cancelled.\n\nNo changes have been made to your system.",
			webflow.WithButtonBar(webflow.SimpleClose()))
		return
	}

	// Show file copy progress
	f.ShowFileProgress("Copying Files", func(files webflow.FileList) {
		demoFiles := []string{
			"bin/demoapp",
			"bin/demoapp-cli",
			"lib/libdemo.so",
			"share/icons/demo.png",
			"share/applications/demo.desktop",
			"doc/README.md",
			"doc/manual.pdf",
			"examples/hello.demo",
			"examples/advanced.demo",
		}

		for i, file := range demoFiles {
			if files.Cancelled() {
				return
			}

			files.SetProgress(i+1, len(demoFiles))
			files.AddFile(file, webflow.FileInProgress)
			files.SetCurrentFile(file)
			files.SetStatus("Copying " + file + "...")

			// Simulate file copy
			time.Sleep(300 * time.Millisecond)

			files.UpdateFile(file, webflow.FileComplete)
		}

		files.SetStatus("All files copied successfully!")
		time.Sleep(500 * time.Millisecond)
	})

	// Build installation log
	var logContent strings.Builder
	logContent.WriteString("Post-Installation Setup Log\n")
	logContent.WriteString("===========================\n\n")
	logContent.WriteString("Starting post-installation setup...\n")
	logContent.WriteString("Checking system requirements...\n")
	logContent.WriteString("  [OK] All requirements met\n")
	logContent.WriteString("Configuring application...\n")
	logContent.WriteString("  [OK] Configuration saved\n")

	if config != nil {
		if createShortcut, ok := config["create_shortcut"].(bool); ok && createShortcut {
			logContent.WriteString("Creating desktop shortcut...\n")
			logContent.WriteString("  [OK] Shortcut created\n")
		}
		if startMenu, ok := config["start_menu"].(bool); ok && startMenu {
			logContent.WriteString("Adding to start menu...\n")
			logContent.WriteString("  [OK] Start menu entry added\n")
		}
	}

	logContent.WriteString("Registering application...\n")
	logContent.WriteString("  [OK] Application registered\n")
	logContent.WriteString("\nSetup completed successfully!\n")
	logContent.WriteString(fmt.Sprintf("\nInstalled to: %s\n", installPath))

	logText := logContent.String()

	// Completion page with Details button for log viewing
	for {
		_, result := f.ShowMessage(
			"Installation Complete",
			fmt.Sprintf("Demo Application has been successfully installed!\n\n"+
				"Location: %s\n\n"+
				"Thank you for installing Demo Application, %s!\n\n"+
				"Click 'Details' to view the installation log.", installPath, userName),
			webflow.WithButtonBar(webflow.ButtonBar{
				Left: webflow.NewButton("Details", "left"),
				Next: webflow.NewButton("Finish", webflow.ButtonClose).WithPrimary(),
			}),
		)

		if result == webflow.ButtonResultLeft {
			// User clicked Details - show log review with save option
			f.ShowReviewWithSave("Installation Log", logText,
				func() {
					log.Println("Log copied to clipboard")
				},
				func() {
					// In a real app, this would show a save file dialog
					log.Println("Save to file requested")
				},
				webflow.WithSubtitle(installPath),
			)
			continue // Return to completion screen after viewing log
		}
		break // User clicked Finish - exit
	}

	log.Println("Installation wizard completed successfully")
}
