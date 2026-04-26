package prompts

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
)

//go:embed system.md user.md
var defaults embed.FS

type Templates struct {
	System string
	User   string
}

func Load() Templates {
	return Templates{
		System: loadTemplate("system.md"),
		User:   loadTemplate("user.md"),
	}
}

func Render(template string, values map[string]string) string {
	rendered := template
	for key, value := range values {
		rendered = strings.ReplaceAll(rendered, "{{"+key+"}}", value)
	}

	return strings.TrimSpace(rendered)
}

func loadTemplate(name string) string {
	for _, dir := range candidateDirs() {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil && strings.TrimSpace(string(data)) != "" {
			return string(data)
		}
	}

	data, err := defaults.ReadFile(name)
	if err != nil {
		return ""
	}

	return string(data)
}

func candidateDirs() []string {
	dirs := make([]string, 0, 3)
	if configured := strings.TrimSpace(os.Getenv("PDF_TRANSLATOR_PROMPTS_DIR")); configured != "" {
		dirs = append(dirs, configured)
	}
	if executable, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(executable), "prompts"))
	}
	if workingDir, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(workingDir, "prompts"))
	}

	return dirs
}
