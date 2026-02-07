package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/oukeidos/focst/internal/files"
	"github.com/oukeidos/focst/internal/logger"
	"github.com/oukeidos/focst/internal/names"
)

const emptyDictionaryName = "None (Empty)"

type dictionaryEntry struct {
	Name       string
	SourceLang string
	TargetLang string
}

func (d dictionaryEntry) label() string {
	if d.Name == emptyDictionaryName {
		return d.Name
	}
	if d.SourceLang == "" || d.TargetLang == "" {
		return fmt.Sprintf("%s [unknown]", d.Name)
	}
	return fmt.Sprintf("%s [%s->%s]", d.Name, d.SourceLang, d.TargetLang)
}

func dictMetaSourceKey(name string) string { return "DictMeta." + name + ".source" }
func dictMetaTargetKey(name string) string { return "DictMeta." + name + ".target" }

func (a *focstApp) prefs() fyne.Preferences {
	app := fyne.CurrentApp()
	if app == nil {
		return nil
	}
	return app.Preferences()
}

func (a *focstApp) setDictionaryMeta(name, source, target string) {
	prefs := a.prefs()
	if prefs == nil || name == "" || name == emptyDictionaryName {
		return
	}
	prefs.SetString(dictMetaSourceKey(name), source)
	prefs.SetString(dictMetaTargetKey(name), target)
}

func (a *focstApp) getDictionaryMeta(name string) (string, string) {
	prefs := a.prefs()
	if prefs == nil || name == "" || name == emptyDictionaryName {
		return "", ""
	}
	return prefs.String(dictMetaSourceKey(name)), prefs.String(dictMetaTargetKey(name))
}

func (a *focstApp) clearDictionaryMeta(name string) {
	prefs := a.prefs()
	if prefs == nil || name == "" || name == emptyDictionaryName {
		return
	}
	prefs.SetString(dictMetaSourceKey(name), "")
	prefs.SetString(dictMetaTargetKey(name), "")
}

func (a *focstApp) listDictionaryEntries(parent fyne.Window) []dictionaryEntry {
	home, _ := os.UserHomeDir()
	namesDir := filepath.Join(home, ".focst", "names")
	if err := os.MkdirAll(namesDir, 0700); err != nil {
		a.reportDictListError(parent, fmt.Errorf("failed to create dictionary directory: %w", err))
		return []dictionaryEntry{{Name: emptyDictionaryName}}
	}
	files, err := os.ReadDir(namesDir)
	if err != nil {
		a.reportDictListError(parent, fmt.Errorf("failed to read dictionary directory: %w", err))
		return []dictionaryEntry{{Name: emptyDictionaryName}}
	}

	entries := []dictionaryEntry{{Name: emptyDictionaryName}}
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") {
			name := strings.TrimSuffix(f.Name(), ".json")
			src, tgt := a.getDictionaryMeta(name)
			entries = append(entries, dictionaryEntry{
				Name:       name,
				SourceLang: src,
				TargetLang: tgt,
			})
		}
	}
	if len(entries) > 2 {
		sort.Slice(entries[1:], func(i, j int) bool {
			return entries[i+1].Name < entries[j+1].Name
		})
	}
	return entries
}

func (a *focstApp) filterDictionaryEntries(entries []dictionaryEntry, showAll bool) []dictionaryEntry {
	if showAll {
		return entries
	}
	filtered := make([]dictionaryEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Name == emptyDictionaryName {
			filtered = append(filtered, entry)
			continue
		}
		if entry.SourceLang == "" || entry.TargetLang == "" {
			continue
		}
		if entry.SourceLang == a.config.SourceLang && entry.TargetLang == a.config.TargetLang {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func (a *focstApp) reportDictListError(parent fyne.Window, err error) {
	if err == nil || a == nil || parent == nil {
		return
	}
	logger.Error("Dictionary list error", "error", err)
	a.dictErrOnce.Do(func() {
		a.safeDo("dict.list_error_dialog", func() {
			dialog.ShowError(err, parent)
		})
	})
}

func (a *focstApp) loadDictionary(name string) (map[string]string, error) {
	if name == "" || name == emptyDictionaryName {
		return make(map[string]string), nil
	}
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".focst", "names", name+".json")
	sourceCode, targetCode := a.getDictionaryMeta(name)
	if sourceCode == "" {
		sourceCode = a.config.SourceLang
	}
	if targetCode == "" {
		targetCode = a.config.TargetLang
	}
	mapping, err := names.LoadMappingFile(path, sourceCode, targetCode)
	if err != nil {
		err = fmt.Errorf("failed to load dictionary %q (%s -> %s): %w", name, sourceCode, targetCode, err)
		logger.Error("Failed to load dictionary", "path", path, "source", sourceCode, "target", targetCode, "error", err)
		return nil, err
	}
	return mapping, nil
}

func toCharacterMappings(mapping map[string]string) []names.CharacterMapping {
	keys := make([]string, 0, len(mapping))
	for k := range mapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]names.CharacterMapping, 0, len(keys))
	for _, k := range keys {
		out = append(out, names.CharacterMapping{
			Source: k,
			Target: mapping[k],
		})
	}
	return out
}

func (a *focstApp) saveDictionary(savePath string) error {
	data, err := names.EncodeMappings(toCharacterMappings(a.config.NamesMapping), a.config.SourceLang, a.config.TargetLang)
	if err != nil {
		return err
	}
	if err := files.RejectSymlinkPath(savePath); err != nil {
		return err
	}
	return files.AtomicWrite(savePath, data, 0600)
}

func (a *focstApp) deleteDictionary(name string) error {
	if name == "" || name == emptyDictionaryName {
		return nil
	}
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".focst", "names", name+".json")
	return os.Remove(path)
}
