package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type friendlyTheme struct {
	base fyne.Theme
}

func NewFriendlyTheme() fyne.Theme {
	return &friendlyTheme{base: theme.LightTheme()}
}

func (t *friendlyTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0xF7, G: 0xF9, B: 0xFC, A: 0xFF}
	case theme.ColorNameButton:
		return color.NRGBA{R: 0xEA, G: 0xEF, B: 0xF5, A: 0xFF}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x70, G: 0x78, B: 0x85, A: 0xFF}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 0xE1, G: 0xE6, B: 0xEE, A: 0xFF}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0x18, G: 0x24, B: 0x32, A: 0xFF}
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x1C, G: 0x7C, B: 0x54, A: 0xFF}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	case theme.ColorNameInputBorder:
		return color.NRGBA{R: 0x98, G: 0xA6, B: 0xB8, A: 0xFF}
	case theme.ColorNameHeaderBackground:
		return color.NRGBA{R: 0xE8, G: 0xF1, B: 0xF7, A: 0xFF}
	case theme.ColorNameHover:
		return color.NRGBA{R: 0xD7, G: 0xE7, B: 0xF1, A: 0xFF}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0x1C, G: 0x7C, B: 0x54, A: 0x66}
	case theme.ColorNameHyperlink:
		return color.NRGBA{R: 0x0B, G: 0x5C, B: 0xA8, A: 0xFF}
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0x57, G: 0x64, B: 0x75, A: 0xFF}
	case theme.ColorNamePressed, theme.ColorNameSelection:
		return color.NRGBA{R: 0x1C, G: 0x7C, B: 0x54, A: 0x30}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0x45, G: 0x55, B: 0x66, A: 0xCC}
	case theme.ColorNameScrollBarBackground:
		return color.NRGBA{R: 0xDD, G: 0xE5, B: 0xEE, A: 0xFF}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 0xC9, G: 0xD3, B: 0xDF, A: 0xFF}
	default:
		return t.base.Color(name, variant)
	}
}

func (t *friendlyTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t *friendlyTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *friendlyTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return t.base.Size(name) + 1
	case theme.SizeNameHeadingText:
		return t.base.Size(name) + 2
	case theme.SizeNamePadding:
		return t.base.Size(name) + 1
	default:
		return t.base.Size(name)
	}
}
