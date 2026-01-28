# webflow

Declarative wizard UI library for Go installers and setup assistants.

Built on [webframe](https://github.com/crafted-tech/webframe), which provides the underlying WebView2/WebKit window. webflow adds a higher-level API for multi-page wizard flows with built-in page types (messages, choices, forms, progress) and a shadcn-inspired UI with automatic dark/light mode support.

Communication between Go and the frontend uses `window.external.invoke()`. The HTML/CSS/JS is embedded in the binary - no external assets required at runtime other than custom resources like company logos.

## Packages

- **webflow** - Core wizard flow API (`ShowMessage`, `ShowChoice`, `ShowForm`, `ShowProgress`)
- **webflow/installer** - Step execution, logging, and common installer utilities
- **webflow/platform** - Platform-specific helpers (elevation, shortcuts, services, app registration)

## Shared Assets

The `installer/assets/` directory contains shared resources for downstream installers:

- `installer-icon.ico` - Standard installer icon (used by unison-test, unison-auditor)

Downstream projects locate these via Go module path resolution:

```python
# In build scripts
webflow_dir = get_go_module_dir("github.com/crafted-tech/webflow", project_dir)
installer_icon = webflow_dir / "installer" / "assets" / "installer-icon.ico"
```

## Usage

See `doc.go` in each package for API documentation and examples.

```go
import (
    "github.com/crafted-tech/webflow"
    "github.com/crafted-tech/webflow/installer"
    "github.com/crafted-tech/webflow/platform"
)
```

## Examples

- `examples/demo1/` - Basic wizard flow
- `examples/demo2/` - Component gallery
