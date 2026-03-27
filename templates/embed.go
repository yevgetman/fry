package templates

import "embed"

// TemplateFS embeds the markdown reference files shipped with fry,
// including the identity layer files under identity/.
//
//go:embed *.md identity/*.md identity/identity.json
var TemplateFS embed.FS
