package app

import (
	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"

	"pdftranslator/internal/compiler"
	"pdftranslator/internal/config"
	"pdftranslator/internal/infra/logging"
	"pdftranslator/internal/infra/workspace"
	"pdftranslator/internal/latex"
	"pdftranslator/internal/translate"
	"pdftranslator/internal/translate/providers/openaicompat"
	"pdftranslator/internal/translate/providers/stub"
)

type Bootstrap struct {
	FyneApp fyne.App
	Store   *config.Store
	Logger  *logging.Logger
	Service *Service
}

func BootstrapDesktop() (*Bootstrap, error) {
	fyneDesktopApp := fyneapp.NewWithID(config.AppID)

	store, err := config.NewStore()
	if err != nil {
		return nil, err
	}

	logger, err := logging.New(store.Directory())
	if err != nil {
		return nil, err
	}

	registry := translate.NewRegistry(
		stub.New(),
		openaicompat.New(),
	)

	service := NewService(
		store,
		logger,
		workspace.NewManager(),
		translate.NewService(registry, translate.NewPromptBuilder()),
		latex.NewValidator(),
		latex.NewRenderer(),
		compiler.NewEngine(),
	)

	return &Bootstrap{
		FyneApp: fyneDesktopApp,
		Store:   store,
		Logger:  logger,
		Service: service,
	}, nil
}

func (b *Bootstrap) Close() error {
	if b.Logger == nil {
		return nil
	}

	return b.Logger.Close()
}
