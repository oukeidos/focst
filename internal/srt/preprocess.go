package srt

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	parenRegex = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]|（[^）]*）|［[^］]*］`)
)

// IDMap tracks mapping from internal IDs to original IDs.
type IDMap struct {
	InternalID int `json:"internal_id"`
	OriginalID int `json:"original_id"`
}

// Preprocess performs cleaning and filtering on the provided segments.
// It removes text within (), [], （）, and ［］, filters out segments with only
// symbols, and re-indexes the remaining segments.
// Bracket removal and meaningless filtering are restricted to Japanese ("ja").
func Preprocess(segments []Segment, sourceLangCode string) []Segment {
	cleaned, _ := PreprocessWithMappingOptions(segments, sourceLangCode, true)
	return cleaned
}

// PreprocessWithOptions performs preprocessing with optional language-specific rules.
func PreprocessWithOptions(segments []Segment, sourceLangCode string, applyLangRules bool) []Segment {
	cleaned, _ := PreprocessWithMappingOptions(segments, sourceLangCode, applyLangRules)
	return cleaned
}

// PreprocessWithMapping performs preprocessing and returns ID mappings.
func PreprocessWithMapping(segments []Segment, sourceLangCode string) ([]Segment, []IDMap) {
	return PreprocessWithMappingOptions(segments, sourceLangCode, true)
}

// PreprocessWithMappingOptions performs preprocessing and returns ID mappings.
// Language-specific rules can be disabled with applyLangRules=false.
func PreprocessWithMappingOptions(segments []Segment, sourceLangCode string, applyLangRules bool) ([]Segment, []IDMap) {
	var cleaned []Segment
	var originalIDs []int

	for _, seg := range segments {
		newLines := make([]string, 0, len(seg.Lines))
		for _, line := range seg.Lines {
			cleanedLine := line
			if applyLangRules && sourceLangCode == "ja" {
				cleanedLine = parenRegex.ReplaceAllString(line, "")
				cleanedLine = strings.ReplaceAll(cleanedLine, "<", "")
				cleanedLine = strings.ReplaceAll(cleanedLine, ">", "")
			}
			cleanedLine = strings.TrimSpace(cleanedLine)
			if cleanedLine != "" {
				newLines = append(newLines, cleanedLine)
			}
		}

		// If no lines left after bracket removal, skip the segment
		if len(newLines) == 0 {
			continue
		}

		// Check if the remaining text is just symbols/punctuation (Japanese only)
		if applyLangRules && sourceLangCode == "ja" && isMeaningless(newLines) {
			continue
		}

		originalIDs = append(originalIDs, seg.ID)
		seg.Lines = newLines
		cleaned = append(cleaned, seg)
	}

	// Re-reindex
	for i := range cleaned {
		cleaned[i].ID = i + 1
	}

	mapping := make([]IDMap, 0, len(cleaned))
	for i := range cleaned {
		mapping = append(mapping, IDMap{
			InternalID: cleaned[i].ID,
			OriginalID: originalIDs[i],
		})
	}

	return cleaned, mapping
}

func isMeaningless(lines []string) bool {
	for _, line := range lines {
		for _, r := range line {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				return false
			}
		}
	}
	return true
}
