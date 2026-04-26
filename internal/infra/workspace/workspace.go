package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Manager struct{}

type RunWorkspace struct {
	RunID                string
	WorkingDir           string
	TempTeXPath          string
	TempPDFPath          string
	TempCompilerLogPath  string
	TranslationDebugPath string
	SavedTeXPath         string
	SavedPDFPath         string
	SavedCompilerLogPath string
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Prepare(inputPath, outputDir, targetLanguage string) (RunWorkspace, error) {
	baseName := sanitizeName(strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath)))
	if baseName == "" {
		baseName = "document"
	}

	runID := time.Now().Format("20060102-150405")
	buildRoot := filepath.Join(outputDir, "_pdftranslator_runs", fmt.Sprintf("%s-%s", baseName, runID))
	if err := os.MkdirAll(buildRoot, 0o755); err != nil {
		return RunWorkspace{}, err
	}

	targetToken := sanitizeName(strings.ToLower(targetLanguage))
	if targetToken == "" {
		targetToken = "translated"
	}

	return RunWorkspace{
		RunID:                runID,
		WorkingDir:           buildRoot,
		TempTeXPath:          filepath.Join(buildRoot, "translated.tex"),
		TempPDFPath:          filepath.Join(buildRoot, "translated.pdf"),
		TempCompilerLogPath:  filepath.Join(buildRoot, "xelatex.log"),
		TranslationDebugPath: filepath.Join(buildRoot, "translation-debug.json"),
		SavedTeXPath:         filepath.Join(outputDir, fmt.Sprintf("%s.%s.tex", baseName, targetToken)),
		SavedPDFPath:         filepath.Join(outputDir, fmt.Sprintf("%s.%s.pdf", baseName, targetToken)),
		SavedCompilerLogPath: filepath.Join(outputDir, fmt.Sprintf("%s.%s.xelatex.log", baseName, targetToken)),
	}, nil
}

func CopyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}

	return output.Close()
}

func sanitizeName(value string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	value = re.ReplaceAllString(strings.TrimSpace(value), "-")
	return strings.Trim(value, "-.")
}
