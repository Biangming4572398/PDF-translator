package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	appsvc "pdftranslator/internal/app"
	"pdftranslator/internal/config"
)

type page string

const (
	pageDashboard  page = "dashboard"
	pageCollection page = "collection"
	pageResults    page = "results"
)

type Shell struct {
	app     fyne.App
	window  fyne.Window
	service *appsvc.Service

	dashboard  *DashboardView
	collection *CollectionView
	result     *ResultView

	pages map[page]fyne.CanvasObject

	selectedPDF   string
	outputDir     string
	inputDocument appsvc.InputDocument
	stageLines    []string
	logLines      []string
	running       bool
	lastRunStage  appsvc.Stage

	progressMu           sync.RWMutex
	progressTimerCancel  context.CancelFunc
	progressStageStarted time.Time
	progressStageName    string
	progressStageIndex   int

	providerDisplay    map[string]string
	sidebarProvider    *widget.Label
	fullScreenAction   *widget.ToolbarAction
	baseURLEntry       *widget.Entry
	apiKeyEntry        *widget.Entry
	modelSelect        *widget.SelectEntry
	modelStatusLabel   *widget.Label
	modelFetchButton   *widget.Button
	suppressModelSave  bool
	suppressConfigSave bool
}

func NewShell(app fyne.App, window fyne.Window, service *appsvc.Service) *Shell {
	shell := &Shell{
		app:             app,
		window:          window,
		service:         service,
		outputDir:       defaultDownloadsDir(),
		stageLines:      []string{},
		logLines:        []string{},
		providerDisplay: map[string]string{},
	}

	descriptors := service.ProviderDescriptors()
	for _, descriptor := range descriptors {
		shell.providerDisplay[descriptor.Name] = descriptor.DisplayName
	}

	languages := config.SupportedTranslationLanguages()

	shell.dashboard = NewDashboardView(
		languages,
		shell.choosePDF,
		shell.chooseOutputFolder,
		shell.startTranslation,
	)
	shell.dashboard.SourceSelect.OnChanged = func(language string) {
		if language == config.LanguageChinese {
			shell.dashboard.TargetSelect.SetSelected(config.LanguageEnglish)
			return
		}
		if language == config.LanguageEnglish {
			shell.dashboard.TargetSelect.SetSelected(config.LanguageChinese)
		}
	}
	shell.dashboard.TargetSelect.OnChanged = func(language string) {
		if language == shell.dashboard.SourceSelect.Selected {
			if language == config.LanguageChinese {
				shell.dashboard.SourceSelect.SetSelected(config.LanguageEnglish)
				return
			}
			shell.dashboard.SourceSelect.SetSelected(config.LanguageChinese)
		}
	}
	shell.collection = NewCollectionView(shell.useCollectionPDF, shell.refreshCollection)
	shell.result = NewResultView(shell.renderSavedTeX, shell.chooseTeXForRender)

	shell.pages = map[page]fyne.CanvasObject{
		pageDashboard:  shell.dashboard.Root,
		pageCollection: shell.collection.Root,
		pageResults:    shell.result.Root,
	}

	settings := service.CurrentSettings()
	shell.dashboard.SourceSelect.SetSelected(settings.DefaultSourceLanguage)
	shell.dashboard.TargetSelect.SetSelected(settings.DefaultTargetLanguage)
	shell.dashboard.SetOutputFolder(shell.outputDir)
	shell.refreshProviderDisplay()
	shell.refreshCollection()
	shell.result.SetStatus("Choose a PDF to prepare the shared input/output workspace.")

	content := container.NewMax(
		shell.dashboard.Root,
		shell.collection.Root,
		shell.result.Root,
	)

	toolbar := shell.buildToolbar()

	sidebar := shell.buildSidebar()
	root := container.NewBorder(toolbar, nil, sidebar, nil, container.NewPadded(content))

	window.SetContent(root)
	window.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		shell.handleDroppedURIs(uris)
	})
	window.Canvas().SetOnTypedKey(func(event *fyne.KeyEvent) {
		switch event.Name {
		case fyne.KeyEscape:
			if shell.window.FullScreen() {
				shell.setFullScreen(false)
			}
		case fyne.KeyF11:
			shell.toggleFullScreen()
		}
	})

	shell.navigate(pageDashboard)
	shell.fetchToolbarModels()
	shell.checkCompilerOnStartup()
	return shell
}

func (s *Shell) buildToolbar() fyne.CanvasObject {
	urlLabel := widget.NewLabel("URL")
	urlLabel.Wrapping = fyne.TextTruncate

	baseURLEntry := widget.NewEntry()
	baseURLEntry.PlaceHolder = config.DefaultOpenRouterBaseURL
	baseURLEntry.OnSubmitted = func(_ string) {
		s.saveToolbarConnection()
	}
	s.baseURLEntry = baseURLEntry

	apiKeyLabel := widget.NewLabel("API Key")
	apiKeyLabel.Wrapping = fyne.TextTruncate

	apiKeyEntry := widget.NewPasswordEntry()
	apiKeyEntry.PlaceHolder = "Hidden"
	apiKeyEntry.OnSubmitted = func(_ string) {
		s.saveToolbarConnection()
	}
	s.apiKeyEntry = apiKeyEntry

	modelLabel := widget.NewLabel("Model")
	modelLabel.Wrapping = fyne.TextTruncate

	modelSelect := widget.NewSelectEntry(nil)
	modelSelect.PlaceHolder = "Choose a model"
	modelSelect.OnChanged = func(model string) {
		s.saveToolbarModel(model)
	}
	s.modelSelect = modelSelect

	modelStatusLabel := widget.NewLabel("Model list")
	modelStatusLabel.Wrapping = fyne.TextTruncate
	s.modelStatusLabel = modelStatusLabel

	modelFetchButton := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), s.fetchToolbarModels)
	s.modelFetchButton = modelFetchButton

	fullScreenButton := widget.NewButtonWithIcon("", theme.ViewFullScreenIcon(), s.toggleFullScreen)
	settingsButton := widget.NewButtonWithIcon("", theme.SettingsIcon(), s.showSettings)

	s.syncToolbarConnectionFields()
	s.syncToolbarModelSelection()

	return container.NewBorder(
		nil,
		nil,
		nil,
		container.NewHBox(fullScreenButton, settingsButton),
		container.NewHBox(
			urlLabel,
			container.NewGridWrap(fyne.NewSize(320, baseURLEntry.MinSize().Height), baseURLEntry),
			apiKeyLabel,
			container.NewGridWrap(fyne.NewSize(220, apiKeyEntry.MinSize().Height), apiKeyEntry),
			modelLabel,
			container.NewGridWrap(fyne.NewSize(360, modelSelect.MinSize().Height), modelSelect),
			modelFetchButton,
			container.NewGridWrap(fyne.NewSize(130, modelStatusLabel.MinSize().Height), modelStatusLabel),
		),
	)
}

func (s *Shell) buildSidebar() fyne.CanvasObject {
	title := widget.NewLabel("Translator")
	title.Wrapping = fyne.TextWrapWord

	subtitle := widget.NewLabel("A calm desktop workspace for PDF translation and local XeLaTeX builds.")
	subtitle.Wrapping = fyne.TextWrapWord

	currentProvider := widget.NewLabel("Provider: " + s.providerDisplay[s.service.CurrentSettings().CurrentProvider])
	currentProvider.Wrapping = fyne.TextWrapWord
	s.sidebarProvider = currentProvider

	// TODO: customize sidebar
	return widget.NewCard(
		"Navigation",
		"",
		container.NewVBox(
			title,
			subtitle,
			widget.NewSeparator(),
			currentProvider,
			widget.NewSeparator(),
			widget.NewButtonWithIcon("Dashboard", theme.HomeIcon(), func() {
				s.navigate(pageDashboard)
			}),
			widget.NewButtonWithIcon("Downloads", theme.DownloadIcon(), func() {
				s.refreshCollection()
				s.navigate(pageCollection)
			}),
			widget.NewButtonWithIcon("Results", theme.DocumentPrintIcon(), func() {
				s.navigate(pageResults)
			}),
		),
	)
}

func (s *Shell) navigate(target page) {
	for pageID, object := range s.pages {
		if pageID == target {
			object.Show()
		} else {
			object.Hide()
		}
	}
}

func (s *Shell) toggleFullScreen() {
	s.setFullScreen(!s.window.FullScreen())
}

func (s *Shell) setFullScreen(enabled bool) {
	s.window.SetFullScreen(enabled)
}

func (s *Shell) choosePDF() {
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.NewError(err, s.window).Show()
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()

		s.applyInputSelection(reader.URI().Path())
	}, s.window)
	fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".pdf"}))
	fileDialog.Show()
}

func (s *Shell) chooseOutputFolder() {
	folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			dialog.NewError(err, s.window).Show()
			return
		}
		if uri == nil {
			return
		}

		s.outputDir = uri.Path()
		s.dashboard.SetOutputFolder(s.outputDir)
		s.result.SetStatus(fmt.Sprintf("Output folder ready: %s", s.outputDir))
	}, s.window)
	folderDialog.Show()
}

func (s *Shell) chooseTeXForRender() {
	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.NewError(err, s.window).Show()
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()

		texPath := reader.URI().Path()
		s.renderTeX(texPath, filepath.Dir(texPath))
	}, s.window)
	fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".tex"}))
	fileDialog.Show()
}

func (s *Shell) renderSavedTeX() {
	texPath := strings.TrimSpace(s.result.texPath)
	if texPath == "" {
		dialog.NewInformation("Choose LaTeX", "There is no saved LaTeX draft selected yet. Please choose a .tex file to render.", s.window).Show()
		return
	}

	outputDir := strings.TrimSpace(s.outputDir)
	if outputDir == "" {
		outputDir = filepath.Dir(texPath)
	}

	s.renderTeX(texPath, outputDir)
}

func (s *Shell) renderTeX(texPath, outputDir string) {
	if s.running {
		dialog.NewInformation("Please wait", "A translation or rendering job is already running.", s.window).Show()
		return
	}

	s.running = true
	s.startWorkflowProgress()
	s.lastRunStage = appsvc.StageXeLaTeXCompilation
	s.result.SetStatus("Rendering LaTeX locally with XeLaTeX. No API call will be made.")
	s.appendLog("Rendering LaTeX locally: " + texPath)
	s.navigate(pageResults)

	request := appsvc.CompileTeXRequest{
		TeXPath:         texPath,
		OutputDirectory: outputDir,
		InputPDFPath:    s.selectedPDF,
	}

	go func() {
		result, err := s.service.CompileExistingTeX(context.Background(), request, func(event appsvc.ProgressEvent) {
			fyne.Do(func() {
				s.appendStage(event)
			})
		})

		fyne.Do(func() {
			defer func() {
				s.running = false
				s.stopWorkflowProgressTimer()
			}()

			if result.SavedTeXPath != "" {
				s.result.SetResult(result)
			}
			if err != nil {
				s.result.SetStatus("Rendering failed. The LaTeX file and XeLaTeX log are still available for review.")
				dialog.NewError(err, s.window).Show()
				return
			}

			s.outputDir = result.OutputDirectory
			s.dashboard.SetOutputFolder(s.outputDir)
			s.result.SetStatus("Rendering complete. The PDF was generated from the selected LaTeX file without another API call.")
			s.appendLog("Rendered PDF from LaTeX: " + result.FinalPDFPath)
		})
	}()
}

func (s *Shell) applyInputSelection(path string) {
	document, err := s.service.InspectInputPDF(path)
	if err != nil {
		dialog.NewError(err, s.window).Show()
		return
	}

	s.selectedPDF = path
	s.inputDocument = document
	if strings.TrimSpace(s.outputDir) == "" {
		s.outputDir = filepath.Dir(path)
	}

	s.dashboard.SetSelectedPDF(path)
	s.dashboard.SetOutputFolder(s.outputDir)
	s.dashboard.SetCurrentStage("PDF loaded and ready for translation.")
	s.result.SetStatus("Source PDF prepared. Translation output will appear here after the run completes.")
	s.appendLog(fmt.Sprintf("Prepared input PDF: %s", document.FileName))
	s.navigate(pageDashboard)
}

func (s *Shell) useCollectionPDF(path string) {
	s.applyInputSelection(path)
}

func (s *Shell) refreshCollection() {
	downloadsDir := defaultDownloadsDir()
	s.collection.SetDirectory(downloadsDir)

	items, err := scanPDFs(downloadsDir)
	if err != nil {
		dialog.NewError(err, s.window).Show()
		return
	}

	s.collection.SetItems(items)
}

func (s *Shell) handleDroppedURIs(uris []fyne.URI) {
	for _, uri := range uris {
		if strings.EqualFold(uri.Extension(), ".pdf") {
			s.applyInputSelection(uri.Path())
			return
		}
	}

	dialog.NewInformation("Drop a PDF", "Please drop a PDF file so the app can prepare it for translation.", s.window).Show()
}

func (s *Shell) startTranslation() {
	s.saveToolbarConnection()

	if s.running {
		return
	}
	if strings.TrimSpace(s.selectedPDF) == "" {
		dialog.NewInformation("Choose a PDF", "Please choose a PDF before starting the translation.", s.window).Show()
		return
	}
	if strings.TrimSpace(s.outputDir) == "" {
		dialog.NewInformation("Choose an Output Folder", "Please choose an output folder so the files have a safe place to go.", s.window).Show()
		return
	}

	s.running = true
	s.dashboard.SetTranslationRunning(true)
	s.startWorkflowProgress()
	s.lastRunStage = ""
	s.stageLines = nil
	s.logLines = nil
	s.refreshProgressText()
	s.navigate(pageDashboard)

	request := appsvc.RunRequest{
		InputPDFPath:    s.selectedPDF,
		OutputDirectory: s.outputDir,
		SourceLanguage:  s.dashboard.SourceSelect.Selected,
		TargetLanguage:  s.dashboard.TargetSelect.Selected,
		ProviderName:    s.service.CurrentSettings().CurrentProvider,
	}

	go func() {
		result, err := s.service.RunTranslation(context.Background(), request, func(event appsvc.ProgressEvent) {
			fyne.Do(func() {
				s.appendStage(event)
			})
		})

		fyne.Do(func() {
			defer func() {
				s.running = false
				s.stopWorkflowProgressTimer()
				s.dashboard.SetTranslationRunning(false)
			}()

			if err != nil {
				if result.InputPDFPath != "" {
					s.result.SetResult(result)
				}
				s.result.SetStatus("The run stopped before a final PDF was produced. The saved LaTeX draft and logs should help with troubleshooting.")
				s.showRunFailureNotice(err, result)
				return
			}

			s.result.SetResult(result)
			s.result.SetStatus("Translation complete. The final PDF, LaTeX draft, and build log were saved.")
			s.appendLog("Translation completed successfully.")
			s.navigate(pageResults)
		})
	}()
}

func (s *Shell) appendStage(event appsvc.ProgressEvent) {
	line := fmt.Sprintf("%s  %s", event.Timestamp.Format("15:04:05"), humanStage(event.Stage)+": "+event.Message)
	s.stageLines = append(s.stageLines, line)
	if event.Stage != appsvc.StageFailed {
		s.lastRunStage = event.Stage
		s.updateWorkflowProgress(event.Stage, event.Timestamp)
	}
	s.dashboard.SetCurrentStage(humanStage(event.Stage) + ".")
	s.result.SetStatus(event.Message)
	s.appendLog(event.Message)
	s.refreshProgressText()
}

func (s *Shell) startWorkflowProgress() {
	s.stopWorkflowProgressTimer()

	now := time.Now()
	s.progressMu.Lock()
	s.progressStageStarted = now
	s.progressStageName = "Starting"
	s.progressStageIndex = 0
	ctx, cancel := context.WithCancel(context.Background())
	s.progressTimerCancel = cancel
	s.progressMu.Unlock()

	s.dashboard.SetWorkflowProgress(0, workflowActionTotal(), "Starting", 0)

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.progressMu.RLock()
				started := s.progressStageStarted
				stageName := s.progressStageName
				stageIndex := s.progressStageIndex
				s.progressMu.RUnlock()

				elapsed := time.Since(started)
				fyne.Do(func() {
					s.dashboard.SetWorkflowProgress(stageIndex, workflowActionTotal(), stageName, elapsed)
				})
			}
		}
	}()
}

func (s *Shell) stopWorkflowProgressTimer() {
	s.progressMu.Lock()
	cancel := s.progressTimerCancel
	s.progressTimerCancel = nil
	s.progressMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (s *Shell) updateWorkflowProgress(stage appsvc.Stage, started time.Time) {
	index, stageName := workflowProgressPosition(stage)
	if started.IsZero() {
		started = time.Now()
	}

	s.progressMu.Lock()
	s.progressStageStarted = started
	s.progressStageName = stageName
	s.progressStageIndex = index
	s.progressMu.Unlock()

	s.dashboard.SetWorkflowProgress(index, workflowActionTotal(), stageName, 0)
}

func (s *Shell) showRunFailureNotice(err error, result appsvc.RunResult) {
	title := "Translation failed"
	stageName := humanStage(s.lastRunStage)
	body := "The translation request stopped before the app could produce a final PDF."

	switch s.lastRunStage {
	case appsvc.StageLatexValidation:
		title = "LaTeX validation failed"
		body = "The provider returned content, but the LaTeX safety checks could not accept it."
	case appsvc.StageXeLaTeXCompilation:
		title = "Rendering failed"
		body = "The LaTeX draft was saved, but XeLaTeX could not render the final PDF."
	case appsvc.StageTranslationRunning:
		title = "Translation failed"
		body = "The provider request did not complete successfully."
	case appsvc.StagePDFLoaded, appsvc.StageRequestPrepared:
		title = "Request preparation failed"
		body = "The PDF was loaded, but the app could not prepare or send the raw-PDF request."
	}

	message := fmt.Sprintf("%s\n\nStage: %s\n\nDetails:\n%s", body, stageName, err.Error())
	if result.SavedTeXPath != "" {
		message += "\n\nSaved LaTeX:\n" + result.SavedTeXPath
	}
	if result.CompilerLogPath != "" {
		message += "\n\nCompiler log:\n" + result.CompilerLogPath
	}

	label := widget.NewLabel(message)
	label.Wrapping = fyne.TextWrapWord

	notice := dialog.NewCustomConfirm(
		title,
		"View Results",
		"Close",
		container.NewVScroll(label),
		func(viewResults bool) {
			if viewResults {
				s.navigate(pageResults)
			}
		},
		s.window,
	)
	notice.Resize(fyne.NewSize(560, 360))
	notice.Show()
}

func (s *Shell) appendLog(message string) {
	if strings.TrimSpace(message) == "" {
		return
	}

	s.logLines = append(s.logLines, message)
	s.refreshProgressText()
}

func (s *Shell) refreshProgressText() {
	s.dashboard.SetStageHistory(joinMessages(s.stageLines))
	s.dashboard.SetActivityLog(joinMessages(s.logLines))
}

func (s *Shell) refreshProviderDisplay() {
	displayName := s.providerDisplay[s.service.CurrentSettings().CurrentProvider]
	if displayName == "" {
		displayName = s.service.CurrentSettings().CurrentProvider
	}

	s.dashboard.SetProvider(displayName)
	if s.sidebarProvider != nil {
		s.sidebarProvider.SetText("Provider: " + displayName)
	}
	s.syncToolbarConnectionFields()
	s.syncToolbarModelSelection()
}

func (s *Shell) showSettings() {
	ShowSettingsWindow(
		s.app,
		s.service.CurrentSettings(),
		s.service.ProviderDescriptors(),
		s.service.DiscoverCompilers,
		s.service.FetchModels,
		func(updated config.Settings) error {
			if err := s.service.SaveSettings(updated); err != nil {
				return err
			}

			s.dashboard.SourceSelect.SetSelected(updated.DefaultSourceLanguage)
			s.dashboard.TargetSelect.SetSelected(updated.DefaultTargetLanguage)
			s.refreshProviderDisplay()
			s.fetchToolbarModels()
			return nil
		},
	)
}

func (s *Shell) checkCompilerOnStartup() {
	go func() {
		time.Sleep(600 * time.Millisecond)
		status := s.service.EnsureCompilerConfigured()

		fyne.Do(func() {
			if status.Found {
				if status.AutoConfigured {
					s.appendLog(status.Message)
					s.result.SetStatus("MiKTeX was detected and saved. XeLaTeX is ready for local PDF generation.")
				}
				return
			}

			s.appendLog(status.Message)
			s.result.SetStatus(status.Message)
			dialog.NewError(errors.New(status.Message), s.window).Show()
		})
	}()
}

func (s *Shell) syncToolbarModelSelection() {
	if s.modelSelect == nil {
		return
	}

	settings := s.service.CurrentSettings()
	cfg := settings.Provider(settings.CurrentProvider)
	s.suppressModelSave = true
	s.modelSelect.SetText(cfg.Model)
	s.suppressModelSave = false
}

func (s *Shell) fetchToolbarModels() {
	if s.modelSelect == nil || s.modelFetchButton == nil {
		return
	}

	s.saveToolbarConnection()

	settings := s.service.CurrentSettings()
	providerName := settings.CurrentProvider
	cfg := settings.Provider(providerName)

	s.modelStatusLabel.SetText("Fetching...")
	s.modelFetchButton.Disable()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		models, err := s.service.FetchModels(ctx, providerName, cfg)
		fyne.Do(func() {
			s.modelFetchButton.Enable()
			if err != nil {
				s.modelStatusLabel.SetText("Model list unavailable")
				return
			}

			options := modelOptions(models)
			selected := cfg.Model
			if selected == "" && len(options) > 0 {
				selected = options[0]
			}

			s.suppressModelSave = true
			s.modelSelect.SetOptions(options)
			s.modelSelect.SetText(selected)
			s.suppressModelSave = false

			if cfg.Model == "" && selected != "" {
				s.saveToolbarModel(selected)
			}
			s.modelStatusLabel.SetText(fmt.Sprintf("%d available", len(options)))
		})
	}()
}

func (s *Shell) saveToolbarModel(model string) {
	if s.suppressModelSave {
		return
	}

	model = strings.TrimSpace(model)
	if model == "" {
		return
	}

	settings := s.service.CurrentSettings()
	providerName := settings.CurrentProvider
	cfg := settings.Provider(providerName)
	if cfg.Model == model {
		return
	}

	settings.Providers = cloneProviderConfigs(settings.Providers)
	cfg.Model = model
	settings.Providers[providerName] = cfg
	if err := s.service.SaveSettings(settings); err != nil && s.modelStatusLabel != nil {
		s.modelStatusLabel.SetText("Model not saved")
		return
	}
	if s.modelStatusLabel != nil {
		s.modelStatusLabel.SetText("Model saved")
	}
}

func (s *Shell) syncToolbarConnectionFields() {
	if s.baseURLEntry == nil || s.apiKeyEntry == nil {
		return
	}

	settings := s.service.CurrentSettings()
	cfg := settings.Provider(settings.CurrentProvider)

	s.suppressConfigSave = true
	s.baseURLEntry.SetText(cfg.BaseURL)
	s.apiKeyEntry.SetText(cfg.APIKey)
	s.suppressConfigSave = false
}

func (s *Shell) saveToolbarConnection() {
	if s.suppressConfigSave || s.baseURLEntry == nil || s.apiKeyEntry == nil {
		return
	}

	settings := s.service.CurrentSettings()
	providerName := settings.CurrentProvider
	cfg := settings.Provider(providerName)
	baseURL := strings.TrimSpace(s.baseURLEntry.Text)
	apiKey := strings.TrimSpace(s.apiKeyEntry.Text)

	if cfg.BaseURL == baseURL && cfg.APIKey == apiKey {
		return
	}

	settings.Providers = cloneProviderConfigs(settings.Providers)
	cfg.BaseURL = baseURL
	cfg.APIKey = apiKey
	settings.Providers[providerName] = cfg

	if err := s.service.SaveSettings(settings); err != nil && s.modelStatusLabel != nil {
		s.modelStatusLabel.SetText("Connection not saved")
		return
	}
	if s.modelStatusLabel != nil {
		s.modelStatusLabel.SetText("Connection saved")
	}
}

func cloneProviderConfigs(input map[string]config.ProviderConfig) map[string]config.ProviderConfig {
	output := make(map[string]config.ProviderConfig, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func workflowActionTotal() int {
	return 5
}

func workflowProgressPosition(stage appsvc.Stage) (int, string) {
	switch stage {
	case appsvc.StagePDFLoaded:
		return 1, "PDF loaded"
	case appsvc.StageRequestPrepared:
		return 2, "Request prepared"
	case appsvc.StageTranslationRunning:
		return 3, "Translation in progress"
	case appsvc.StageLatexValidation:
		return 4, "LaTeX validation"
	case appsvc.StageXeLaTeXCompilation:
		return 5, "XeLaTeX rendering"
	case appsvc.StageDone:
		return workflowActionTotal(), "Done"
	default:
		return 0, "Starting"
	}
}

func humanStage(stage appsvc.Stage) string {
	switch stage {
	case appsvc.StagePDFLoaded:
		return "PDF loaded"
	case appsvc.StageRequestPrepared:
		return "Request prepared"
	case appsvc.StageTranslationRunning:
		return "Translation in progress"
	case appsvc.StageLatexValidation:
		return "LaTeX validation in progress"
	case appsvc.StageXeLaTeXCompilation:
		return "XeLaTeX compilation in progress"
	case appsvc.StageDone:
		return "Done"
	case appsvc.StageFailed:
		return "Failed"
	default:
		return string(stage)
	}
}
