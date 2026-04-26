package app

import "time"

type Stage string

const (
	StagePDFLoaded          Stage = "pdf_loaded"
	StageRequestPrepared    Stage = "request_prepared"
	StageTranslationRunning Stage = "translation_in_progress"
	StageLatexValidation    Stage = "latex_validation_in_progress"
	StageXeLaTeXCompilation Stage = "xelatex_compilation_in_progress"
	StageDone               Stage = "done"
	StageFailed             Stage = "failed"
)

type ProgressEvent struct {
	Stage     Stage
	Message   string
	Timestamp time.Time
}

type RunRequest struct {
	InputPDFPath    string
	OutputDirectory string
	SourceLanguage  string
	TargetLanguage  string
	ProviderName    string
}

type CompileTeXRequest struct {
	TeXPath         string
	OutputDirectory string
	InputPDFPath    string
}

type InputDocument struct {
	Path      string
	FileName  string
	SizeBytes int64
}

type RunResult struct {
	InputPDFPath         string
	OutputDirectory      string
	ProviderName         string
	SourceLanguage       string
	TargetLanguage       string
	SavedTeXPath         string
	FinalPDFPath         string
	CompilerLogPath      string
	TranslationDebugPath string
	OutputPreview        string
	Warnings             []string
}

type CompilerCheck struct {
	Found          bool
	AutoConfigured bool
	BinaryPath     string
	TeXRoot        string
	Message        string
}
