package speedtest

import "strings"

func NormalizeLanguage(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "en":
		return "en"
	default:
		return "cn"
	}
}

func Text(lang, en, zh string) string {
	if NormalizeLanguage(lang) == "en" {
		return en
	}
	return zh
}
