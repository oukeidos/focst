package language

import (
	"sort"
)

// Language represents a supported language with its configuration.
type Language struct {
	Code       string
	Name       string
	DefaultCPL int // Characters Per Line
	DefaultCPS int // Characters Per Second
}

// Default settings as requested
const (
	DefaultCPL = 42
	DefaultCPS = 17
)

// Languages is a map of supported languages code -> Language.
var Languages = map[string]Language{
	"af":       {Code: "af", Name: "Afrikaans", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"sq":       {Code: "sq", Name: "Albanian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"am":       {Code: "am", Name: "Amharic", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},  // fallback
	"ar":       {Code: "ar", Name: "Arabic", DefaultCPL: DefaultCPL, DefaultCPS: 20},
	"hy":       {Code: "hy", Name: "Armenian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},    // fallback
	"as":       {Code: "as", Name: "Assamese", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},    // fallback
	"az":       {Code: "az", Name: "Azerbaijani", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"eu":       {Code: "eu", Name: "Basque", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"be":       {Code: "be", Name: "Belarusian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"bn":       {Code: "bn", Name: "Bengali", DefaultCPL: DefaultCPL, DefaultCPS: 22},
	"bs":       {Code: "bs", Name: "Bosnian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"bg":       {Code: "bg", Name: "Bulgarian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"ca":       {Code: "ca", Name: "Catalan", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"ceb":      {Code: "ceb", Name: "Cebuano", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},  // fallback
	"zh":       {Code: "zh-Hans", Name: "Chinese (Simplified)", DefaultCPL: 16, DefaultCPS: 11}, // Default to Simplified
	"zh-Hans":  {Code: "zh-Hans", Name: "Chinese (Simplified)", DefaultCPL: 16, DefaultCPS: 11},
	"zh-Hant":  {Code: "zh-Hant", Name: "Chinese (Traditional)", DefaultCPL: 16, DefaultCPS: 11},
	"co":       {Code: "co", Name: "Corsican", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"hr":       {Code: "hr", Name: "Croatian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"cs":       {Code: "cs", Name: "Czech", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"da":       {Code: "da", Name: "Danish", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"dv":       {Code: "dv", Name: "Dhivehi", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"nl":       {Code: "nl", Name: "Dutch", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"en":       {Code: "en", Name: "English", DefaultCPL: DefaultCPL, DefaultCPS: 20},
	"eo":       {Code: "eo", Name: "Esperanto", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"et":       {Code: "et", Name: "Estonian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},  // fallback
	"fil":      {Code: "fil", Name: "Filipino", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"fi":       {Code: "fi", Name: "Finnish", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"fr":       {Code: "fr", Name: "French", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"fy":       {Code: "fy", Name: "Frisian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"gl":       {Code: "gl", Name: "Galician", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"ka":       {Code: "ka", Name: "Georgian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"de":       {Code: "de", Name: "German", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"el":       {Code: "el", Name: "Greek", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"gu":       {Code: "gu", Name: "Gujarati", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},       // fallback
	"ht":       {Code: "ht", Name: "Haitian Creole", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"ha":       {Code: "ha", Name: "Hausa", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},          // fallback
	"haw":      {Code: "haw", Name: "Hawaiian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},      // fallback
	"iw":       {Code: "iw", Name: "Hebrew", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"hi":       {Code: "hi", Name: "Hindi", DefaultCPL: DefaultCPL, DefaultCPS: 22},
	"hmn":      {Code: "hmn", Name: "Hmong", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"hu":       {Code: "hu", Name: "Hungarian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"is":       {Code: "is", Name: "Icelandic", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"ig":       {Code: "ig", Name: "Igbo", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"id":       {Code: "id", Name: "Indonesian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"ga":       {Code: "ga", Name: "Irish", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"it":       {Code: "it", Name: "Italian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"ja":       {Code: "ja", Name: "Japanese", DefaultCPL: 13, DefaultCPS: 4},                  // fallback (CPS)
	"jv":       {Code: "jv", Name: "Javanese", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"kn":       {Code: "kn", Name: "Kannada", DefaultCPL: DefaultCPL, DefaultCPS: 22},
	"kk":       {Code: "kk", Name: "Kazakh", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"km":       {Code: "km", Name: "Khmer", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},  // fallback
	"ko":       {Code: "ko", Name: "Korean", DefaultCPL: 16, DefaultCPS: 12},
	"kri":      {Code: "kri", Name: "Krio", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},         // fallback
	"ku":       {Code: "ku", Name: "Kurdish", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},       // fallback
	"ky":       {Code: "ky", Name: "Kyrgyz", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},        // fallback
	"lo":       {Code: "lo", Name: "Lao", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},           // fallback
	"la":       {Code: "la", Name: "Latin", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},         // fallback
	"lv":       {Code: "lv", Name: "Latvian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},       // fallback
	"lt":       {Code: "lt", Name: "Lithuanian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},    // fallback
	"lb":       {Code: "lb", Name: "Luxembourgish", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"mk":       {Code: "mk", Name: "Macedonian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},    // fallback
	"mg":       {Code: "mg", Name: "Malagasy", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},      // fallback
	"ms":       {Code: "ms", Name: "Malay", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"ml":       {Code: "ml", Name: "Malayalam", DefaultCPL: DefaultCPL, DefaultCPS: 22},
	"mt":       {Code: "mt", Name: "Maltese", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"mi":       {Code: "mi", Name: "Maori", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},   // fallback
	"mr":       {Code: "mr", Name: "Marathi", DefaultCPL: DefaultCPL, DefaultCPS: 22},
	"mni-Mtei": {Code: "mni-Mtei", Name: "Meiteilon (Manipuri)", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"mn":       {Code: "mn", Name: "Mongolian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},                  // fallback
	"my":       {Code: "my", Name: "Myanmar (Burmese)", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},          // fallback
	"ne":       {Code: "ne", Name: "Nepali", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},                     // fallback
	"no":       {Code: "no", Name: "Norwegian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"ny":       {Code: "ny", Name: "Nyanja (Chichewa)", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"or":       {Code: "or", Name: "Odia (Oriya)", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},      // fallback
	"ps":       {Code: "ps", Name: "Pashto", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},            // fallback
	"fa":       {Code: "fa", Name: "Persian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},           // fallback
	"pl":       {Code: "pl", Name: "Polish", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"pt":       {Code: "pt", Name: "Portuguese", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"pa":       {Code: "pa", Name: "Punjabi", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"ro":       {Code: "ro", Name: "Romanian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"ru":       {Code: "ru", Name: "Russian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"sm":       {Code: "sm", Name: "Samoan", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},       // fallback
	"gd":       {Code: "gd", Name: "Scots Gaelic", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"sr":       {Code: "sr", Name: "Serbian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"st":       {Code: "st", Name: "Sesotho", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},             // fallback
	"sn":       {Code: "sn", Name: "Shona", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},               // fallback
	"sd":       {Code: "sd", Name: "Sindhi", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},              // fallback
	"si":       {Code: "si", Name: "Sinhala (Sinhalese)", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"sk":       {Code: "sk", Name: "Slovak", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"sl":       {Code: "sl", Name: "Slovenian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"so":       {Code: "so", Name: "Somali", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},    // fallback
	"es":       {Code: "es", Name: "Spanish", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"su":       {Code: "su", Name: "Sundanese", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"sw":       {Code: "sw", Name: "Swahili", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},   // fallback
	"sv":       {Code: "sv", Name: "Swedish", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"tg":       {Code: "tg", Name: "Tajik", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"ta":       {Code: "ta", Name: "Tamil", DefaultCPL: DefaultCPL, DefaultCPS: 22},
	"te":       {Code: "te", Name: "Telugu", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"th":       {Code: "th", Name: "Thai", DefaultCPL: 35, DefaultCPS: DefaultCPS},
	"tr":       {Code: "tr", Name: "Turkish", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"uk":       {Code: "uk", Name: "Ukrainian", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"ur":       {Code: "ur", Name: "Urdu", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},   // fallback
	"ug":       {Code: "ug", Name: "Uyghur", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"uz":       {Code: "uz", Name: "Uzbek", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},  // fallback
	"vi":       {Code: "vi", Name: "Vietnamese", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"cy":       {Code: "cy", Name: "Welsh", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
	"xh":       {Code: "xh", Name: "Xhosa", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},   // fallback
	"yi":       {Code: "yi", Name: "Yiddish", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS}, // fallback
	"yo":       {Code: "yo", Name: "Yoruba", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},  // fallback
	"zu":       {Code: "zu", Name: "Zulu", DefaultCPL: DefaultCPL, DefaultCPS: DefaultCPS},
}

// GetLanguageCode returns strict matching code or empty if not found.
func GetLanguage(code string) (Language, bool) {
	lang, ok := Languages[code]
	return lang, ok
}

// LanguageEntry represents a map entry for listing.
type LanguageEntry struct {
	ID string // The map key (CLI flag)
	Language
}

// GetSupportedLanguages returns a list of supported languages sorted by Name and then ID.
func GetSupportedLanguages() []LanguageEntry {
	entries := make([]LanguageEntry, 0, len(Languages))
	for k, v := range Languages {
		entries = append(entries, LanguageEntry{ID: k, Language: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].ID < entries[j].ID
	})
	return entries
}
