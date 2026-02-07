package main

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/oukeidos/focst/internal/auth"
	"github.com/oukeidos/focst/internal/language"
	"github.com/oukeidos/focst/internal/logger"
	"github.com/oukeidos/focst/internal/metadata"
	"github.com/oukeidos/focst/internal/pipeline"
	"github.com/oukeidos/focst/internal/recovery"
)

// largeTheme increases the base text size globally.
type largeTheme struct{ fyne.Theme }

func (m largeTheme) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNameText {
		return 23 // Match API set screen prominence
	}
	if n == theme.SizeNameCaptionText {
		return 17 // Scale captions accordingly
	}
	return theme.DefaultTheme().Size(n)
}

type AppState int

const (
	StateIdle AppState = iota
	StateProcessing
	StateSuccess
	StateFailure
	StatePartialSuccess
	StateCanceled
	StateNoKey
)

type focstApp struct {
	window  fyne.Window
	state   AppState
	content *fyne.Container

	// UI Components
	idleView           fyne.CanvasObject
	processingView     fyne.CanvasObject
	successView        fyne.CanvasObject
	failureView        fyne.CanvasObject
	partialSuccessView fyne.CanvasObject
	canceledView       fyne.CanvasObject
	apiKeyView         fyne.CanvasObject
	errorOverlay       *canvas.Rectangle

	// Runtime data
	isAnimating         bool
	sessionKey          string
	lastInputPath       string
	lastRecoveryLogPath string
	lastWasRepair       bool
	currentConfirmWin   fyne.Window
	currentSettingsWin  fyne.Window
	config              AppConfig
	activeDictLabel     *widget.Label
	cancelMu            sync.Mutex
	activeCancel        context.CancelFunc
	activeCancelID      uint64
	dictErrOnce         sync.Once
	panicNoticeOnce     sync.Once

	// Settings entries for syncing

	// Settings entries for syncing
	settingsGeminiEntry  *widget.Entry
	settingsOpenaiEntry  *widget.Entry
	settingsGeminiStatus *widget.Label
	settingsOpenaiStatus *widget.Label
}

type minSizeBox struct {
	size fyne.Size
	pos  fyne.Position
}

func (m *minSizeBox) MinSize() fyne.Size      { return m.size }
func (m *minSizeBox) Size() fyne.Size         { return m.size }
func (m *minSizeBox) Position() fyne.Position { return m.pos }
func (m *minSizeBox) Resize(s fyne.Size)      { m.size = s }
func (m *minSizeBox) Move(p fyne.Position)    { m.pos = p }
func (m *minSizeBox) Show()                   {}
func (m *minSizeBox) Hide()                   {}
func (m *minSizeBox) Visible() bool           { return false }
func (m *minSizeBox) Refresh()                {}

type fixedWidthEntry struct {
	widget.Entry
	width float32
}

func newFixedWidthEntry(width float32) *fixedWidthEntry {
	e := &fixedWidthEntry{width: width}
	e.ExtendBaseWidget(e)
	return e
}

func (e *fixedWidthEntry) MinSize() fyne.Size {
	size := e.Entry.MinSize()
	size.Width = e.width
	return size
}

func newFocstApp(w fyne.Window) *focstApp {
	a := &focstApp{window: w}
	a.loadConfig()
	a.setupUI()

	// Initial key check
	a.syncMainKeyState()

	return a
}

func (a *focstApp) setActiveCancel(cancel context.CancelFunc) uint64 {
	a.cancelMu.Lock()
	if a.activeCancel != nil {
		a.activeCancel()
	}
	a.activeCancel = cancel
	a.activeCancelID++
	id := a.activeCancelID
	a.cancelMu.Unlock()
	return id
}

func (a *focstApp) clearActiveCancel(id uint64) {
	a.cancelMu.Lock()
	if a.activeCancelID == id {
		a.activeCancel = nil
	}
	a.cancelMu.Unlock()
}

func (a *focstApp) cancelActive(reason string) {
	a.cancelMu.Lock()
	cancel := a.activeCancel
	a.activeCancel = nil
	a.cancelMu.Unlock()
	if cancel != nil {
		logger.Warn("Cancellation requested", "reason", reason)
		cancel()
	}
}

func (a *focstApp) syncMainKeyState() {
	if a.state == StateProcessing {
		return
	}
	key, _ := auth.GetKey("gemini", false)
	if key == "" && a.sessionKey == "" {
		a.setState(StateNoKey)
	} else {
		// Always call setState to ensure other layers are hidden during first run
		a.setState(StateIdle)
	}
}

func (a *focstApp) refreshSettingsEntries() {
	if a.currentSettingsWin == nil {
		return
	}
	gk, _ := auth.GetKey("gemini", false)
	ok, _ := auth.GetKey("openai", false)

	if a.settingsGeminiEntry != nil {
		a.settingsGeminiEntry.SetText("")
		a.settingsGeminiEntry.SetPlaceHolder("Enter new key")
	}
	if a.settingsOpenaiEntry != nil {
		a.settingsOpenaiEntry.SetText("")
		a.settingsOpenaiEntry.SetPlaceHolder("Enter new key")
	}
	if a.settingsGeminiStatus != nil {
		if gk != "" {
			a.settingsGeminiStatus.SetText("Saved in keychain")
		} else {
			a.settingsGeminiStatus.SetText("Not saved")
		}
	}
	if a.settingsOpenaiStatus != nil {
		if ok != "" {
			a.settingsOpenaiStatus.SetText("Saved in keychain")
		} else {
			a.settingsOpenaiStatus.SetText("Not saved")
		}
	}
}

// tappableIcon is a custom widget that implements Tappable and Hoverable.
type tappableIcon struct {
	widget.BaseWidget
	icon      *canvas.Image
	isHovered bool
	minSize   fyne.Size
	action    func()
}

func newTappableIcon(res fyne.Resource, action func(), minSize fyne.Size) *tappableIcon {
	icon := canvas.NewImageFromResource(res)
	icon.FillMode = canvas.ImageFillContain

	t := &tappableIcon{icon: icon, action: action, minSize: minSize}
	t.ExtendBaseWidget(t)
	return t
}

func newColoredIcon(res fyne.Resource, colorName fyne.ThemeColorName, action func()) *tappableIcon {
	// Create a themed resource that uses our specific color
	themed := theme.NewThemedResource(res)
	// Override the source color for this themed resource
	themed.ColorName = colorName

	icon := canvas.NewImageFromResource(themed)
	icon.FillMode = canvas.ImageFillContain

	t := &tappableIcon{icon: icon, action: action, minSize: fyne.NewSize(100, 100)}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableIcon) Tapped(_ *fyne.PointEvent) {
	if t.action != nil {
		t.action()
	}
}

func (t *tappableIcon) MouseIn(_ *desktop.MouseEvent) {
	t.setHover(true)
}

func (t *tappableIcon) MouseMoved(_ *desktop.MouseEvent) {
	t.setHover(true)
}

func (t *tappableIcon) MouseOut() {
	t.setHover(false)
}

func (t *tappableIcon) setHover(on bool) {
	safeDo("ui.tappable_icon.hover", func() {
		t.isHovered = on
		if on {
			t.icon.Translucency = 0.4 // Hover feedback
		} else {
			t.icon.Translucency = 0.0
		}
		t.icon.Refresh()
	})
}

func (t *tappableIcon) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func (t *tappableIcon) MinSize() fyne.Size {
	if t.minSize.Width > 0 && t.minSize.Height > 0 {
		return t.minSize
	}
	return fyne.NewSize(40, 40)
}

func (t *tappableIcon) CreateRenderer() fyne.WidgetRenderer {
	return &tappableIconRenderer{
		t:    t,
		icon: t.icon,
	}
}

type tappableIconRenderer struct {
	t    *tappableIcon
	icon *canvas.Image
}

func (r *tappableIconRenderer) Layout(s fyne.Size) {
	r.icon.Resize(s)
	r.icon.Move(fyne.NewPos(0, 0))
}

func (r *tappableIconRenderer) MinSize() fyne.Size {
	return r.t.MinSize()
}

func (r *tappableIconRenderer) Refresh() {
	canvas.Refresh(r.icon)
}

func (r *tappableIconRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.icon}
}

func (r *tappableIconRenderer) Destroy() {}

// largeSpinner is a custom breathing ring widget.
type largeSpinner struct {
	widget.BaseWidget
}

func newLargeSpinner() *largeSpinner {
	s := &largeSpinner{}
	s.ExtendBaseWidget(s)
	return s
}

func (s *largeSpinner) CreateRenderer() fyne.WidgetRenderer {
	c := canvas.NewCircle(color.Transparent)
	c.StrokeColor = theme.Color(theme.ColorNamePrimary)
	c.StrokeWidth = 8 // Even thicker

	r := &largeSpinnerRenderer{circle: c, s: s}

	safeGo("ui.spinner.animate", func() {
		for {
			for i := 0; i <= 20; i++ {
				alpha := uint8(50 + 150*float32(i)/20)
				baseColor := theme.Color(theme.ColorNamePrimary)
				red, g, b, _ := baseColor.RGBA()
				safeDo("ui.spinner.frame_in", func() {
					c.StrokeColor = color.NRGBA{R: uint8(red >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: alpha}
					r.Refresh()
				})
				time.Sleep(50 * time.Millisecond)
			}
			for i := 20; i >= 0; i-- {
				alpha := uint8(50 + 150*float32(i)/20)
				baseColor := theme.Color(theme.ColorNamePrimary)
				red, g, b, _ := baseColor.RGBA()
				safeDo("ui.spinner.frame_out", func() {
					c.StrokeColor = color.NRGBA{R: uint8(red >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: alpha}
					r.Refresh()
				})
				time.Sleep(50 * time.Millisecond)
			}
		}
	})

	return r
}

type largeSpinnerRenderer struct {
	circle *canvas.Circle
	s      *largeSpinner
}

func (r *largeSpinnerRenderer) Layout(size fyne.Size) {
	r.circle.Resize(size)
}

func (r *largeSpinnerRenderer) MinSize() fyne.Size {
	return fyne.NewSize(140, 140)
}

func (r *largeSpinnerRenderer) Refresh() {
	if r.circle != nil {
		canvas.Refresh(r.circle)
	}
}

func (r *largeSpinnerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.circle}
}

func (r *largeSpinnerRenderer) Destroy() {}

func (a *focstApp) setupUI() {
	// Pre-build all views once
	a.idleView = container.NewCenter(newDropZone(a.showFilePicker))
	a.processingView = container.NewCenter(newLargeSpinner())

	a.successView = container.NewCenter(newColoredIcon(theme.ConfirmIcon(), theme.ColorNameSuccess, func() { a.setState(StateIdle) }))
	a.failureView = container.NewCenter(newColoredIcon(theme.CancelIcon(), theme.ColorNameError, func() {
		a.showConfirmWindow("Retry Process", "The process failed. Would you like to retry?", func() {
			if a.lastWasRepair {
				go a.startRepair(a.lastInputPath)
			} else {
				go a.startTranslation(a.lastInputPath)
			}
		})
	}))
	a.partialSuccessView = container.NewCenter(newColoredIcon(theme.WarningIcon(), theme.ColorNameWarning, func() {
		a.showConfirmWindow("Repair Session", "Some segments failed. Would you like to attempt a repair now?", func() {
			logPath := a.partialSuccessRepairLogPath()
			go a.startRepair(logPath)
		})
	}))
	a.canceledView = container.NewCenter(newColoredIcon(theme.MediaStopIcon(), theme.ColorNameWarning, func() { a.setState(StateIdle) }))
	a.apiKeyView = a.createApiKeyView()

	// Settings button (Upper Right) - and make it small
	settingsBtn := newTappableIcon(theme.MoreVerticalIcon(), a.showSettingsWindow, fyne.NewSize(24, 24))
	settingsContainer := container.NewHBox(layout.NewSpacer(), container.NewPadded(settingsBtn))

	// Create a persistent error overlay
	a.errorOverlay = canvas.NewRectangle(color.Transparent)
	a.errorOverlay.Hide()

	a.activeDictLabel = widget.NewLabelWithStyle("Dictionary: None", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})

	// Content stack
	views := container.NewStack(
		a.idleView,
		a.processingView,
		a.successView,
		a.failureView,
		a.partialSuccessView,
		a.canceledView,
		a.apiKeyView,
	)

	a.content = container.NewStack(
		views,
		container.NewBorder(settingsContainer, a.activeDictLabel, nil, nil),
		a.errorOverlay,
	)

	a.window.SetContent(a.content)
	a.updateActiveDictLabel()
}

func (a *focstApp) updateActiveDictLabel() {
	if a.activeDictLabel == nil {
		return
	}
	if a.config.LastDict == "" || a.config.LastDict == "None (Empty)" {
		a.activeDictLabel.Hide()
	} else {
		name := a.config.LastDict
		runes := []rune(name)
		if len(runes) > 6 {
			name = string(runes[:5]) + ".."
		}
		a.activeDictLabel.SetText("D: " + name)
		a.activeDictLabel.Show()
	}
	if a.content != nil {
		a.content.Refresh()
	}
}

func (a *focstApp) showSettingsWindow() {
	if a.currentSettingsWin != nil {
		a.currentSettingsWin.RequestFocus()
		return
	}

	w := fyne.CurrentApp().NewWindow("Settings")
	a.currentSettingsWin = w
	w.SetOnClosed(func() {
		a.currentSettingsWin = nil
		a.settingsGeminiEntry = nil
		a.settingsOpenaiEntry = nil
	})

	// --- 1. Keys Tab ---
	a.settingsGeminiEntry = widget.NewPasswordEntry()
	a.settingsGeminiEntry.SetPlaceHolder("Enter new key")
	a.settingsOpenaiEntry = widget.NewPasswordEntry()
	a.settingsOpenaiEntry.SetPlaceHolder("Enter new key")

	a.settingsGeminiStatus = widget.NewLabel("")
	a.settingsOpenaiStatus = widget.NewLabel("")
	a.refreshSettingsEntries()

	keysTab := container.NewPadded(container.NewVBox(
		widget.NewLabelWithStyle("API Keys", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewForm(
			widget.NewFormItem("Gemini Key", container.NewVBox(a.settingsGeminiEntry, a.settingsGeminiStatus)),
			widget.NewFormItem("OpenAI Key", container.NewVBox(a.settingsOpenaiEntry, a.settingsOpenaiStatus)),
		),
		widget.NewButton("Save Keys to Keychain", func() {
			saveResult, err := saveKeysToKeychain(a.settingsGeminiEntry.Text, a.settingsOpenaiEntry.Text, auth.SaveKey)
			if saveResult.GeminiSaved {
				a.sessionKey = strings.TrimSpace(a.settingsGeminiEntry.Text)
			}
			a.syncMainKeyState()
			a.refreshSettingsEntries()
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			a.settingsGeminiEntry.SetText("")
			a.settingsOpenaiEntry.SetText("")
			dialog.ShowInformation("Saved", "API Keys have been updated in your keychain.", w)
		}),
		widget.NewSeparator(),
		widget.NewButtonWithIcon("Reset All Keys", theme.DeleteIcon(), func() {
			dialog.ShowConfirm("Reset", "Are you sure you want to delete all saved keys from keychain?", func(ok bool) {
				if ok {
					err := resetKeysInKeychain(auth.DeleteKey)
					a.refreshSettingsEntries()
					if err != nil {
						a.syncMainKeyState()
						dialog.ShowError(err, w)
						return
					}
					a.sessionKey = "" // Also clear session key on reset
					a.settingsGeminiEntry.SetText("")
					a.settingsOpenaiEntry.SetText("")
					a.syncMainKeyState()
					dialog.ShowInformation("Reset Complete", "All saved keys were deleted from keychain.", w)
				}
			}, w)
		}),
	))

	// --- 2. Languages Tab ---
	allLangs := language.GetSupportedLanguages()
	var langNames []string
	codeToName := make(map[string]string)
	nameToCode := make(map[string]string)
	for _, l := range allLangs {
		if l.ID == "zh" {
			continue // Skip redundant Chinese Simplified root
		}
		langNames = append(langNames, l.Name)
		codeToName[l.Code] = l.Name
		nameToCode[l.Name] = l.Code
	}
	refreshDictionaryOptions := func() {}

	srcSelect := newSearchableSelect(w, "Select Source Language", langNames, func(s string) {
		a.config.SourceLang = nameToCode[s]
		a.saveConfig()
		refreshDictionaryOptions()
	})
	srcSelect.SetText(codeToName[a.config.SourceLang])

	tgtSelect := newSearchableSelect(w, "Select Target Language", langNames, func(s string) {
		selectedCode := nameToCode[s]
		if selectedCode == a.config.SourceLang {
			dialog.ShowError(fmt.Errorf("source and target languages cannot be the same"), w)
			return
		}
		a.config.TargetLang = selectedCode
		a.saveConfig()
		refreshDictionaryOptions()
	})
	tgtSelect.SetText(codeToName[a.config.TargetLang])

	langsTab := container.NewPadded(container.NewVBox(
		widget.NewLabelWithStyle("Language Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewForm(
			widget.NewFormItem("Source Language", srcSelect),
			widget.NewFormItem("Target Language", tgtSelect),
		),
	))

	namesResultContainer := container.NewVBox()
	namesScroll := container.NewScroll(namesResultContainer)
	namesScroll.SetMinSize(fyne.NewSize(0, 150))

	refreshNamesResultUI := func() {
		namesResultContainer.Objects = nil
		for sKey, tVal := range a.config.NamesMapping {
			currS := sKey
			currT := tVal
			sEntry := widget.NewEntry()
			sEntry.SetText(currS)
			tEntry := widget.NewEntry()
			tEntry.SetText(currT)

			sEntry.OnChanged = func(newS string) {
				delete(a.config.NamesMapping, currS)
				currS = newS
				a.config.NamesMapping[newS] = tEntry.Text
			}
			tEntry.OnChanged = func(newT string) {
				a.config.NamesMapping[sEntry.Text] = newT
			}
			namesResultContainer.Add(container.NewGridWithColumns(2, sEntry, tEntry))
		}
		namesResultContainer.Refresh()
	}

	dictEntries := a.listDictionaryEntries(w)
	dictSelect := widget.NewSelect(nil, nil)
	dictFilterHint := widget.NewLabel("")
	showAllDictionaries := false
	selectedDictName := a.config.LastDict
	if selectedDictName == "" {
		selectedDictName = emptyDictionaryName
	}
	dictLabelToName := make(map[string]string)
	dictNameToLabel := make(map[string]string)
	buildDictionaryOptions := func() []string {
		filtered := a.filterDictionaryEntries(dictEntries, showAllDictionaries)
		options := make([]string, 0, len(filtered))
		dictLabelToName = make(map[string]string, len(filtered))
		dictNameToLabel = make(map[string]string, len(filtered))
		for _, entry := range filtered {
			label := entry.label()
			options = append(options, label)
			dictLabelToName[label] = entry.Name
			dictNameToLabel[entry.Name] = label
		}
		return options
	}
	setSelectedDictionary := func(name string) {
		mapping, err := a.loadDictionary(name)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		selectedDictName = name
		a.config.LastDict = name
		a.saveConfig()
		a.updateActiveDictLabel()
		a.config.NamesMapping = mapping
		refreshNamesResultUI()
	}

	var saveBtn *widget.Button
	updateOverwriteState := func() {
		if saveBtn == nil {
			return
		}
		if selectedDictName == "" || selectedDictName == emptyDictionaryName {
			saveBtn.Disable()
			return
		}
		saveBtn.Enable()
	}
	refreshDictionaryOptions = func() {
		dictEntries = a.listDictionaryEntries(w)
		dictSelect.Options = buildDictionaryOptions()

		noMatch := !showAllDictionaries && len(dictSelect.Options) == 1 && dictNameToLabel[emptyDictionaryName] != ""
		if noMatch {
			dictFilterHint.SetText("No dictionaries match current Source/Target. Enable 'Show all dictionaries' to view all.")
		} else {
			dictFilterHint.SetText("")
		}

		if _, ok := dictNameToLabel[selectedDictName]; !ok {
			if selectedDictName != emptyDictionaryName {
				setSelectedDictionary(emptyDictionaryName)
			}
			selectedDictName = emptyDictionaryName
		}

		label := dictNameToLabel[selectedDictName]
		if label == "" {
			label = dictNameToLabel[emptyDictionaryName]
		}
		dictSelect.SetSelected(label)
		dictSelect.Refresh()
		updateOverwriteState()
	}
	dictSelect.OnChanged = func(s string) {
		name, ok := dictLabelToName[s]
		if !ok || name == "" {
			name = emptyDictionaryName
		}
		if name == selectedDictName {
			updateOverwriteState()
			return
		}
		prevName := selectedDictName
		setSelectedDictionary(name)
		if selectedDictName != name {
			prevLabel := dictNameToLabel[prevName]
			if prevLabel == "" {
				prevLabel = dictNameToLabel[emptyDictionaryName]
			}
			if prevLabel != "" && dictSelect.Selected != prevLabel {
				dictSelect.SetSelected(prevLabel)
			}
		}
		updateOverwriteState()
	}
	showAllCheck := widget.NewCheck("Show all dictionaries", func(checked bool) {
		showAllDictionaries = checked
		refreshDictionaryOptions()
	})
	showAllCheck.SetChecked(false)
	refreshDictionaryOptions()

	deleteDictBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		sel := selectedDictName
		if sel == "" || sel == emptyDictionaryName {
			return
		}
		dialog.ShowConfirm("Delete Dictionary", "Are you sure you want to delete '"+sel+"'?", func(ok bool) {
			if ok {
				a.deleteDictionary(sel)
				a.clearDictionaryMeta(sel)
				setSelectedDictionary(emptyDictionaryName)
				refreshDictionaryOptions()
			}
		}, w)
	})

	workTitle := widget.NewEntry()
	workYear := widget.NewEntry()
	workType := widget.NewSelect([]string{"movie", "drama", "animation"}, nil)
	workType.SetSelected("movie")

	extractBtn := widget.NewButton("Run Name Extraction", nil)
	saveBtn = widget.NewButton("Overwrite Dictionary", nil)
	saveAsBtn := widget.NewButton("Save as New Dictionary", nil)
	updateOverwriteState()

	extractBtn.OnTapped = func() {
		if workTitle.Text == "" {
			dialog.ShowError(fmt.Errorf("please enter a title"), w)
			return
		}
		extractBtn.Disable()
		extractBtn.SetText("Extracting...")

		go a.startNameExtraction(workType.Selected, workTitle.Text, workYear.Text, w, func(mapping map[string]string, err error) {
			extractBtn.Enable()
			extractBtn.SetText("Run Name Extraction")
			if err != nil {
				if errors.Is(err, errOpenAIKeyMissing) {
					return
				}
				dialog.ShowError(err, w)
				return
			}
			a.config.NamesMapping = mapping
			refreshNamesResultUI()
		})
	}

	saveAction := func(savePath string) {
		err := a.saveDictionary(savePath)
		if err != nil {
			dialog.ShowError(err, w)
		} else {
			dialog.ShowInformation("Saved", "Dictionary has been saved.", w)
			base := filepath.Base(savePath)
			name := strings.TrimSuffix(base, ".json")
			a.setDictionaryMeta(name, a.config.SourceLang, a.config.TargetLang)
			setSelectedDictionary(name)
			refreshDictionaryOptions()
		}
	}

	saveBtn.OnTapped = func() {
		if selectedDictName == "" || selectedDictName == emptyDictionaryName {
			dialog.ShowError(fmt.Errorf("no dictionary selected to overwrite"), w)
			return
		}
		home, _ := os.UserHomeDir()
		path := filepath.Join(home, ".focst", "names", selectedDictName+".json")
		dialog.ShowConfirm("Confirm Overwrite", "Overwrite the existing dictionary file?", func(ok bool) {
			if ok {
				saveAction(path)
			}
		}, w)
	}

	saveAsBtn.OnTapped = func() {
		home, _ := os.UserHomeDir()
		namesDir := filepath.Join(home, ".focst", "names")
		fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil || writer == nil {
				return
			}
			defer writer.Close()

			savePath := writer.URI().Path()
			if filepath.Ext(savePath) == "" {
				savePath += ".json"
			}
			if !isPathWithinDir(savePath, namesDir) {
				dialog.ShowError(fmt.Errorf("save location must be inside %s", namesDir), w)
				return
			}
			saveAction(savePath)
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
		fd.SetFileName("names.json")
		if uri := storage.NewFileURI(namesDir); uri != nil {
			if lister, err := storage.ListerForURI(uri); err == nil {
				fd.SetLocation(lister)
			}
		}
		fd.Show()
	}

	resultsHeader := widget.NewLabelWithStyle("Results / Manual Editing", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	resultsButtons := container.NewHBox(saveBtn, saveAsBtn)
	resultsArea := container.NewBorder(resultsHeader, resultsButtons, nil, nil, namesScroll)

	namesTop := container.NewVBox(
		widget.NewLabelWithStyle("Dictionary Management", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(
			widget.NewLabel("Load Existing:"),
			container.NewGridWrap(fyne.NewSize(200, 40), dictSelect),
			container.NewGridWrap(fyne.NewSize(40, 40), deleteDictBtn),
		),
		showAllCheck,
		dictFilterHint,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Run New Extraction", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewForm(
			widget.NewFormItem("Media Title", workTitle),
			widget.NewFormItem("Year", workYear),
			widget.NewFormItem("Type", workType),
		),
		extractBtn,
		widget.NewSeparator(),
	)

	namesTab := container.NewPadded(container.NewBorder(namesTop, nil, nil, nil, resultsArea))

	// --- 4. Models Tab ---
	models := metadata.GeminiModelIDs()
	modelSelect := widget.NewSelect(models, func(s string) {
		a.config.Model = s
		a.saveConfig()
	})
	modelSelect.SetSelected(a.config.Model)

	modelsTab := container.NewPadded(container.NewVBox(
		widget.NewLabelWithStyle("Model Selection", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewForm(
			widget.NewFormItem("Completion Model", modelSelect),
		),
	))

	// --- 5. Advanced Tab ---
	chunkEntry := newFixedWidthEntry(120)
	chunkEntry.SetText(strconv.Itoa(a.config.ChunkSize))
	chunkEntry.OnChanged = func(s string) {
		if v, err := strconv.Atoi(s); err == nil {
			if v < 1 {
				logger.Warn("Chunk size clamped", "requested", v, "effective", 1)
				v = 1
				if strconv.Itoa(v) != s {
					chunkEntry.SetText(strconv.Itoa(v))
				}
			}
			if v > maxChunkSizeGUI {
				logger.Warn("Chunk size clamped", "requested", v, "effective", maxChunkSizeGUI)
				v = maxChunkSizeGUI
				if strconv.Itoa(v) != s {
					chunkEntry.SetText(strconv.Itoa(v))
				}
			}
			a.config.ChunkSize = v
			a.saveConfig()
		}
	}

	concurrencyEntry := newFixedWidthEntry(120)
	concurrencyEntry.SetText(strconv.Itoa(a.config.Concurrency))
	concurrencyEntry.OnChanged = func(s string) {
		if v, err := strconv.Atoi(s); err == nil {
			clamped, changed := pipeline.ClampConcurrency(v)
			if changed {
				logger.Warn("Concurrency clamped", "requested", v, "effective", clamped, "max", pipeline.MaxConcurrency)
			}
			a.config.Concurrency = clamped
			a.saveConfig()
			if changed && strconv.Itoa(clamped) != s {
				concurrencyEntry.SetText(strconv.Itoa(clamped))
			}
		}
	}

	contextEntry := newFixedWidthEntry(120)
	contextEntry.SetText(strconv.Itoa(a.config.ContextSize))
	contextEntry.OnChanged = func(s string) {
		if v, err := strconv.Atoi(s); err == nil {
			if v < 0 {
				logger.Warn("Context size clamped", "requested", v, "effective", 0)
				v = 0
				if strconv.Itoa(v) != s {
					contextEntry.SetText(strconv.Itoa(v))
				}
			}
			if v > maxContextSizeGUI {
				logger.Warn("Context size clamped", "requested", v, "effective", maxContextSizeGUI)
				v = maxContextSizeGUI
				if strconv.Itoa(v) != s {
					contextEntry.SetText(strconv.Itoa(v))
				}
			}
			a.config.ContextSize = v
			a.saveConfig()
		}
	}

	retryCheck := widget.NewCheck("Retry on Long Lines", func(b bool) {
		a.config.RetryOnLongLines = b
		a.saveConfig()
	})
	retryCheck.SetChecked(a.config.RetryOnLongLines)

	promptCPLCheck := widget.NewCheck("Prompt CPL Enforcement", func(b bool) {
		a.config.NoPromptCPL = !b
		a.saveConfig()
	})
	promptCPLCheck.SetChecked(!a.config.NoPromptCPL)

	preprocessCheck := widget.NewCheck("Preprocessing", func(b bool) {
		a.config.NoPreprocess = !b
		a.saveConfig()
	})
	preprocessCheck.SetChecked(!a.config.NoPreprocess)

	langPreprocessCheck := widget.NewCheck("Lang-Specific Preprocess", func(b bool) {
		a.config.NoLangPreprocess = !b
		a.saveConfig()
	})
	langPreprocessCheck.SetChecked(!a.config.NoLangPreprocess)

	postprocessCheck := widget.NewCheck("Postprocessing", func(b bool) {
		a.config.NoPostprocess = !b
		a.saveConfig()
	})
	postprocessCheck.SetChecked(!a.config.NoPostprocess)

	langPostprocessCheck := widget.NewCheck("Lang-Specific Postprocess", func(b bool) {
		a.config.NoLangPostprocess = !b
		a.saveConfig()
	})
	langPostprocessCheck.SetChecked(!a.config.NoLangPostprocess)

	maxTokensEntry := newFixedWidthEntry(120)
	maxTokensEntry.SetText(strconv.Itoa(a.config.ExtractionMaxTokens))
	maxTokensEntry.OnChanged = func(s string) {
		if v, err := strconv.Atoi(s); err == nil {
			if v > maxExtractionTokens {
				logger.Warn("Extraction max tokens clamped", "requested", v, "effective", maxExtractionTokens)
				v = maxExtractionTokens
				if strconv.Itoa(v) != s {
					maxTokensEntry.SetText(strconv.Itoa(v))
				}
			}
			a.config.ExtractionMaxTokens = v
			a.saveConfig()
		}
	}

	resetBtn := widget.NewButtonWithIcon("Reset to Defaults", theme.HistoryIcon(), func() {
		a.config.ChunkSize = 100
		a.config.Concurrency = 7
		a.config.ContextSize = 5
		a.config.RetryOnLongLines = false
		a.config.NoPromptCPL = false
		a.config.NoPreprocess = false
		a.config.NoPostprocess = false
		a.config.NoLangPreprocess = false
		a.config.NoLangPostprocess = false
		a.config.ExtractionMaxTokens = 16384

		// Update UI
		chunkEntry.SetText("100")
		concurrencyEntry.SetText("7")
		contextEntry.SetText("5")
		retryCheck.SetChecked(false)
		promptCPLCheck.SetChecked(true)
		preprocessCheck.SetChecked(true)
		langPreprocessCheck.SetChecked(true)
		postprocessCheck.SetChecked(true)
		langPostprocessCheck.SetChecked(true)
		maxTokensEntry.SetText("16384")

		a.saveConfig()
		dialog.ShowInformation("Reset", "Advanced settings have been reset to defaults.", w)
	})
	resetBtn.Resize(fyne.NewSize(220, 40))

	leftCol := container.NewVBox(
		widget.NewLabelWithStyle("Technical Parameters", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewForm(
			widget.NewFormItem("Concurrency (1-20)", concurrencyEntry),
			widget.NewFormItem("Chunk Size (1-200)", chunkEntry),
			widget.NewFormItem("Context Size (0-20)", contextEntry),
		),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Names Extraction", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewForm(
			widget.NewFormItem("Max Tokens (1-128000)", maxTokensEntry),
		),
	)

	rightCol := container.NewVBox(
		widget.NewLabelWithStyle("Processing Toggles", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		retryCheck,
		promptCPLCheck,
		preprocessCheck,
		langPreprocessCheck,
		postprocessCheck,
		langPostprocessCheck,
	)

	advancedGrid := container.NewGridWithColumns(2, leftCol, rightCol)
	advancedTab := container.NewPadded(container.NewVBox(
		advancedGrid,
		container.NewCenter(container.NewPadded(container.NewHBox(layout.NewSpacer(), resetBtn, layout.NewSpacer()))),
	))

	// --- 6. About Tab ---
	aboutTab := buildAboutTab(w)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Keys", theme.StorageIcon(), keysTab),
		container.NewTabItemWithIcon("Languages", theme.SearchReplaceIcon(), langsTab),
		container.NewTabItemWithIcon("Names", theme.DocumentIcon(), namesTab),
		container.NewTabItemWithIcon("Models", theme.GridIcon(), modelsTab),
		container.NewTabItemWithIcon("Advanced", theme.SettingsIcon(), advancedTab),
		container.NewTabItemWithIcon("About", theme.InfoIcon(), aboutTab),
	)

	minSize := tabs.MinSize()
	targetSize := fyne.NewSize(minSize.Width+80, minSize.Height+10)
	content := container.NewMax(&minSizeBox{size: targetSize}, tabs)

	w.SetContent(content)
	w.Resize(targetSize)
	w.CenterOnScreen()
	w.Show()
}

// newSearchableSelect creates a button that opens a searchable dialog for selection
func newSearchableSelect(w fyne.Window, title string, options []string, onSelected func(string)) *widget.Button {
	selectedLabel := "Select..."
	btn := widget.NewButton(selectedLabel, nil)

	btn.OnTapped = func() {
		searchEntry := widget.NewEntry()
		searchEntry.SetPlaceHolder("Search...")

		listItems := options
		list := widget.NewList(
			func() int { return len(listItems) },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(id widget.ListItemID, obj fyne.CanvasObject) {
				obj.(*widget.Label).SetText(listItems[id])
			},
		)

		var d dialog.Dialog
		list.OnSelected = func(id widget.ListItemID) {
			onSelected(listItems[id])
			btn.SetText(listItems[id])
			d.Hide()
		}

		searchEntry.OnChanged = func(s string) {
			s = strings.ToLower(s)
			listItems = nil
			for _, opt := range options {
				if strings.Contains(strings.ToLower(opt), s) {
					listItems = append(listItems, opt)
				}
			}
			list.Refresh()
		}

		content := container.NewBorder(searchEntry, nil, nil, nil, container.NewStack(list))
		d = dialog.NewCustom(title, "Cancel", content, w)
		d.Resize(fyne.NewSize(400, 500))
		d.Show()

		// Auto-focus search entry
		w.Canvas().Focus(searchEntry)
	}

	return btn
}

type keySaveResult struct {
	GeminiSaved bool
	OpenAISaved bool
}

func saveKeysToKeychain(geminiKey, openaiKey string, saveFn func(service, key string) error) (keySaveResult, error) {
	result := keySaveResult{}
	var errs []string
	if strings.TrimSpace(geminiKey) != "" {
		if err := saveFn("gemini", geminiKey); err != nil {
			errs = append(errs, fmt.Sprintf("failed to save Gemini key: %v", err))
		} else {
			result.GeminiSaved = true
		}
	}
	if strings.TrimSpace(openaiKey) != "" {
		if err := saveFn("openai", openaiKey); err != nil {
			errs = append(errs, fmt.Sprintf("failed to save OpenAI key: %v", err))
		} else {
			result.OpenAISaved = true
		}
	}
	if len(errs) > 0 {
		return result, errors.New(strings.Join(errs, "; "))
	}
	return result, nil
}

func resetKeysInKeychain(deleteFn func(service string) error) error {
	var errs []string
	if err := deleteFn("gemini"); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Gemini key: %v", err))
	}
	if err := deleteFn("openai"); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete OpenAI key: %v", err))
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// hugeButton is a custom button with large text for accessibility
type hugeButton struct {
	widget.BaseWidget
	text   *canvas.Text
	bg     *canvas.Rectangle
	action func()
}

func newHugeButton(label string, bgColor color.Color, action func()) *hugeButton {
	t := canvas.NewText(label, color.Black)
	t.TextSize = 24 // Substantially larger
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.Alignment = fyne.TextAlignCenter

	bg := canvas.NewRectangle(bgColor)
	bg.CornerRadius = 8

	b := &hugeButton{text: t, bg: bg, action: action}
	b.ExtendBaseWidget(b)
	return b
}

func (b *hugeButton) Tapped(_ *fyne.PointEvent) {
	if b.action != nil {
		b.action()
	}
}

func (b *hugeButton) CreateRenderer() fyne.WidgetRenderer {
	return &hugeButtonRenderer{b: b}
}

type hugeButtonRenderer struct {
	b *hugeButton
}

func (r *hugeButtonRenderer) Layout(s fyne.Size) {
	r.b.bg.Resize(s)
	r.b.text.Resize(s)
}
func (r *hugeButtonRenderer) MinSize() fyne.Size { return fyne.NewSize(85, 50) } // Larger buttons
func (r *hugeButtonRenderer) Refresh()           { r.b.bg.Refresh(); r.b.text.Refresh() }
func (r *hugeButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.b.bg, r.b.text}
}
func (r *hugeButtonRenderer) Destroy() {}

func (a *focstApp) createApiKeyView() fyne.CanvasObject {
	input := widget.NewPasswordEntry()
	input.SetPlaceHolder("API KEY")

	title := canvas.NewText("SET API KEY", color.Black)
	title.TextSize = 28 // Max Size
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	accent := theme.Color(theme.ColorNamePrimary)
	saveBtn := newHugeButton("SAVE", accent, func() {
		key := strings.TrimSpace(input.Text)
		if key == "" {
			a.flashRed()
			return
		}
		if err := auth.SaveKey("gemini", key); err != nil {
			a.flashRed()
			return
		}
		a.sessionKey = key
		a.syncMainKeyState()
		a.refreshSettingsEntries()
		input.SetText("")
	})

	onceBtn := newHugeButton("ONCE", color.NRGBA{R: 200, G: 200, B: 200, A: 255}, func() {
		key := strings.TrimSpace(input.Text)
		if key == "" {
			a.flashRed()
			return
		}
		a.sessionKey = key
		a.syncMainKeyState()
		input.SetText("")
		// Session key doesn't go to settings entries (persistent)
		// but main window state will change.
	})

	btns := container.NewGridWithColumns(2, saveBtn, onceBtn)

	card := container.NewVBox(
		container.NewCenter(title),
		input,
		container.NewPadded(btns),
	)

	return container.NewCenter(container.NewPadded(card))
}

func (a *focstApp) setState(s AppState) {
	a.safeDo("app.set_state", func() {
		a.state = s
		a.idleView.Hide()
		a.processingView.Hide()
		a.successView.Hide()
		a.failureView.Hide()
		a.partialSuccessView.Hide()
		a.canceledView.Hide()
		a.apiKeyView.Hide()

		switch s {
		case StateIdle:
			a.idleView.Show()
		case StateProcessing:
			a.processingView.Show()
		case StateNoKey:
			a.apiKeyView.Show()
		case StateSuccess:
			a.successView.Show()
		case StatePartialSuccess:
			a.partialSuccessView.Show()
		case StateFailure:
			a.failureView.Show()
		case StateCanceled:
			a.canceledView.Show()
		}

		a.content.Refresh()
	})
}

func (a *focstApp) flashRed() {
	if a.isAnimating {
		return
	}
	a.isAnimating = true

	a.safeDo("app.flash_red.start", func() {
		a.errorOverlay.Show()
		a.content.Refresh()
		fmt.Println("DEBUG: Starting Red Flash")
	})

	a.safeGo("app.flash_red.animate", func() {
		steps := 10
		duration := 150 * time.Millisecond
		sleep := duration / time.Duration(steps)

		// Fade in
		for i := 1; i <= steps; i++ {
			alpha := uint8(120 * float32(i) / float32(steps))
			a.safeDo("app.flash_red.fade_in", func() {
				a.errorOverlay.FillColor = color.NRGBA{R: 255, G: 0, B: 0, A: alpha}
				canvas.Refresh(a.errorOverlay)
			})
			time.Sleep(sleep)
		}
		// Fade out
		for i := steps; i >= 0; i-- {
			alpha := uint8(120 * float32(i) / float32(steps))
			a.safeDo("app.flash_red.fade_out", func() {
				a.errorOverlay.FillColor = color.NRGBA{R: 255, G: 0, B: 0, A: alpha}
				canvas.Refresh(a.errorOverlay)
			})
			time.Sleep(sleep)
		}

		a.safeDo("app.flash_red.end", func() {
			a.errorOverlay.FillColor = color.Transparent
			a.errorOverlay.Hide()
			a.isAnimating = false
			a.content.Refresh()
		})
	})
}

func (a *focstApp) showConfirmWindow(title, message string, onYes func()) {
	if a.currentConfirmWin != nil {
		a.currentConfirmWin.RequestFocus()
		return
	}

	confirmWin := fyne.CurrentApp().NewWindow(title)
	a.currentConfirmWin = confirmWin
	confirmWin.SetOnClosed(func() {
		a.currentConfirmWin = nil
	})

	msg := canvas.NewText(message, color.Black)
	msg.TextSize = 20
	msg.TextStyle = fyne.TextStyle{Bold: true}
	msg.Alignment = fyne.TextAlignCenter

	accent := theme.Color(theme.ColorNamePrimary)
	yesBtn := newHugeButton("YES", accent, func() {
		confirmWin.Close()
		onYes()
	})

	noBtn := newHugeButton("NO", color.NRGBA{R: 200, G: 200, B: 200, A: 255}, func() {
		confirmWin.Close()
		a.setState(StateIdle)
	})

	btns := container.NewGridWithColumns(2, yesBtn, noBtn)

	card := container.NewVBox(
		container.NewPadded(container.NewCenter(msg)),
		container.NewPadded(btns),
	)

	confirmWin.SetContent(container.NewCenter(container.NewPadded(card)))
	confirmWin.Resize(fyne.NewSize(450, 200))
	confirmWin.CenterOnScreen()
	confirmWin.Show()
}

func (a *focstApp) handleDropped(uri fyne.URI) {
	if a.state == StateProcessing {
		return
	}

	path := uri.Path()
	ext := strings.ToLower(filepath.Ext(path))

	// Track last input for retry/repair
	a.lastInputPath = path
	a.lastWasRepair = (ext == ".json")
	a.lastRecoveryLogPath = ""
	if a.lastWasRepair {
		a.lastRecoveryLogPath = path
	}

	// Supported subtitle formats: .srt, .vtt, .ttml, .stl, .ssa, .ass
	if ext == ".srt" || ext == ".vtt" || ext == ".ttml" || ext == ".stl" || ext == ".ssa" || ext == ".ass" {
		go a.startTranslation(path)
	} else if ext == ".json" {
		go a.startRepair(path)
	} else {
		a.flashRed()
	}
}

func (a *focstApp) partialSuccessRepairLogPath() string {
	if a.lastWasRepair {
		return a.lastInputPath
	}
	if a.lastRecoveryLogPath != "" {
		return a.lastRecoveryLogPath
	}
	// Fallback for unexpected missing state; preserved for backward compatibility.
	return recovery.GenerateRecoveryPath(a.lastInputPath)
}

func (a *focstApp) showFilePicker() {
	// Create a separate window for the file picker to avoid being cramped in 200x200
	pickerWin := fyne.CurrentApp().NewWindow("Select File")
	pickerWin.Resize(fyne.NewSize(1000, 800))

	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		defer pickerWin.Close()
		if err != nil || reader == nil {
			return
		}
		a.handleDropped(reader.URI())
		reader.Close()
	}, pickerWin)

	fd.SetFilter(storage.NewExtensionFileFilter([]string{".srt", ".vtt", ".ttml", ".stl", ".ssa", ".ass", ".json"}))
	fd.Resize(fyne.NewSize(1000, 800))
	pickerWin.Show()
	fd.Show()
}

func isPathWithinDir(path, dir string) bool {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return false
	}
	return true
}

type dropZone struct {
	widget.BaseWidget
	isHovered bool
	onTapped  func()
}

func newDropZone(onTapped func()) *dropZone {
	d := &dropZone{onTapped: onTapped}
	d.ExtendBaseWidget(d)
	return d
}

func (d *dropZone) Tapped(_ *fyne.PointEvent) {
	if d.onTapped != nil {
		d.onTapped()
	}
}

func (d *dropZone) MouseIn(_ *desktop.MouseEvent) {
	d.setHover(true)
}

func (d *dropZone) MouseMoved(_ *desktop.MouseEvent) {
	d.setHover(true)
}

func (d *dropZone) MouseOut() {
	d.setHover(false)
}

func (d *dropZone) setHover(on bool) {
	safeDo("ui.drop_zone.hover", func() {
		d.isHovered = on
		d.Refresh()
	})
}

func (d *dropZone) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

func (d *dropZone) CreateRenderer() fyne.WidgetRenderer {
	thickness := float32(4)
	size := float32(80)
	accentColor := color.NRGBA{R: 200, G: 200, B: 200, A: 255}

	hBar := canvas.NewRectangle(accentColor)
	hBar.Resize(fyne.NewSize(size, thickness))

	vBar := canvas.NewRectangle(accentColor)
	vBar.Resize(fyne.NewSize(thickness, size))

	bg := canvas.NewRectangle(color.Transparent)

	return &dropZoneRenderer{
		hBar: hBar,
		vBar: vBar,
		bg:   bg,
		d:    d,
	}
}

type dropZoneRenderer struct {
	hBar *canvas.Rectangle
	vBar *canvas.Rectangle
	bg   *canvas.Rectangle
	d    *dropZone
}

func (r *dropZoneRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	centerX, centerY := s.Width/2, s.Height/2
	r.hBar.Move(fyne.NewPos(centerX-r.hBar.Size().Width/2, centerY-r.hBar.Size().Height/2))
	r.vBar.Move(fyne.NewPos(centerX-r.vBar.Size().Width/2, centerY-r.vBar.Size().Height/2))
}

func (r *dropZoneRenderer) MinSize() fyne.Size { return fyne.NewSize(100, 100) }
func (r *dropZoneRenderer) Refresh() {
	accentColor := color.Color(color.NRGBA{R: 200, G: 200, B: 200, A: 255})
	if r.d.isHovered {
		accentColor = theme.Color(theme.ColorNamePrimary)
	}
	r.hBar.FillColor = accentColor
	r.vBar.FillColor = accentColor
	canvas.Refresh(r.hBar)
	canvas.Refresh(r.vBar)
}
func (r *dropZoneRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.hBar, r.vBar}
}
func (r *dropZoneRenderer) Destroy() {}

func main() {
	// Initialize logger for debug/error tracing
	logger.Init(logger.LevelInfo, nil)
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Unrecovered GUI panic", "scope", "main", "panic", fmt.Sprint(r))
			os.Exit(1)
		}
	}()

	myApp := app.NewWithID("com.focst.app")
	myApp.Settings().SetTheme(largeTheme{Theme: theme.DefaultTheme()})
	myApp.SetIcon(appIcon())

	w := myApp.NewWindow("focst")
	w.SetIcon(appIcon())
	w.SetMaster()
	w.Resize(fyne.NewSize(200, 200))
	w.SetFixedSize(true)
	w.CenterOnScreen()

	fa := newFocstApp(w)
	w.SetCloseIntercept(func() {
		fa.cancelActive("window closed")
		fa.sessionKey = ""
		fa.syncMainKeyState()
		w.SetCloseIntercept(nil)
		w.Close()
	})

	w.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		if len(uris) > 0 {
			fa.handleDropped(uris[0])
		}
	})

	w.ShowAndRun()
}
