package providers

import "context"

// Translator translates text between languages. src may be LangAuto.
type Translator interface {
	Name() string
	Supports(src, target Language) bool
	Translate(ctx context.Context, text string, src, target Language) (string, error)
}

// SupportedPair reports whether src and target are distinct languages the
// app supports. src may be LangAuto (detect the source).
func SupportedPair(src, target Language) bool {
	return src != target && target.IsSupported() && (src == LangAuto || src.IsSupported())
}
