package main

import (
	"log"

	"fyne.io/fyne/v2"

	"pdftranslator/assets"
	appsvc "pdftranslator/internal/app"
	"pdftranslator/internal/platform"
	"pdftranslator/internal/ui"
)

func main() {
	platform.EnableDPIAwareness()

	bootstrap, err := appsvc.BootstrapDesktop()
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}
	defer func() {
		if err := bootstrap.Close(); err != nil {
			log.Printf("logger close warning: %v", err)
		}
	}()

	bootstrap.FyneApp.Settings().SetTheme(ui.NewFriendlyTheme())
	bootstrap.FyneApp.SetIcon(assets.AppIcon)

	window := bootstrap.FyneApp.NewWindow("PDF Translator")
	window.SetIcon(assets.AppIcon)
	window.Resize(fyne.NewSize(1480, 920))
	window.SetMaster()
	ui.NewShell(bootstrap.FyneApp, window, bootstrap.Service)

	window.Show()
	bootstrap.FyneApp.Run()
}
