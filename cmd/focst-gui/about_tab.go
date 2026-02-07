package main

import (
	"fmt"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/oukeidos/focst/internal/licenses"
	"github.com/oukeidos/focst/internal/version"
)

const githubURL = "https://github.com/oukeidos/focst"

func buildAboutTab(w fyne.Window) fyne.CanvasObject {
	aboutSection := container.NewVBox(
		widget.NewLabelWithStyle("About", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewForm(
			widget.NewFormItem("App", widget.NewLabel("focst")),
			widget.NewFormItem("Version", widget.NewLabel(version.Version)),
			widget.NewFormItem("Commit", widget.NewLabel(version.Commit)),
			widget.NewFormItem("Build", widget.NewLabel(version.BuildDate)),
			widget.NewFormItem("Copyright", widget.NewLabel("(c) 2026 oukeidos")),
			widget.NewFormItem("Links", buildLinksRow()),
		),
	)

	viewLicenseBtn := widget.NewButton("View LICENSE", func() {
		text := licenses.LicenseText()
		if strings.TrimSpace(text) == "" {
			dialog.ShowError(fmt.Errorf("embedded LICENSE is empty; run scripts/collect_third_party_licenses.py"), w)
			return
		}
		showTextDialog(w, "LICENSE", text)
	})

	viewNoticesBtn := widget.NewButton("View THIRD_PARTY_NOTICES", func() {
		text := licenses.NoticesText()
		if strings.TrimSpace(text) == "" {
			dialog.ShowError(fmt.Errorf("embedded THIRD_PARTY_NOTICES is empty; run scripts/collect_third_party_licenses.py"), w)
			return
		}
		showTextDialog(w, "Third-Party Notices", text)
	})

	fullLicensesBtn := widget.NewButton("Open Full License Bundle", func() {
		text := licenses.FullText()
		if strings.TrimSpace(text) == "" {
			dialog.ShowError(fmt.Errorf("embedded THIRD_PARTY_LICENSES_FULL is empty; run scripts/collect_third_party_licenses.py"), w)
			return
		}
		showTextDialog(w, "Full Third-Party Licenses", text)
	})

	viewDisclaimerBtn := widget.NewButton("View Disclaimer", func() {
		text := licenses.DisclaimerText()
		if strings.TrimSpace(text) == "" {
			dialog.ShowError(fmt.Errorf("embedded DISCLAIMER is empty; run scripts/collect_third_party_licenses.py"), w)
			return
		}
		showTextDialog(w, "Disclaimer", text)
	})

	licensesSection := container.NewVBox(
		widget.NewLabelWithStyle("Licenses", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(viewLicenseBtn, viewNoticesBtn),
		fullLicensesBtn,
	)

	disclaimerSection := container.NewVBox(
		widget.NewLabelWithStyle("Disclaimer", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		viewDisclaimerBtn,
	)

	return container.NewPadded(container.NewVScroll(container.NewVBox(
		aboutSection,
		widget.NewSeparator(),
		licensesSection,
		widget.NewSeparator(),
		disclaimerSection,
	)))
}

func buildLinksRow() fyne.CanvasObject {
	githubLink := newHyperlink("GitHub", githubURL)
	return container.NewHBox(githubLink)
}

func newHyperlink(label, raw string) *widget.Hyperlink {
	u, _ := url.Parse(raw)
	return widget.NewHyperlink(label, u)
}

func showTextDialog(w fyne.Window, title, text string) {
	entry := widget.NewMultiLineEntry()
	entry.SetText(text)
	entry.Wrapping = fyne.TextWrapWord
	lock := false
	entry.OnChanged = func(s string) {
		if lock || s == text {
			return
		}
		lock = true
		entry.SetText(text)
		lock = false
	}
	scroll := container.NewScroll(entry)
	scroll.SetMinSize(fyne.NewSize(720, 520))
	d := dialog.NewCustom(title, "Close", scroll, w)
	d.Resize(fyne.NewSize(760, 560))
	d.Show()
}
