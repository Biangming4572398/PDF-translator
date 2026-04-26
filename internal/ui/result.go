package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	appsvc "pdftranslator/internal/app"
)

type ResultView struct {
	Root               fyne.CanvasObject
	StatusLabel        *widget.Label
	InputPathLabel     *widget.Label
	OutputPathLabel    *widget.Label
	TeXPathLabel       *widget.Label
	LogPathLabel       *widget.Label
	OutputPreviewLabel *widget.Label
	WarningsLabel      *widget.Label

	inputPath  string
	outputPath string
	logPath    string
	texPath    string
}

func NewResultView(onRenderSavedTeX func(), onChooseTeX func()) *ResultView {
	statusLabel := widget.NewLabel("Choose a PDF to prepare the shared input/output workspace.")
	statusLabel.Wrapping = fyne.TextWrapWord

	inputPathLabel := widget.NewLabel("No input PDF selected.")
	inputPathLabel.Wrapping = fyne.TextWrapWord

	outputPathLabel := widget.NewLabel("No translated PDF saved yet.")
	outputPathLabel.Wrapping = fyne.TextWrapWord

	texPathLabel := widget.NewLabel("No LaTeX draft saved yet.")
	texPathLabel.Wrapping = fyne.TextWrapWord

	logPathLabel := widget.NewLabel("No XeLaTeX log saved yet.")
	logPathLabel.Wrapping = fyne.TextWrapWord

	outputPreviewLabel := widget.NewLabel("The translated LaTeX preview will appear here after a run completes.")
	outputPreviewLabel.Wrapping = fyne.TextWrapWord

	warningsLabel := widget.NewLabel("Warnings and safeguards will be shown here when needed.")
	warningsLabel.Wrapping = fyne.TextWrapWord

	view := &ResultView{
		StatusLabel:        statusLabel,
		InputPathLabel:     inputPathLabel,
		OutputPathLabel:    outputPathLabel,
		TeXPathLabel:       texPathLabel,
		LogPathLabel:       logPathLabel,
		OutputPreviewLabel: outputPreviewLabel,
		WarningsLabel:      warningsLabel,
	}

	inputCard := widget.NewCard(
		"Input PDF",
		"Source file details.",
		container.NewVBox(
			widget.NewLabel("Source file"),
			inputPathLabel,
			widget.NewButtonWithIcon("Open Input PDF", theme.DocumentIcon(), func() {
				_ = openPathInSystem(view.inputPath)
			}),
		),
	)

	// TODO: customize result panel
	outputCard := widget.NewCard(
		"Translated Output",
		"Final PDF, generated LaTeX, and build notes stay together.",
		container.NewVBox(
			widget.NewLabel("Final PDF"),
			outputPathLabel,
			widget.NewButtonWithIcon("Open Final PDF", theme.DocumentPrintIcon(), func() {
				_ = openPathInSystem(view.outputPath)
			}),
			widget.NewSeparator(),
			widget.NewLabel("Saved LaTeX"),
			texPathLabel,
			container.NewHBox(
				widget.NewButtonWithIcon("Open LaTeX Draft", theme.DocumentCreateIcon(), func() {
					_ = openPathInSystem(view.texPath)
				}),
				widget.NewButtonWithIcon("Render Saved LaTeX", theme.MediaPlayIcon(), onRenderSavedTeX),
				widget.NewButtonWithIcon("Choose .tex and Render", theme.FileIcon(), onChooseTeX),
			),
			widget.NewSeparator(),
			widget.NewLabel("Compiler log"),
			logPathLabel,
			widget.NewButtonWithIcon("Open Build Log", theme.InfoIcon(), func() {
				_ = openPathInSystem(view.logPath)
			}),
			widget.NewSeparator(),
			widget.NewLabel("Translated preview"),
			outputPreviewLabel,
			widget.NewSeparator(),
			widget.NewLabel("Warnings"),
			warningsLabel,
		),
	)

	sharedScroll := container.NewVScroll(container.NewGridWithColumns(2, inputCard, outputCard))
	view.Root = container.NewBorder(
		widget.NewCard(
			"Results Workspace",
			"The input and output stay visible together so it is easier to review the run at a glance.",
			statusLabel,
		),
		nil,
		nil,
		nil,
		sharedScroll,
	)

	return view
}

func (v *ResultView) SetStatus(message string) {
	if strings.TrimSpace(message) == "" {
		message = "Choose a PDF to prepare the shared input/output workspace."
	}

	v.StatusLabel.SetText(message)
}

func (v *ResultView) SetResult(result appsvc.RunResult) {
	v.inputPath = result.InputPDFPath
	v.outputPath = result.FinalPDFPath
	v.texPath = result.SavedTeXPath
	v.logPath = result.CompilerLogPath

	v.InputPathLabel.SetText(result.InputPDFPath)

	if strings.TrimSpace(result.FinalPDFPath) == "" {
		v.OutputPathLabel.SetText("No translated PDF saved yet.")
	} else {
		v.OutputPathLabel.SetText(result.FinalPDFPath)
	}

	if strings.TrimSpace(result.SavedTeXPath) == "" {
		v.TeXPathLabel.SetText("No LaTeX draft saved yet.")
	} else {
		v.TeXPathLabel.SetText(result.SavedTeXPath)
	}

	if strings.TrimSpace(result.CompilerLogPath) == "" {
		v.LogPathLabel.SetText("No XeLaTeX log saved yet.")
	} else {
		v.LogPathLabel.SetText(result.CompilerLogPath)
	}

	if strings.TrimSpace(result.OutputPreview) != "" {
		v.OutputPreviewLabel.SetText(result.OutputPreview)
	}

	if len(result.Warnings) == 0 {
		v.WarningsLabel.SetText("Warnings and safeguards will be shown here when needed.")
	} else {
		v.WarningsLabel.SetText(strings.Join(result.Warnings, "\n"))
	}
}
