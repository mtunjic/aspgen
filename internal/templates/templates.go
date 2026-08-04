package templates

import "embed"

// FS contains the editable default templates shipped with aspgen.
// Users can export these files and override them with --templates.
//
//go:embed files
var FS embed.FS
