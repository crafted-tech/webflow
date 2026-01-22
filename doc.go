/*
Package webflow provides a declarative API for building wizard-like applications
using HTML rendering. It is designed for installers, setup assistants,
configuration tools, and onboarding flows.

# Basic Usage

Create a new Flow and display pages using the Show* methods:

	f, err := webflow.New(
		webflow.WithTitle("My App Setup"),
		webflow.WithSize(600, 450),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Welcome message
	f.ShowMessage("Welcome", "Welcome to the setup wizard!",
		webflow.Button{Label: "Next", ID: "next", Primary: true},
	)

	// Choice selection
	selected, _ := f.ShowChoice("Select Type",
		[]webflow.Choice{
			{Label: "Full Installation"},
			{Label: "Minimal"},
			{Label: "Custom"},
		},
		webflow.WithButtonBar(webflow.WizardMiddle()),
	)

	// Form input
	values, _ := f.ShowForm("Configuration", []webflow.FormField{
		{ID: "path", Type: webflow.FieldPath, Label: "Install Location:", Default: "/opt/app"},
		{ID: "shortcut", Type: webflow.FieldCheckbox, Label: "Create desktop shortcut", Default: true},
	})

	// Progress
	f.ShowProgress("Installing", func(p webflow.Progress) {
		for i := 0; i <= 100; i++ {
			if p.Cancelled() {
				return
			}
			p.Update(float64(i), fmt.Sprintf("Processing... %d%%", i))
			time.Sleep(50 * time.Millisecond)
		}
	})

	// Done
	f.ShowMessage("Complete", "Installation complete!", webflow.FinishButton)

# Page Types

The package provides several methods for different page types:

  - ShowMessage: Display text with configurable buttons
  - ShowChoice: Display single-selection with Choice structs (labels + optional descriptions)
  - ShowMultiChoice: Display multi-selection with Choice structs (labels + optional descriptions)
  - ShowForm: Display a form with various input types
  - ShowProgress: Display a progress bar with cancellation support
  - ShowPage: Display a fully custom page (advanced use)

# Form Fields

Forms support the following field types:

  - FieldText: Single-line text input
  - FieldPassword: Password input (masked)
  - FieldCheckbox: Boolean checkbox
  - FieldSelect: Dropdown selection
  - FieldPath: File/directory path with browse button
  - FieldTextArea: Multi-line text input

# Styling

The UI uses a modern, shadcn-inspired design with automatic dark/light mode
detection. Custom styling can be achieved by modifying the embedded CSS.

# JS->Go Communication

Internally, the package uses window.external.invoke() for JavaScript to Go
communication. Button clicks and form submissions are automatically handled.
*/
package webflow
