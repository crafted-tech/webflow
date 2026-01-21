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
			resp := f.ShowWelcome(webflow.WelcomeConfig{
				Logo:             logoSVG,
				LogoHeight:       64,
				Title:            TF("welcome.title", appName),
				Message:          TF("welcome.message", appName),
				LanguageSelector: true,
			})
			if webflow.IsClose(resp) {
				return
			}
			if webflow.LanguageChanged(resp) {
				continue
			}
			step = stepLicense

		case stepLicense:
			// License agreement - use ShowMessage for full back navigation support
			resp := f.ShowMessage(
				T("license.title"),
				licenseText,
				webflow.WithSubtitle(T("license.label")),
				webflow.WithButtonBar(webflow.WizardLicense()),
				webflow.WithBorderedContent(),
			)
			if webflow.IsBack(resp) {
				step = stepWelcome
				continue
			}
			if webflow.IsClose(resp) {
				return
			}
			step = stepInstallType

		case stepInstallType:
			// Installation type selection with translated options
			resp := f.ShowChoice(
				T("installType.title"),
				[]string{
					T("installType.full"),
					T("installType.minimal"),
					T("installType.custom"),
				},
				webflow.WithButtonBar(webflow.WizardMiddle()),
			)
			if webflow.IsBack(resp) {
				step = stepLicense
				continue
			}
			if webflow.IsClose(resp) {
				return
			}
			installType = resp.(int)
			if installType == 2 {
				step = stepComponents
			} else {
				step = stepUserInfo
			}

		case stepComponents:
			// Custom component selection with translated labels
			resp := f.ShowMultiChoice(
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
			if webflow.IsBack(resp) {
				step = stepInstallType
				continue
			}
			if webflow.IsClose(resp) {
				return
			}
			components := resp.([]int)
			if len(components) == 0 {
				noCompResp := f.ShowMessage(
					T("noComponents.title"),
					T("noComponents.message"),
					webflow.WithButtonBar(webflow.ButtonBar{
						Back:  webflow.NewButton(T("button.back"), webflow.ButtonBack),
						Close: webflow.NewButton(T("button.close"), webflow.ButtonClose),
					}),
				)
				if webflow.IsBack(noCompResp) {
					continue // Stay on component selection
				}
				if webflow.IsClose(noCompResp) {
					return
				}
			}
			selectedComponents = components
			step = stepUserInfo

		case stepUserInfo:
			// Get user name
			resp := f.ShowTextInput(
				T("userInfo.title"),
				T("userInfo.prompt"),
				userName,
				webflow.WithButtonBar(webflow.WizardMiddle()),
			)
			if webflow.IsBack(resp) {
				if installType == 2 {
					step = stepComponents
				} else {
					step = stepInstallType
				}
				continue
			}
			if webflow.IsClose(resp) {
				return
			}
			userName = resp.(string)
			if userName == "" {
				userName = "User"
			}
			step = stepDirectory

		case stepDirectory:
			// Installation directory
			resp := f.ShowForm(
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
			if webflow.IsBack(resp) {
				step = stepUserInfo
				continue
			}
			if webflow.IsClose(resp) {
				return
			}
			formData := resp.(map[string]any)
			if path, ok := formData["install_path"].(string); ok && path != "" {
				installPath = path
			}
			step = stepConfig

		case stepConfig:
			// Configuration form with translated labels
			resp := f.ShowForm(
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
			if webflow.IsBack(resp) {
				step = stepDirectory
				continue
			}
			if webflow.IsClose(resp) {
				return
			}
			config = resp.(map[string]any)
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

			resp := f.ShowMessage(
				T("ready.title"),
				webflow.SummaryConfig{Items: summaryItems},
				webflow.WithButtonBar(webflow.WizardInstall()),
			)
			if webflow.IsBack(resp) {
				step = stepConfig
			} else if webflow.IsClose(resp) {
				f.ShowMessage(T("cancelled.title"),
					T("cancelled.message"),
					webflow.WithButtonBar(webflow.SimpleClose()))
				return
			} else {
				step = stepConfirm
			}

		case stepConfirm:
			// Confirmation with checkbox using ShowConfirmWithCheckbox
			resp := f.ShowConfirmWithCheckbox(webflow.ConfirmCheckboxConfig{
				Title:          T("confirm.title"),
				Message:        TF("confirm.message", appName),
				CheckboxLabel:  T("confirm.checkbox"),
				WarningMessage: T("confirm.warning"),
			})
			if resp != true {
				step = stepReady
				continue
			}
			step = stepInstall
		}
	}

	// Show installation progress with translated status messages
	// Use library's installing.* keys
	progressResp := f.ShowProgress(T("installing.title"), func(p webflow.Progress) {
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

	if webflow.IsClose(progressResp) {
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
		resp := f.ShowMessage(
			T("complete.title"),
			TF("complete.message", appName),
			webflow.WithButtonBar(webflow.ButtonBar{
				Left: webflow.NewButton(T("button.details"), "details"),
				Next: webflow.NewButton(T("button.finish"), webflow.ButtonClose).WithPrimary(),
			}),
		)

		if webflow.IsButton(resp, "details") {
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
