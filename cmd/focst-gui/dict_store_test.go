package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/oukeidos/focst/internal/names"
)

func TestSaveDictionaryWritesFile(t *testing.T) {
	tmp := t.TempDir()
	app := &focstApp{
		config: AppConfig{
			SourceLang:   "ja",
			TargetLang:   "ko",
			NamesMapping: map[string]string{"a": "b"},
		},
	}
	savePath := filepath.Join(tmp, "names.json")
	if err := app.saveDictionary(savePath); err != nil {
		t.Fatalf("saveDictionary failed: %v", err)
	}
	data, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !strings.Contains(string(data), `"ja"`) || !strings.Contains(string(data), `"ko"`) {
		t.Fatalf("saved file is not the common names schema: %s", string(data))
	}
	decoded, err := names.DecodeMappings(data, "ja", "ko")
	if err != nil {
		t.Fatalf("saved file could not be decoded by common loader: %v", err)
	}
	if len(decoded) != 1 || decoded[0].Source != "a" || decoded[0].Target != "b" {
		t.Fatalf("decoded mappings mismatch: %+v", decoded)
	}
}

func TestSaveDictionaryRejectsSymlink(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target.txt")
	if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(tmp, "names.json")
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink not permitted on Windows: %v", err)
		}
		t.Fatalf("symlink: %v", err)
	}

	app := &focstApp{
		config: AppConfig{
			SourceLang:   "ja",
			TargetLang:   "ko",
			NamesMapping: map[string]string{"a": "b"},
		},
	}
	if err := app.saveDictionary(link); err == nil {
		t.Fatalf("expected error on symlink, got nil")
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "original" {
		t.Fatalf("target modified via symlink: %s", string(data))
	}
}

func TestFilterDictionaryEntries(t *testing.T) {
	app := &focstApp{
		config: AppConfig{
			SourceLang: "ja",
			TargetLang: "ko",
		},
	}
	entries := []dictionaryEntry{
		{Name: emptyDictionaryName},
		{Name: "ja_ko_ok", SourceLang: "ja", TargetLang: "ko"},
		{Name: "ko_ja_other", SourceLang: "ko", TargetLang: "ja"},
		{Name: "unknown"},
	}

	filtered := app.filterDictionaryEntries(entries, false)
	if len(filtered) != 2 {
		t.Fatalf("filtered count = %d, want 2", len(filtered))
	}
	if filtered[0].Name != emptyDictionaryName {
		t.Fatalf("first filtered entry = %q, want %q", filtered[0].Name, emptyDictionaryName)
	}
	if filtered[1].Name != "ja_ko_ok" {
		t.Fatalf("second filtered entry = %q, want %q", filtered[1].Name, "ja_ko_ok")
	}

	all := app.filterDictionaryEntries(entries, true)
	if len(all) != len(entries) {
		t.Fatalf("show-all count = %d, want %d", len(all), len(entries))
	}
}

func TestLoadDictionary_InvalidFileReturnsError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dictDir := filepath.Join(tmp, ".focst", "names")
	if err := os.MkdirAll(dictDir, 0700); err != nil {
		t.Fatalf("mkdir names dir: %v", err)
	}
	path := filepath.Join(dictDir, "broken.json")
	if err := os.WriteFile(path, []byte(`{"not":"supported"}`), 0600); err != nil {
		t.Fatalf("write broken dictionary: %v", err)
	}

	app := &focstApp{
		config: AppConfig{
			SourceLang: "ja",
			TargetLang: "ko",
		},
	}
	got, err := app.loadDictionary("broken")
	if err == nil {
		t.Fatalf("expected error for invalid dictionary schema")
	}
	if got != nil {
		t.Fatalf("expected nil mapping on error, got: %#v", got)
	}
}

func TestLoadDictionary_ValidFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	dictDir := filepath.Join(tmp, ".focst", "names")
	if err := os.MkdirAll(dictDir, 0700); err != nil {
		t.Fatalf("mkdir names dir: %v", err)
	}
	path := filepath.Join(dictDir, "ok.json")
	data, err := names.EncodeMappings([]names.CharacterMapping{{Source: "田中", Target: "타나카"}}, "ja", "ko")
	if err != nil {
		t.Fatalf("encode mappings: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write dictionary: %v", err)
	}

	app := &focstApp{
		config: AppConfig{
			SourceLang: "ja",
			TargetLang: "ko",
		},
	}
	got, err := app.loadDictionary("ok")
	if err != nil {
		t.Fatalf("loadDictionary failed: %v", err)
	}
	if got["田中"] != "타나카" {
		t.Fatalf("loaded mapping mismatch: %#v", got)
	}
}
