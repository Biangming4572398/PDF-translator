package compiler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrXeLaTeXNotFound = errors.New("xelatex executable not found")
	ErrOutputMissing   = errors.New("xelatex completed but did not create the expected PDF")
)

type Request struct {
	CompilerPath        string
	TeXRoot             string
	WorkingDir          string
	OutputDir           string
	TeXPath             string
	LogPath             string
	Passes              int
	Timeout             time.Duration
	AutoInstallPackages bool
}

type Result struct {
	BinaryPath    string
	Arguments     []string
	Passes        int
	Stdout        string
	Stderr        string
	CombinedLog   string
	OutputPDFPath string
	LogPath       string
	PDFExists     bool
	LogExcerpt    string
}

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Compile(ctx context.Context, request Request) (Result, error) {
	binaryPath, err := ResolveBinary(request.CompilerPath, request.TeXRoot)
	if err != nil {
		return Result{}, err
	}

	passes := request.Passes
	if passes <= 0 {
		passes = 2
	}

	timeout := request.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	var stdoutAll strings.Builder
	var stderrAll strings.Builder
	var combined strings.Builder

	outputDir := request.OutputDir
	if outputDir == "" {
		outputDir = request.WorkingDir
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return Result{}, err
	}

	outputPDFPath := filepath.Join(
		outputDir,
		strings.TrimSuffix(filepath.Base(request.TeXPath), filepath.Ext(request.TeXPath))+".pdf",
	)
	arguments := buildArguments(request, binaryPath)

	combined.WriteString("===== XeLaTeX configuration =====\n")
	combined.WriteString(fmt.Sprintf("binary: %s\n", binaryPath))
	combined.WriteString(fmt.Sprintf("working_dir: %s\n", request.WorkingDir))
	combined.WriteString(fmt.Sprintf("tex_file: %s\n", request.TeXPath))
	combined.WriteString(fmt.Sprintf("expected_pdf: %s\n", outputPDFPath))
	combined.WriteString(fmt.Sprintf("passes: %d\n", passes))
	combined.WriteString(fmt.Sprintf("arguments: %s\n\n", strings.Join(arguments, " ")))

	for pass := 1; pass <= passes; pass++ {
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		cmd := exec.CommandContext(runCtx, binaryPath, arguments...)
		cmd.Dir = request.WorkingDir
		cmd.Env = buildEnvironment(os.Environ(), binaryPath)

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		cancel()

		stdoutAll.WriteString(stdout.String())
		stderrAll.WriteString(stderr.String())
		combined.WriteString(fmt.Sprintf("===== XeLaTeX pass %d =====\n", pass))
		combined.WriteString(stdout.String())
		if stderr.Len() > 0 {
			combined.WriteString("\n----- STDERR -----\n")
			combined.WriteString(stderr.String())
		}
		combined.WriteString("\n")

		if err != nil {
			result := Result{
				BinaryPath:    binaryPath,
				Arguments:     arguments,
				Passes:        pass,
				Stdout:        stdoutAll.String(),
				Stderr:        stderrAll.String(),
				CombinedLog:   combined.String(),
				OutputPDFPath: outputPDFPath,
				LogPath:       request.LogPath,
				PDFExists:     fileExists(outputPDFPath),
			}
			result.LogExcerpt = extractUsefulLogExcerpt(result.CombinedLog)

			if request.LogPath != "" {
				_ = os.WriteFile(request.LogPath, []byte(result.CombinedLog), 0o644)
			}

			return result, err
		}
	}

	result := Result{
		BinaryPath:    binaryPath,
		Arguments:     arguments,
		Passes:        passes,
		Stdout:        stdoutAll.String(),
		Stderr:        stderrAll.String(),
		CombinedLog:   combined.String(),
		OutputPDFPath: outputPDFPath,
		LogPath:       request.LogPath,
		PDFExists:     fileExists(outputPDFPath),
	}
	result.LogExcerpt = extractUsefulLogExcerpt(result.CombinedLog)

	if request.LogPath != "" {
		_ = os.WriteFile(request.LogPath, []byte(result.CombinedLog), 0o644)
	}

	if !result.PDFExists {
		return result, ErrOutputMissing
	}

	return result, nil
}

func buildArguments(request Request, binaryPath string) []string {
	args := []string{
		"-interaction=nonstopmode",
		"-halt-on-error",
		"-file-line-error",
		fmt.Sprintf("-output-directory=%s", firstNonEmpty(request.OutputDir, request.WorkingDir)),
	}

	if isMiKTeXPath(binaryPath) {
		args = append(args, "--disable-write18")
		if request.AutoInstallPackages {
			args = append(args, "--enable-installer")
		} else {
			args = append(args, "--disable-installer")
		}
	}

	args = append(args, request.TeXPath)
	return args
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func buildEnvironment(current []string, binaryPath string) []string {
	binDir := filepath.Dir(binaryPath)
	if binDir == "." || binDir == "" {
		return current
	}

	output := make([]string, 0, len(current)+1)
	pathSet := false
	for _, entry := range current {
		if strings.HasPrefix(strings.ToUpper(entry), "PATH=") {
			pathValue := ""
			if index := strings.Index(entry, "="); index >= 0 && index+1 < len(entry) {
				pathValue = entry[index+1:]
			}
			output = append(output, "PATH="+binDir+string(os.PathListSeparator)+pathValue)
			pathSet = true
			continue
		}

		output = append(output, entry)
	}
	if !pathSet {
		output = append(output, "PATH="+binDir)
	}

	return output
}

func ResolveBinary(configuredPath, texRoot string) (string, error) {
	for _, candidate := range configuredCandidates(configuredPath, texRoot) {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	if path, err := exec.LookPath("xelatex"); err == nil {
		return path, nil
	}

	if path, err := exec.LookPath("xelatex.exe"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("%w: choose xelatex.exe or the portable MiKTeX folder in settings", ErrXeLaTeXNotFound)
}

type Installation struct {
	BinaryPath string
	TeXRoot    string
	Source     string
}

func DiscoverInstallations(extraRoot string) []Installation {
	seen := map[string]bool{}
	var installations []Installation

	add := func(binaryPath, root, source string) {
		if binaryPath == "" || seen[strings.ToLower(binaryPath)] {
			return
		}
		if info, err := os.Stat(binaryPath); err != nil || info.IsDir() {
			return
		}

		seen[strings.ToLower(binaryPath)] = true
		installations = append(installations, Installation{
			BinaryPath: binaryPath,
			TeXRoot:    root,
			Source:     source,
		})
	}

	for _, candidate := range configuredCandidates("", extraRoot) {
		add(candidate, extraRoot, "configured TeX folder")
	}

	if path, err := exec.LookPath("xelatex"); err == nil {
		add(path, filepath.Dir(filepath.Dir(filepath.Dir(path))), "PATH")
	}
	if path, err := exec.LookPath("xelatex.exe"); err == nil {
		add(path, filepath.Dir(filepath.Dir(filepath.Dir(path))), "PATH")
	}

	for _, root := range commonRoots() {
		for _, candidate := range configuredCandidates("", root) {
			add(candidate, root, "common MiKTeX location")
		}
		for _, found := range findNamedExecutable(root, "xelatex.exe", 5, 8) {
			add(found, inferRoot(found), "nearby search")
		}
	}

	return installations
}

func configuredCandidates(configuredPath, texRoot string) []string {
	var candidates []string
	if configuredPath != "" {
		candidates = append(candidates, configuredPath)
	}
	if texRoot == "" {
		return candidates
	}

	candidates = append(candidates,
		filepath.Join(texRoot, "xelatex.exe"),
		filepath.Join(texRoot, "miktex", "bin", "x64", "xelatex.exe"),
		filepath.Join(texRoot, "miktex", "bin", "xelatex.exe"),
		filepath.Join(texRoot, "bin", "x64", "xelatex.exe"),
		filepath.Join(texRoot, "bin", "xelatex.exe"),
		filepath.Join(texRoot, "texmfs", "install", "miktex", "bin", "x64", "xelatex.exe"),
		filepath.Join(texRoot, "texmfs", "install", "miktex", "bin", "xelatex.exe"),
	)
	return candidates
}

func commonRoots() []string {
	var roots []string
	if executable, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(executable)
		roots = append(roots, exeDir, filepath.Dir(exeDir))
	}
	if workingDir, err := os.Getwd(); err == nil {
		roots = append(roots, workingDir)
	}

	keys := []string{
		"LOCALAPPDATA",
		"ProgramFiles",
		"ProgramFiles(x86)",
		"USERPROFILE",
	}

	for _, key := range keys {
		value := os.Getenv(key)
		if value == "" {
			continue
		}

		switch key {
		case "LOCALAPPDATA":
			roots = append(roots, filepath.Join(value, "Programs", "MiKTeX"))
		case "ProgramFiles", "ProgramFiles(x86)":
			roots = append(roots, filepath.Join(value, "MiKTeX"))
		case "USERPROFILE":
			roots = append(
				roots,
				filepath.Join(value, "Downloads"),
				filepath.Join(value, "Desktop"),
				filepath.Join(value, "Documents"),
			)
		}
	}

	return roots
}

func findNamedExecutable(root, name string, maxDepth, maxResults int) []string {
	if root == "" || maxDepth < 0 || maxResults <= 0 {
		return nil
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return nil
	}

	var found []string
	var walk func(string, int)
	walk = func(dir string, depth int) {
		if len(found) >= maxResults || depth > maxDepth {
			return
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		for _, entry := range entries {
			if len(found) >= maxResults {
				return
			}

			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				walk(path, depth+1)
				continue
			}

			if strings.EqualFold(entry.Name(), name) {
				found = append(found, path)
			}
		}
	}

	walk(root, 0)
	return found
}

func inferRoot(binaryPath string) string {
	dir := filepath.Dir(binaryPath)
	for {
		if strings.EqualFold(filepath.Base(dir), "miktex") {
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Dir(binaryPath)
		}
		dir = parent
	}
}

func isMiKTeXPath(binaryPath string) bool {
	return strings.Contains(strings.ToLower(binaryPath), "miktex")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func extractUsefulLogExcerpt(log string) string {
	lines := strings.Split(log, "\n")
	interesting := make([]string, 0, 20)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lowered := strings.ToLower(trimmed)
		if strings.HasPrefix(trimmed, "!") ||
			strings.Contains(lowered, "error") ||
			strings.Contains(lowered, "fatal") ||
			strings.Contains(lowered, "not found") ||
			strings.Contains(lowered, "missing") ||
			strings.Contains(lowered, "emergency stop") {
			interesting = append(interesting, trimmed)
		}
	}

	if len(interesting) == 0 {
		if len(lines) > 20 {
			lines = lines[len(lines)-20:]
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}

	if len(interesting) > 20 {
		interesting = interesting[:20]
	}
	return strings.TrimSpace(strings.Join(interesting, "\n"))
}
