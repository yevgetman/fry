package consciousness

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/templates"
)

// LoadCoreIdentity reads the core identity and disposition. It prefers the
// structured identity.json (produced by Reflection) and falls back to the
// hand-authored .md files if identity.json does not exist.
func LoadCoreIdentity() (string, error) {
	jsonID, err := LoadIdentityJSON()
	if err != nil {
		return "", fmt.Errorf("load identity: %w", err)
	}
	if jsonID != nil {
		return RenderIdentityForPrompt(jsonID), nil
	}

	return loadCoreIdentityFromMD()
}

// LoadDisposition reads only the disposition layer. Prefers identity.json,
// falls back to disposition.md.
func LoadDisposition() (string, error) {
	jsonID, err := LoadIdentityJSON()
	if err != nil {
		return "", fmt.Errorf("load disposition: %w", err)
	}
	if jsonID != nil {
		return RenderDispositionForPrompt(jsonID), nil
	}

	data, err := fs.ReadFile(templates.TemplateFS, config.IdentityDispositionFile)
	if err != nil {
		return "", fmt.Errorf("load disposition: %w", err)
	}
	return string(data), nil
}

// LoadFullIdentity reads all identity layers: core, disposition, and any
// domain files. Prefers identity.json (which includes domains), falls back
// to walking the identity/ directory for .md files.
func LoadFullIdentity() (string, error) {
	jsonID, err := LoadIdentityJSON()
	if err != nil {
		return "", fmt.Errorf("load identity: %w", err)
	}
	if jsonID != nil {
		return RenderIdentityForPrompt(jsonID), nil
	}

	return loadFullIdentityFromMD()
}

// loadCoreIdentityFromMD reads core.md + disposition.md and concatenates them.
func loadCoreIdentityFromMD() (string, error) {
	core, err := fs.ReadFile(templates.TemplateFS, config.IdentityCoreFile)
	if err != nil {
		return "", fmt.Errorf("load core identity: %w", err)
	}

	disp, err := fs.ReadFile(templates.TemplateFS, config.IdentityDispositionFile)
	if err != nil {
		return "", fmt.Errorf("load disposition: %w", err)
	}

	var b strings.Builder
	b.Write(core)
	if !strings.HasSuffix(string(core), "\n") {
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.Write(disp)
	return b.String(), nil
}

// loadFullIdentityFromMD reads all .md identity files including domains.
func loadFullIdentityFromMD() (string, error) {
	coreIdentity, err := loadCoreIdentityFromMD()
	if err != nil {
		return "", err
	}

	var domains []string
	domainDir := config.IdentityDomainsDir
	entries, readErr := fs.ReadDir(templates.TemplateFS, domainDir)
	if readErr == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			data, err := fs.ReadFile(templates.TemplateFS, domainDir+"/"+entry.Name())
			if err != nil {
				continue
			}
			domains = append(domains, string(data))
		}
	}

	if len(domains) == 0 {
		return coreIdentity, nil
	}

	var b strings.Builder
	b.WriteString(coreIdentity)
	for _, d := range domains {
		b.WriteByte('\n')
		b.WriteString(d)
		if !strings.HasSuffix(d, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String(), nil
}
