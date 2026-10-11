package providers

import (
	"errors"
	"strings"
)

// Language represents a supported language identified by its canonical BCP-47 tag with the "-IN" region.
type Language string

const (
	LangEnglish   Language = "en-IN"
	LangHindi     Language = "hi-IN"
	LangMalayalam Language = "ml-IN"
	LangKannada   Language = "kn-IN"
	LangTelugu    Language = "te-IN"
	LangMarathi   Language = "mr-IN"
	LangAuto      Language = "auto"
)

// String returns the string representation of the language tag.
func (l Language) String() string {
	return string(l)
}

// BCP47 returns the canonical BCP-47 tag (e.g. "hi-IN").
func (l Language) BCP47() string {
	return string(l)
}

// Base returns the 2-letter base language code without region (e.g. "hi-IN" -> "hi").
func (l Language) Base() string {
	base, _, _ := strings.Cut(string(l), "-")
	return strings.ToLower(base)
}

// Name returns the human-readable English name of the language.
func (l Language) Name() string {
	switch l {
	case LangEnglish:
		return "English"
	case LangHindi:
		return "Hindi"
	case LangMalayalam:
		return "Malayalam"
	case LangKannada:
		return "Kannada"
	case LangTelugu:
		return "Telugu"
	case LangMarathi:
		return "Marathi"
	case LangAuto:
		return "Auto"
	default:
		return string(l)
	}
}

// IsSupported reports whether the language is one of the supported Indian languages.
func (l Language) IsSupported() bool {
	switch l {
	case LangEnglish, LangHindi, LangMalayalam, LangKannada, LangTelugu, LangMarathi:
		return true
	default:
		return false
	}
}

// ErrInvalidLanguage indicates that a language string could not be recognized.
var ErrInvalidLanguage = errors.New("invalid language")

// ParseLanguage parses an input string (e.g. "hi", "hi-IN", "hin", "en-US", "auto")
// into its canonical Language enum.
func ParseLanguage(s string) (Language, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "auto") {
		return LangAuto, nil
	}
	base, _, _ := strings.Cut(s, "-")
	switch strings.ToLower(base) {
	case "en", "eng":
		return LangEnglish, nil
	case "hi", "hin":
		return LangHindi, nil
	case "ml", "mal":
		return LangMalayalam, nil
	case "kn", "kan":
		return LangKannada, nil
	case "te", "tel":
		return LangTelugu, nil
	case "mr", "mar":
		return LangMarathi, nil
	default:
		return "", ErrInvalidLanguage
	}
}
