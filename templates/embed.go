package templates

import "embed"

// TemplateFS embeds the markdown reference files shipped with fry,
// including the identity layer files under identity/.
//
//go:embed *.md identity/*.md
var TemplateFS embed.FS
