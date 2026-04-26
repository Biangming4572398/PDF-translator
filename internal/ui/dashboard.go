package ui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type DashboardView struct {
	Root              fyne.CanvasObject
	ProviderLabel     *widget.Label
	InputPathLabel    *widget.Label
	OutputPathLabel   *widget.Label
	StageLabel        *widget.Label
	ProgressBar       *widget.ProgressBar
	ProgressLabel     *widget.Label
	StageTimerLabel   *widget.Label
	StageHistoryLabel *widget.Label
	ActivityLogLabel  *widget.Label
	DropHintLabel     *widget.Label
	SourceSelect      *widget.Select
	TargetSelect      *widget.Select
	StartButton       *widget.Button
}

func NewDashboardView(
	languages []string,
	onChoosePDF func(),
	onChooseOutput func(),
	onStart func(),
) *DashboardView {
	title := canvas.NewText("PDF Translation Desk", color.NRGBA{R: 0x18, G: 0x24, B: 0x32, A: 0xFF})
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 24

	subtitle := widget.NewLabel("Bring in a PDF, choose where the translated files should go, and keep the build notes close at hand.")
	subtitle.Wrapping = fyne.TextWrapWord

	providerLabel := widget.NewLabel("Starter Stub Provider")
	providerLabel.Wrapping = fyne.TextWrapWord

	inputPathLabel := widget.NewLabel("No PDF selected yet.")
	inputPathLabel.Wrapping = fyne.TextWrapWord

	outputPathLabel := widget.NewLabel("No output folder selected yet.")
	outputPathLabel.Wrapping = fyne.TextWrapWord

	stageLabel := widget.NewLabel("Waiting for a PDF.")
	stageLabel.Wrapping = fyne.TextWrapWord

	progressBar := widget.NewProgressBar()
	progressBar.Min = 0
	progressBar.Max = 1
	progressBar.SetValue(0)

	progressLabel := widget.NewLabel("0 of 5 planned actions")
	progressLabel.Wrapping = fyne.TextWrapWord

	stageTimerLabel := widget.NewLabel("Current stage time: 00:00")
	stageTimerLabel.Wrapping = fyne.TextWrapWord

	stageHistoryLabel := widget.NewLabel("The stage-by-stage progress will appear here.")
	stageHistoryLabel.Wrapping = fyne.TextWrapWord

	activityLogLabel := widget.NewLabel("Helpful notes, warnings, and build updates will appear here.")
	activityLogLabel.Wrapping = fyne.TextWrapWord

	dropHintLabel := widget.NewLabel("Drop a PDF anywhere in this window, or choose one using the button below.")
	dropHintLabel.Wrapping = fyne.TextWrapWord

	sourceSelect := widget.NewSelect(languages, nil)
	targetSelect := widget.NewSelect(languages, nil)
	sourceSelect.SetSelected("Chinese")
	targetSelect.SetSelected("English")

	choosePDFButton := widget.NewButtonWithIcon("Choose PDF", theme.FileIcon(), onChoosePDF)
	chooseOutputButton := widget.NewButtonWithIcon("Choose Output Folder", theme.FolderOpenIcon(), onChooseOutput)
	startButton := widget.NewButtonWithIcon("Start Translation", theme.ConfirmIcon(), onStart)
	startButton.Importance = widget.HighImportance

	// TODO: customize main dashboard layout
	intakeCard := widget.NewCard(
		"Document Intake",
		"Keep the workflow simple: PDF in, translation draft out, XeLaTeX build locally.",
		container.NewVBox(
			dropHintLabel,
			widget.NewSeparator(),
			widget.NewLabel("Current provider"),
			providerLabel,
			widget.NewSeparator(),
			widget.NewLabel("Selected PDF"),
			inputPathLabel,
			choosePDFButton,
			widget.NewSeparator(),
			widget.NewLabel("Output folder"),
			outputPathLabel,
			chooseOutputButton,
			widget.NewSeparator(),
			widget.NewLabel("Source language"),
			sourceSelect,
			widget.NewLabel("Target language"),
			targetSelect,
			startButton,
		),
	)

	stageScroll := container.NewVScroll(stageHistoryLabel)
	stageScroll.SetMinSize(fyne.NewSize(0, 180))

	logScroll := container.NewVScroll(activityLogLabel)
	logScroll.SetMinSize(fyne.NewSize(0, 180))

	statusCard := widget.NewCard(
		"Workflow Status",
		"Each stage is recorded so problems are easier to diagnose and the draft files stay available.",
		container.NewVBox(
			widget.NewLabel("Current stage"),
			stageLabel,
			widget.NewSeparator(),
			widget.NewLabel("Progress"),
			progressBar,
			progressLabel,
			stageTimerLabel,
			widget.NewSeparator(),
			widget.NewLabel("Stage history"),
			stageScroll,
			widget.NewSeparator(),
			widget.NewLabel("Activity notes"),
			logScroll,
		),
	)

	hero := widget.NewCard(
		"Welcome",
		"Built for a calm, dependable translation workflow on Windows.",
		container.NewVBox(title, subtitle),
	)

	content := container.NewVBox(
		hero,
		container.NewAdaptiveGrid(2, intakeCard, statusCard),
	)
	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(640, 420))

	return &DashboardView{
		Root:              scroll,
		ProviderLabel:     providerLabel,
		InputPathLabel:    inputPathLabel,
		OutputPathLabel:   outputPathLabel,
		StageLabel:        stageLabel,
		ProgressBar:       progressBar,
		ProgressLabel:     progressLabel,
		StageTimerLabel:   stageTimerLabel,
		StageHistoryLabel: stageHistoryLabel,
		ActivityLogLabel:  activityLogLabel,
		DropHintLabel:     dropHintLabel,
		SourceSelect:      sourceSelect,
		TargetSelect:      targetSelect,
		StartButton:       startButton,
	}
}

func (v *DashboardView) SetProvider(name string) {
	v.ProviderLabel.SetText(name)
}

func (v *DashboardView) SetSelectedPDF(path string) {
	if path == "" {
		v.InputPathLabel.SetText("No PDF selected yet.")
		return
	}

	v.InputPathLabel.SetText(path)
}

func (v *DashboardView) SetOutputFolder(path string) {
	if path == "" {
		v.OutputPathLabel.SetText("No output folder selected yet.")
		return
	}

	v.OutputPathLabel.SetText(path)
}

func (v *DashboardView) SetCurrentStage(message string) {
	v.StageLabel.SetText(message)
}

func (v *DashboardView) SetWorkflowProgress(current, total int, stageName string, elapsed time.Duration) {
	if total <= 0 {
		total = 1
	}
	if current < 0 {
		current = 0
	}
	if current > total {
		current = total
	}
	if stageName == "" {
		stageName = "Waiting"
	}

	v.ProgressBar.SetValue(float64(current) / float64(total))
	if current == total {
		v.ProgressLabel.SetText(fmt.Sprintf("%d of %d actions complete: %s", current, total, stageName))
	} else {
		v.ProgressLabel.SetText(fmt.Sprintf("%d of %d planned actions: %s", current, total, stageName))
	}
	v.StageTimerLabel.SetText("Current stage time: " + formatStageElapsed(elapsed))
}

func (v *DashboardView) SetStageHistory(message string) {
	if message == "" {
		message = "The stage-by-stage progress will appear here."
	}

	v.StageHistoryLabel.SetText(message)
}

func (v *DashboardView) SetActivityLog(message string) {
	if message == "" {
		message = "Helpful notes, warnings, and build updates will appear here."
	}

	v.ActivityLogLabel.SetText(message)
}

func (v *DashboardView) SetTranslationRunning(running bool) {
	if running {
		v.StartButton.SetText("Translation Running...")
		v.StartButton.Disable()
		return
	}

	v.StartButton.SetText("Start Translation")
	v.StartButton.Enable()
}

func formatStageElapsed(elapsed time.Duration) string {
	if elapsed < 0 {
		elapsed = 0
	}

	totalSeconds := int(elapsed.Round(time.Second).Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
