package consciousness

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/templates"
)

// LoadCoreIdentity reads the core identity and disposition from embedded
// template files and returns them concatenated. This is the identity content
// loaded into every build context.
func LoadCoreIdentity() (string, error) {
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

// LoadDisposition reads only the disposition layer from embedded templates.
// This is injected into sprint agent prompts to subtly influence behavior.
func LoadDisposition() (string, error) {
	data, err := fs.ReadFile(templates.TemplateFS, config.IdentityDispositionFile)
	if err != nil {
		return "", fmt.Errorf("load disposition: %w", err)
	}
	return string(data), nil
}

// LoadFullIdentity reads all identity layers: core, disposition, and any
// domain files. Domain files are discovered by walking the identity/domains/
// directory in the embedded filesystem.
func LoadFullIdentity() (string, error) {
	coreIdentity, err := LoadCoreIdentity()
	if err != nil {
		return "", err
	}

	// Attempt to read domain files; if the directory doesn't exist, that's fine.
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
