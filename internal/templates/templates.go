package templates

import "embed"

// FS contains the editable default templates shipped with aspgen.
// Users can export these files and override them with --templates.
//
// The `all:` prefix is required: several view templates (_Layout.cshtml,
// _ViewImports.cshtml, _ViewStart.cshtml) legitimately start with an
// underscore, which a plain `files` pattern would silently exclude from the
// binary.
//
//go:embed all:files
var FS embed.FS
