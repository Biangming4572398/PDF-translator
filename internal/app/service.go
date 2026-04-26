package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pdftranslator/internal/compiler"
	"pdftranslator/internal/config"
	"pdftranslator/internal/infra/logging"
	"pdftranslator/internal/infra/workspace"
	"pdftranslator/internal/latex"
	"pdftranslator/internal/pdf"
	"pdftranslator/internal/translate"
)

type Service struct {
	store      *config.Store
	logger     *logging.Logger
	workspaces *workspace.Manager
	translator *translate.Service
	validator  *latex.Validator
	renderer   *latex.Renderer
	compiler   *compiler.Engine

	mu         sync.RWMutex
	lastResult *RunResult
}

func NewService(
	store *config.Store,
	logger *logging.Logger,
	workspaces *workspace.Manager,
	translator *translate.Service,
	validator *latex.Validator,
	renderer *latex.Renderer,
	compiler *compiler.Engine,
) *Service {
	return &Service{
		store:      store,
		logger:     logger,
		workspaces: workspaces,
		translator: translator,
		validator:  validator,
		renderer:   renderer,
		compiler:   compiler,
	}
}

func (s *Service) CurrentSettings() config.Settings {
	return s.store.Current()
}

func (s *Service) SaveSettings(settings config.Settings) error {
	s.logger.Infof("saving settings for provider %s", settings.CurrentProvider)
	return s.store.Save(settings)
}

func (s *Service) ProviderDescriptors() []translate.ProviderDescriptor {
	return s.translator.Descriptors()
}

func (s *Service) FetchModels(ctx context.Context, providerName string, providerConfig config.ProviderConfig) ([]translate.ModelDescriptor, error) {
	return s.translator.FetchModels(ctx, providerName, providerConfig)
}

func (s *Service) InspectInputPDF(path string) (InputDocument, error) {
	settings := s.store.Current()
	loader := pdf.NewLoader(settings.PDF.MaxFileSizeMB)
	doc, err := loader.Load(path)
	if err != nil {
		return InputDocument{}, err
	}

	return InputDocument{
		Path:      doc.Path,
		FileName:  doc.Name,
		SizeBytes: doc.SizeBytes,
	}, nil
}

func (s *Service) LastResult() *RunResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.lastResult == nil {
		return nil
	}

	copyValue := *s.lastResult
	return &copyValue
}

func (s *Service) RunTranslation(ctx context.Context, request RunRequest, emit func(ProgressEvent)) (RunResult, error) {
	settings := s.store.Current()
	sourceLanguage := firstNonEmpty(request.SourceLanguage, settings.DefaultSourceLanguage)
	targetLanguage := firstNonEmpty(request.TargetLanguage, settings.DefaultTargetLanguage)
	providerName := firstNonEmpty(request.ProviderName, settings.CurrentProvider)

	loader := pdf.NewLoader(settings.PDF.MaxFileSizeMB)

	doc, err := loader.Load(request.InputPDFPath)
	if err != nil {
		s.logger.Errorf("failed to load PDF: %v", err)
		s.emit(emit, StageFailed, err.Error())
		return RunResult{}, err
	}
	s.emit(emit, StagePDFLoaded, fmt.Sprintf("Loaded %s", doc.Name))

	s.emit(emit, StageRequestPrepared, "Raw PDF loaded; provider will attach the file at the HTTP boundary")

	ws, err := s.workspaces.Prepare(doc.Path, request.OutputDirectory, targetLanguage)
	if err != nil {
		s.logger.Errorf("failed to prepare workspace: %v", err)
		s.emit(emit, StageFailed, err.Error())
		return RunResult{}, err
	}

	s.emit(emit, StageTranslationRunning, fmt.Sprintf("Sending raw PDF request to %s", providerName))
	translationResult, err := s.translator.Translate(
		ctx,
		providerName,
		sourceLanguage,
		targetLanguage,
		settings.Provider(providerName),
		doc,
	)
	if err != nil {
		s.logger.Errorf("translation provider failed: %v", err)
		_ = os.WriteFile(ws.TranslationDebugPath, []byte(err.Error()), 0o644)
		s.emit(emit, StageFailed, err.Error())
		return RunResult{}, err
	}

	if err := writeTranslationDebug(ws.TranslationDebugPath, translationResult); err != nil {
		s.logger.Warnf("failed to save translation debug file: %v", err)
	}

	sanitized, err := s.validator.Sanitize(translationResult.Response.LatexCandidate)
	if err != nil {
		s.logger.Errorf("latex validation failed: %v", err)
		s.emit(emit, StageFailed, err.Error())
		return RunResult{}, err
	}
	s.emit(emit, StageLatexValidation, "Validated and sanitized LaTeX output")

	finalTeX := s.renderer.Render(sanitized, latex.Options{
		Title:          buildTitle(doc.Name, targetLanguage),
		MainFont:       settings.Compiler.MainFont,
		SourceLanguage: sourceLanguage,
		TargetLanguage: targetLanguage,
	})

	if err := os.WriteFile(ws.TempTeXPath, []byte(finalTeX), 0o644); err != nil {
		s.logger.Errorf("failed to save temp tex file: %v", err)
		s.emit(emit, StageFailed, err.Error())
		return RunResult{}, err
	}
	if err := os.WriteFile(ws.SavedTeXPath, []byte(finalTeX), 0o644); err != nil {
		s.logger.Warnf("failed to save final tex file copy: %v", err)
	}

	s.emit(emit, StageXeLaTeXCompilation, "Compiling the translated document with XeLaTeX")
	compileResult, err := s.compiler.Compile(ctx, compiler.Request{
		CompilerPath:        settings.Compiler.XeLaTeXPath,
		TeXRoot:             settings.Compiler.TeXRoot,
		WorkingDir:          ws.WorkingDir,
		TeXPath:             ws.TempTeXPath,
		LogPath:             ws.TempCompilerLogPath,
		Passes:              settings.Compiler.Passes,
		Timeout:             time.Duration(settings.Compiler.TimeoutSeconds) * time.Second,
		AutoInstallPackages: settings.Compiler.AutoInstallPackages,
	})

	if compileResult.CombinedLog != "" {
		_ = os.WriteFile(ws.SavedCompilerLogPath, []byte(compileResult.CombinedLog), 0o644)
	}

	if err != nil {
		s.logger.Errorf("xelatex compile failed: %v", err)
		s.emit(emit, StageFailed, compileErrorMessage(err, compileResult))
		return RunResult{
			InputPDFPath:         request.InputPDFPath,
			OutputDirectory:      request.OutputDirectory,
			ProviderName:         providerName,
			SourceLanguage:       sourceLanguage,
			TargetLanguage:       targetLanguage,
			SavedTeXPath:         ws.SavedTeXPath,
			CompilerLogPath:      ws.SavedCompilerLogPath,
			TranslationDebugPath: ws.TranslationDebugPath,
			OutputPreview:        excerpt(finalTeX, 2500),
			Warnings:             append(translationResult.Warnings, sanitized.Warnings...),
		}, err
	}

	if err := workspace.CopyFile(compileResult.OutputPDFPath, ws.SavedPDFPath); err != nil {
		s.logger.Errorf("failed to copy compiled pdf: %v", err)
		s.emit(emit, StageFailed, err.Error())
		return RunResult{}, err
	}

	result := RunResult{
		InputPDFPath:         request.InputPDFPath,
		OutputDirectory:      request.OutputDirectory,
		ProviderName:         providerName,
		SourceLanguage:       sourceLanguage,
		TargetLanguage:       targetLanguage,
		SavedTeXPath:         ws.SavedTeXPath,
		FinalPDFPath:         ws.SavedPDFPath,
		CompilerLogPath:      ws.SavedCompilerLogPath,
		TranslationDebugPath: ws.TranslationDebugPath,
		OutputPreview:        excerpt(finalTeX, 2500),
		Warnings:             append(translationResult.Warnings, sanitized.Warnings...),
	}

	s.mu.Lock()
	s.lastResult = &result
	s.mu.Unlock()

	s.logger.Infof("translation run completed successfully for %s", filepath.Base(request.InputPDFPath))
	s.emit(emit, StageDone, "Translated PDF, LaTeX draft, and build log were saved")
	return result, nil
}

func (s *Service) CompileExistingTeX(ctx context.Context, request CompileTeXRequest, emit func(ProgressEvent)) (RunResult, error) {
	settings := s.store.Current()

	texPath := strings.TrimSpace(request.TeXPath)
	if texPath == "" {
		err := fmt.Errorf("choose a .tex file to render")
		s.emit(emit, StageFailed, err.Error())
		return RunResult{}, err
	}
	if !strings.EqualFold(filepath.Ext(texPath), ".tex") {
		err := fmt.Errorf("selected file is not a .tex file: %s", texPath)
		s.emit(emit, StageFailed, err.Error())
		return RunResult{}, err
	}
	info, err := os.Stat(texPath)
	if err != nil {
		s.emit(emit, StageFailed, err.Error())
		return RunResult{}, err
	}
	if info.IsDir() {
		err := fmt.Errorf("selected path is a folder, not a .tex file: %s", texPath)
		s.emit(emit, StageFailed, err.Error())
		return RunResult{}, err
	}

	outputDir := strings.TrimSpace(request.OutputDirectory)
	if outputDir == "" {
		outputDir = filepath.Dir(texPath)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		s.emit(emit, StageFailed, err.Error())
		return RunResult{}, err
	}

	baseName := strings.TrimSuffix(filepath.Base(texPath), filepath.Ext(texPath))
	runID := time.Now().Format("20060102-150405")
	runDir := filepath.Join(outputDir, "_pdftranslator_runs", fmt.Sprintf("%s-render-%s", baseName, runID))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		s.emit(emit, StageFailed, err.Error())
		return RunResult{}, err
	}

	runLogPath := filepath.Join(runDir, "xelatex.log")
	savedPDFPath := filepath.Join(outputDir, baseName+".pdf")
	savedLogPath := filepath.Join(outputDir, baseName+".xelatex.log")

	s.emit(emit, StageXeLaTeXCompilation, "Rendering saved LaTeX with XeLaTeX")
	compileResult, err := s.compiler.Compile(ctx, compiler.Request{
		CompilerPath:        settings.Compiler.XeLaTeXPath,
		TeXRoot:             settings.Compiler.TeXRoot,
		WorkingDir:          filepath.Dir(texPath),
		OutputDir:           runDir,
		TeXPath:             texPath,
		LogPath:             runLogPath,
		Passes:              settings.Compiler.Passes,
		Timeout:             time.Duration(settings.Compiler.TimeoutSeconds) * time.Second,
		AutoInstallPackages: settings.Compiler.AutoInstallPackages,
	})

	if compileResult.CombinedLog != "" {
		_ = os.WriteFile(savedLogPath, []byte(compileResult.CombinedLog), 0o644)
	}

	texPreview := ""
	if data, readErr := os.ReadFile(texPath); readErr == nil {
		texPreview = excerpt(string(data), 2500)
	}

	result := RunResult{
		InputPDFPath:    request.InputPDFPath,
		OutputDirectory: outputDir,
		SavedTeXPath:    texPath,
		CompilerLogPath: savedLogPath,
		OutputPreview:   texPreview,
	}

	if err != nil {
		s.logger.Errorf("manual xelatex render failed: %v", err)
		s.emit(emit, StageFailed, compileErrorMessage(err, compileResult))
		s.mu.Lock()
		s.lastResult = &result
		s.mu.Unlock()
		return result, err
	}

	if err := workspace.CopyFile(compileResult.OutputPDFPath, savedPDFPath); err != nil {
		s.logger.Errorf("failed to copy rendered pdf: %v", err)
		s.emit(emit, StageFailed, err.Error())
		s.mu.Lock()
		s.lastResult = &result
		s.mu.Unlock()
		return result, err
	}

	result.FinalPDFPath = savedPDFPath
	s.mu.Lock()
	s.lastResult = &result
	s.mu.Unlock()

	s.logger.Infof("manual tex render completed successfully for %s", filepath.Base(texPath))
	s.emit(emit, StageDone, "Rendered PDF from saved LaTeX without another API call")
	return result, nil
}

func (s *Service) DiscoverCompilers(root string) []compiler.Installation {
	settings := s.store.Current()
	if strings.TrimSpace(root) == "" {
		root = settings.Compiler.TeXRoot
	}

	return compiler.DiscoverInstallations(root)
}

func (s *Service) EnsureCompilerConfigured() CompilerCheck {
	settings := s.store.Current()
	binaryPath, err := compiler.ResolveBinary(settings.Compiler.XeLaTeXPath, settings.Compiler.TeXRoot)
	if err == nil {
		return CompilerCheck{
			Found:      true,
			BinaryPath: binaryPath,
			TeXRoot:    settings.Compiler.TeXRoot,
			Message:    "XeLaTeX is ready.",
		}
	}

	installations := compiler.DiscoverInstallations(settings.Compiler.TeXRoot)
	if len(installations) == 0 {
		message := "MiKTeX was not found. Please install MiKTeX or open Settings and choose your portable MiKTeX folder or xelatex.exe."
		s.logger.Warnf("%s last error: %v", message, err)
		return CompilerCheck{
			Found:   false,
			Message: message,
		}
	}

	installation := installations[0]
	settings.Compiler.XeLaTeXPath = installation.BinaryPath
	if strings.TrimSpace(installation.TeXRoot) != "" {
		settings.Compiler.TeXRoot = installation.TeXRoot
	}

	if saveErr := s.store.Save(settings); saveErr != nil {
		message := fmt.Sprintf("MiKTeX was found at %s, but the app could not save that setting: %v", installation.BinaryPath, saveErr)
		s.logger.Warnf(message)
		return CompilerCheck{
			Found:      true,
			BinaryPath: installation.BinaryPath,
			TeXRoot:    installation.TeXRoot,
			Message:    message,
		}
	}

	message := fmt.Sprintf("MiKTeX was found and saved: %s", installation.BinaryPath)
	s.logger.Infof(message)
	return CompilerCheck{
		Found:          true,
		AutoConfigured: true,
		BinaryPath:     installation.BinaryPath,
		TeXRoot:        installation.TeXRoot,
		Message:        message,
	}
}

func (s *Service) emit(emit func(ProgressEvent), stage Stage, message string) {
	if emit != nil {
		emit(ProgressEvent{
			Stage:     stage,
			Message:   message,
			Timestamp: time.Now(),
		})
	}
}

func writeTranslationDebug(path string, result translate.Result) error {
	payload := struct {
		Prompt   translate.Prompt   `json:"prompt"`
		Response translate.Response `json:"response"`
		Warnings []string           `json:"warnings"`
	}{
		Prompt:   result.Prompt,
		Response: result.Response,
		Warnings: result.Warnings,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func excerpt(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 || len(text) <= max {
		return text
	}

	return text[:max] + "\n..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func buildTitle(fileName, targetLanguage string) string {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	if base == "" {
		base = "Document"
	}

	return fmt.Sprintf("%s (%s)", base, targetLanguage)
}

func compileErrorMessage(err error, result compiler.Result) string {
	if result.LogExcerpt == "" {
		return err.Error()
	}

	return fmt.Sprintf("%s\n\nUseful XeLaTeX log lines:\n%s", err.Error(), result.LogExcerpt)
}
