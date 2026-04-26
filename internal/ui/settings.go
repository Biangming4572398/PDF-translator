package ui

import (
	"context"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"pdftranslator/internal/compiler"
	"pdftranslator/internal/config"
	"pdftranslator/internal/translate"
)

func ShowSettingsWindow(
	app fyne.App,
	current config.Settings,
	descriptors []translate.ProviderDescriptor,
	discoverCompilers func(string) []compiler.Installation,
	fetchModels func(context.Context, string, config.ProviderConfig) ([]translate.ModelDescriptor, error),
	onSave func(config.Settings) error,
) {
	settingsCopy := current
	if settingsCopy.Providers == nil {
		settingsCopy.Providers = map[string]config.ProviderConfig{}
	}

	nameToDisplay := map[string]string{}
	displayToName := map[string]string{}
	displayNames := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		nameToDisplay[descriptor.Name] = descriptor.DisplayName
		displayToName[descriptor.DisplayName] = descriptor.Name
		displayNames = append(displayNames, descriptor.DisplayName)
	}

	providerSelect := widget.NewSelect(displayNames, nil)
	apiKeyEntry := widget.NewPasswordEntry()
	baseURLEntry := widget.NewEntry()
	modelEntry := widget.NewSelectEntry(nil)
	timeoutEntry := widget.NewEntry()
	xelatexPathEntry := widget.NewEntry()
	texRootEntry := widget.NewEntry()
	passesEntry := widget.NewEntry()
	mainFontEntry := widget.NewEntry()
	autoInstallCheck := widget.NewCheck("Allow MiKTeX to install missing packages during compilation", nil)
	detectionLabel := widget.NewLabel("Choose a MiKTeX folder or xelatex.exe if it is not already on PATH.")
	detectionLabel.Wrapping = fyne.TextWrapWord
	modelStatusLabel := widget.NewLabel("Fetch models after changing the base URL or API key.")
	modelStatusLabel.Wrapping = fyne.TextWrapWord
	settingsWindow := app.NewWindow("Connection and Build Settings")
	settingsWindow.Resize(fyne.NewSize(760, 600))

	activeProvider := current.CurrentProvider

	stashActiveProvider := func() {
		cfg := settingsCopy.Provider(activeProvider)
		cfg.APIKey = apiKeyEntry.Text
		cfg.BaseURL = baseURLEntry.Text
		cfg.Model = modelEntry.Text
		cfg.TimeoutSeconds = parseIntOrDefault(timeoutEntry.Text, cfg.TimeoutSeconds, config.DefaultProviderTimeout)
		settingsCopy.Providers[activeProvider] = cfg
	}

	loadProvider := func(name string) {
		activeProvider = name
		cfg := settingsCopy.Provider(name)
		apiKeyEntry.SetText(cfg.APIKey)
		baseURLEntry.SetText(cfg.BaseURL)
		modelEntry.SetText(cfg.Model)
		modelEntry.SetOptions(nil)
		timeoutEntry.SetText(strconv.Itoa(cfg.TimeoutSeconds))
	}

	providerSelect.OnChanged = func(displayName string) {
		stashActiveProvider()
		loadProvider(displayToName[displayName])
	}

	if displayName, ok := nameToDisplay[current.CurrentProvider]; ok {
		providerSelect.SetSelected(displayName)
	}
	loadProvider(current.CurrentProvider)

	xelatexPathEntry.SetText(current.Compiler.XeLaTeXPath)
	texRootEntry.SetText(current.Compiler.TeXRoot)
	passesEntry.SetText(strconv.Itoa(current.Compiler.Passes))
	mainFontEntry.SetText(current.Compiler.MainFont)
	autoInstallCheck.SetChecked(current.Compiler.AutoInstallPackages)

	applyInstallation := func(installation compiler.Installation) {
		xelatexPathEntry.SetText(installation.BinaryPath)
		if strings.TrimSpace(installation.TeXRoot) != "" {
			texRootEntry.SetText(installation.TeXRoot)
		}
		detectionLabel.SetText("Using " + installation.BinaryPath)
	}

	findButton := widget.NewButtonWithIcon("Find MiKTeX", theme.SearchIcon(), func() {
		if discoverCompilers == nil {
			dialog.NewInformation("MiKTeX Detection", "Compiler detection is not available in this build.", settingsWindow).Show()
			return
		}

		installations := discoverCompilers(texRootEntry.Text)
		if len(installations) == 0 {
			dialog.NewInformation(
				"MiKTeX Detection",
				"No xelatex.exe was found. Please choose the portable MiKTeX folder or browse directly to xelatex.exe.",
				settingsWindow,
			).Show()
			return
		}

		applyInstallation(installations[0])
	})

	var fetchModelsButton *widget.Button
	fetchModelsButton = widget.NewButtonWithIcon("Fetch Models", theme.ViewRefreshIcon(), func() {
		if fetchModels == nil {
			dialog.NewInformation("Model List", "Model discovery is not available in this build.", settingsWindow).Show()
			return
		}

		stashActiveProvider()
		providerName := activeProvider
		providerConfig := settingsCopy.Provider(providerName)
		providerConfig.APIKey = apiKeyEntry.Text
		providerConfig.BaseURL = baseURLEntry.Text
		providerConfig.Model = modelEntry.Text

		modelStatusLabel.SetText("Fetching models...")
		fetchModelsButton.Disable()

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()

			models, err := fetchModels(ctx, providerName, providerConfig)
			fyne.Do(func() {
				fetchModelsButton.Enable()
				if err != nil {
					modelStatusLabel.SetText("Could not fetch models: " + err.Error())
					return
				}

				options := modelOptions(models)
				modelEntry.SetOptions(options)
				if modelEntry.Text == "" && len(options) > 0 {
					modelEntry.SetText(options[0])
				}
				modelStatusLabel.SetText(strconv.Itoa(len(options)) + " models available from this base URL.")
			})
		}()
	})

	// TODO: customize translation settings UI
	browseXeLaTeXButton := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.NewError(err, settingsWindow).Show()
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()

			xelatexPathEntry.SetText(reader.URI().Path())
			detectionLabel.SetText("Using " + reader.URI().Path())
		}, settingsWindow)
		fileDialog.SetFilter(storage.NewExtensionFileFilter([]string{".exe"}))
		fileDialog.Show()
	})

	browseRootButton := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.NewError(err, settingsWindow).Show()
				return
			}
			if uri == nil {
				return
			}

			texRootEntry.SetText(uri.Path())
			if discoverCompilers == nil {
				return
			}

			installations := discoverCompilers(uri.Path())
			if len(installations) > 0 {
				applyInstallation(installations[0])
			} else {
				detectionLabel.SetText("Folder selected. xelatex.exe was not found inside it yet.")
			}
		}, settingsWindow)
		folderDialog.Show()
	})

	form := widget.NewForm(
		widget.NewFormItem("Provider", providerSelect),
		widget.NewFormItem("API key", apiKeyEntry),
		widget.NewFormItem("Base URL", baseURLEntry),
		widget.NewFormItem("Model", container.NewBorder(nil, nil, nil, fetchModelsButton, modelEntry)),
		widget.NewFormItem("Model list", modelStatusLabel),
		widget.NewFormItem("Timeout (sec)", timeoutEntry),
		widget.NewFormItem("MiKTeX folder", container.NewBorder(nil, nil, nil, browseRootButton, texRootEntry)),
		widget.NewFormItem("XeLaTeX path", container.NewBorder(nil, nil, nil, browseXeLaTeXButton, xelatexPathEntry)),
		widget.NewFormItem("MiKTeX detection", container.NewVBox(findButton, detectionLabel)),
		widget.NewFormItem("MiKTeX packages", autoInstallCheck),
		widget.NewFormItem("XeLaTeX passes", passesEntry),
		widget.NewFormItem("Main font", mainFontEntry),
	)

	saveButton := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		stashActiveProvider()
		settingsCopy.CurrentProvider = activeProvider
		settingsCopy.Compiler.XeLaTeXPath = xelatexPathEntry.Text
		settingsCopy.Compiler.TeXRoot = texRootEntry.Text
		settingsCopy.Compiler.Passes = parseIntOrDefault(passesEntry.Text, current.Compiler.Passes, 2)
		settingsCopy.Compiler.MainFont = mainFontEntry.Text
		settingsCopy.Compiler.AutoInstallPackages = autoInstallCheck.Checked

		if err := onSave(settingsCopy); err != nil {
			dialog.NewError(err, settingsWindow).Show()
			return
		}

		settingsWindow.Close()
	})
	closeButton := widget.NewButtonWithIcon("Close", theme.CancelIcon(), settingsWindow.Close)

	actions := container.NewBorder(
		nil,
		nil,
		nil,
		container.NewHBox(closeButton, saveButton),
		nil,
	)

	settingsWindow.SetContent(container.NewBorder(
		nil,
		container.NewPadded(actions),
		nil,
		nil,
		container.NewVScroll(container.NewPadded(form)),
	))
	settingsWindow.Show()
	settingsWindow.RequestFocus()
}

func modelOptions(models []translate.ModelDescriptor) []string {
	options := make([]string, 0, len(models))
	for _, model := range models {
		if strings.TrimSpace(model.ID) == "" {
			continue
		}
		options = append(options, model.ID)
	}
	return options
}

func parseIntOrDefault(raw string, currentValue int, fallback int) int {
	if raw == "" {
		if currentValue > 0 {
			return currentValue
		}
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		if currentValue > 0 {
			return currentValue
		}
		return fallback
	}

	return value
}
