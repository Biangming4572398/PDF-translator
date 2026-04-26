package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type CollectionView struct {
	Root           fyne.CanvasObject
	DirectoryLabel *widget.Label
	EmptyLabel     *widget.Label
	list           *widget.List
	items          []PDFListItem
}

func NewCollectionView(onUse func(string), onRefresh func()) *CollectionView {
	directoryLabel := widget.NewLabel("No downloads folder found.")
	directoryLabel.Wrapping = fyne.TextWrapWord

	emptyLabel := widget.NewLabel("No PDF files were found in the default Downloads folder.")
	emptyLabel.Wrapping = fyne.TextWrapWord

	list := widget.NewList(
		func() int {
			return 0
		},
		func() fyne.CanvasObject {
			title := widget.NewLabel("Document.pdf")
			title.Wrapping = fyne.TextWrapWord
			meta := widget.NewLabel("0 KB")
			meta.Wrapping = fyne.TextWrapWord
			useButton := widget.NewButton("Use This PDF", nil)

			return container.NewHBox(
				widget.NewIcon(theme.DocumentIcon()),
				container.NewVBox(title, meta),
				useButton,
			)
		},
		func(id widget.ListItemID, object fyne.CanvasObject) {},
	)

	view := &CollectionView{
		DirectoryLabel: directoryLabel,
		EmptyLabel:     emptyLabel,
		list:           list,
	}

	view.list.Length = func() int {
		return len(view.items)
	}
	view.list.UpdateItem = func(id widget.ListItemID, object fyne.CanvasObject) {
		item := view.items[id]
		row := object.(*fyne.Container)
		title := row.Objects[1].(*fyne.Container).Objects[0].(*widget.Label)
		meta := row.Objects[1].(*fyne.Container).Objects[1].(*widget.Label)
		button := row.Objects[2].(*widget.Button)

		title.SetText(item.Name)
		meta.SetText(fmt.Sprintf("%s, updated %s", formatBytes(item.Size), item.Modified.Format("02 Jan 2006 15:04")))
		button.OnTapped = func() {
			onUse(item.Path)
		}
	}

	// TODO: customize sidebar
	header := widget.NewCard(
		"Downloads Collection",
		"Quickly pick a PDF from the default Windows Downloads folder.",
		container.NewBorder(
			nil,
			nil,
			nil,
			widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), onRefresh),
			directoryLabel,
		),
	)

	content := container.NewBorder(
		header,
		nil,
		nil,
		nil,
		container.NewStack(emptyLabel, list),
	)

	view.Root = content
	return view
}

func (v *CollectionView) SetDirectory(path string) {
	if path == "" {
		v.DirectoryLabel.SetText("No downloads folder found.")
		return
	}

	v.DirectoryLabel.SetText(path)
}

func (v *CollectionView) SetItems(items []PDFListItem) {
	v.items = items
	if len(items) == 0 {
		v.EmptyLabel.Show()
	} else {
		v.EmptyLabel.Hide()
	}
	v.list.Refresh()
}
