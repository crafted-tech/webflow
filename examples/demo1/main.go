// Demo installer showcasing all webflow/flow features.
// This demonstrates a complete installation wizard with:
// - Welcome page with logo and language selector (i18n)
// - License agreement with LicenseConfig
// - Choice, MultiChoice pages
// - Text input, Form, Directory picker
// - Confirmation with checkbox
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

// T is a shorthand for webflow.T
func T(key string) string {
	return webflow.T(key)
}

// TF is a shorthand for webflow.TF
func TF(key string, args ...any) string {
	return webflow.TF(key, args...)
}

// App name constant - used in translations with {0} placeholder
const appName = "Demo Application"

func main() {
	// Create a new flow wizard with app translations for i18n support
	f, err := webflow.New(
		webflow.WithTitle("Demo Installer"),
		webflow.WithSize("45em", "35em"),
		webflow.WithResizable(false),
		webflow.WithAppTranslations(loadTranslations()),
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
		stepConfirm
		stepInstall
	)

	step := stepWelcome

	for step < stepInstall {
		switch step {
		case stepWelcome:
			// Welcome page with logo and language selector
			// Use library's welcome.title/message with app name substitution
			result := f.ShowWelcome(webflow.WelcomeConfig{
				Logo:             logoSVG,
				LogoHeight:       64,
				Title:            TF("welcome.title", appName),
				Message:          TF("welcome.message", appName),
				LanguageSelector: true,
			})
			if result == webflow.ButtonResultClose {
				return
			}
			step = stepLicense

		case stepLicense:
			// License agreement - use ShowMessage for full back navigation support
			// (ShowLicense returns bool, which can't distinguish Back from Close)
			_, result := f.ShowMessage(
				T("license.title"),
				licenseText,
				webflow.WithSubtitle(T("license.label")),
				webflow.WithButtonBar(webflow.WizardLicense()),
				webflow.WithBorderedContent(),
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
			// Installation type selection with translated options
			idx, _, result := f.ShowChoice(
				T("installType.title"),
				[]string{
					T("installType.full"),
					T("installType.minimal"),
					T("installType.custom"),
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
			// Custom component selection with translated labels
			components, _, result := f.ShowMultiChoice(
				T("components.title"),
				[]string{
					T("components.core"),
					T("components.docs"),
					T("components.examples"),
					T("components.devtools"),
					T("components.desktop"),
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
					T("noComponents.title"),
					T("noComponents.message"),
					webflow.WithButtonBar(webflow.ButtonBar{
						Back:  webflow.NewButton(T("button.back"), webflow.ButtonBack),
						Close: webflow.NewButton(T("button.close"), webflow.ButtonClose),
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
				T("userInfo.title"),
				T("userInfo.prompt"),
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
				T("directory.title"),
				[]webflow.FormField{
					{
						ID:      "install_path",
						Type:    webflow.FieldPath,
						Label:   T("directory.label"),
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
			// Configuration form with translated labels
			cfg, _, result := f.ShowForm(
				T("config.title"),
				[]webflow.FormField{
					{
						ID:      "create_shortcut",
						Type:    webflow.FieldCheckbox,
						Label:   T("config.shortcut"),
						Default: true,
					},
					{
						ID:      "start_menu",
						Type:    webflow.FieldCheckbox,
						Label:   T("config.startMenu"),
						Default: true,
					},
					{
						ID:      "auto_update",
						Type:    webflow.FieldSelect,
						Label:   T("config.autoUpdate"),
						Options: []string{
							T("config.updateDisabled"),
							T("config.updateCheck"),
							T("config.updateAuto"),
						},
						Default: T("config.updateCheck"),
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
			// Confirmation summary before installation using SummaryConfig
			installTypeNames := []string{
				T("installType.full"),
				T("installType.minimal"),
				T("installType.custom"),
			}

			// Build summary items
			summaryItems := []webflow.SummaryItem{
				{Label: T("ready.type"), Value: installTypeNames[installType]},
				{Label: T("ready.location"), Value: installPath},
				{Label: T("ready.user"), Value: userName},
			}

			// Add component info for custom installation
			if installType == 2 && len(selectedComponents) > 0 {
				componentNames := []string{
					T("components.core"),
					T("components.docs"),
					T("components.examples"),
					T("components.devtools"),
					T("components.desktop"),
				}
				var componentList strings.Builder
				for i, idx := range selectedComponents {
					if i > 0 {
						componentList.WriteString(", ")
					}
					componentList.WriteString(componentNames[idx])
				}
				summaryItems = append(summaryItems, webflow.SummaryItem{
					Label: T("ready.components"),
					Value: componentList.String(),
				})
			}

			_, result := f.ShowMessage(
				T("ready.title"),
				webflow.SummaryConfig{Items: summaryItems},
				webflow.WithButtonBar(webflow.WizardInstall()),
			)
			switch result {
			case webflow.ButtonResultBack:
				step = stepConfig
			case webflow.ButtonResultClose:
				f.ShowMessage(T("cancelled.title"),
					T("cancelled.message"),
					webflow.WithButtonBar(webflow.SimpleClose()))
				return
			case webflow.ButtonResultNext:
				step = stepConfirm
			}

		case stepConfirm:
			// Confirmation with checkbox using ShowConfirmWithCheckbox
			ok := f.ShowConfirmWithCheckbox(webflow.ConfirmCheckboxConfig{
				Title:          T("confirm.title"),
				Message:        TF("confirm.message", appName),
				CheckboxLabel:  T("confirm.checkbox"),
				WarningMessage: T("confirm.warning"),
			})
			if !ok {
				step = stepReady
				continue
			}
			step = stepInstall
		}
	}

	// Show installation progress with translated status messages
	// Use library's installing.* keys
	completed := f.ShowProgress(T("installing.title"), func(p webflow.Progress) {
		steps := []struct {
			progress float64
			status   string
			duration time.Duration
		}{
			{5, T("installing.creatingDirs"), 500 * time.Millisecond},
			{15, T("installing.creatingDirs"), 300 * time.Millisecond},
			{30, T("installing.copyingFiles"), 800 * time.Millisecond},
			{50, T("installing.copyingFiles"), 1000 * time.Millisecond},
			{70, T("installing.permissions"), 600 * time.Millisecond},
			{85, T("installing.creatingShortcuts"), 400 * time.Millisecond},
			{95, T("installing.completing"), 300 * time.Millisecond},
			{100, T("installing.completing"), 200 * time.Millisecond},
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
		f.ShowMessage(T("cancelled.title"),
			T("cancelled.message"),
			webflow.WithButtonBar(webflow.SimpleClose()))
		return
	}

	// Show file copy progress
	f.ShowFileProgress(T("copyingFiles.title"), func(files webflow.FileList) {
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
			files.SetStatus(TF("copyingFiles.status", file))

			// Simulate file copy
			time.Sleep(300 * time.Millisecond)

			files.UpdateFile(file, webflow.FileComplete)
		}

		files.SetStatus(T("copyingFiles.complete"))
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
	// Use library's complete.title/message with app name substitution
	for {
		_, result := f.ShowMessage(
			T("complete.title"),
			TF("complete.message", appName),
			webflow.WithButtonBar(webflow.ButtonBar{
				Left: webflow.NewButton(T("button.details"), "left"),
				Next: webflow.NewButton(T("button.finish"), webflow.ButtonClose).WithPrimary(),
			}),
		)

		if result == webflow.ButtonResultLeft {
			// User clicked Details - show log review with save option
			f.ShowReviewWithSave(T("log.title"), logText,
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
