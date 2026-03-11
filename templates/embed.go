package templates

import "embed"

// TemplateFS embeds the markdown reference files shipped with fry.
//
//go:embed *.md
var TemplateFS embed.FS
