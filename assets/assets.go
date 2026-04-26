package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconBytes []byte

var AppIcon fyne.Resource = fyne.NewStaticResource("icon.png", iconBytes)
