package google

import (
	"context"
	"fmt"

	gtranslate "cloud.google.com/go/translate"
	"golang.org/x/text/language"

	"dictation/internal/providers"
)

// translateClient is a local interface wrapping the method we need from
// *translate.Client, to enable easier testing.
type translateClient interface {
	Translate(ctx context.Context, inputs []string, target language.Tag, opts *gtranslate.Options) ([]gtranslate.Translation, error)
}

// Translator adapts Google's Translate API (v2) to providers.Translator.
type Translator struct {
	client translateClient
}

var _ providers.Translator = (*Translator)(nil)

// NewTranslator returns a Google translate adapter. c is typically a
// *translate.Client.
func NewTranslator(c translateClient) *Translator {
	return &Translator{client: c}
}

// Name returns "google".
func (t *Translator) Name() string {
	return providers.ProviderGoogle.String()
}

// Supports reports whether the pair is supported.
func (t *Translator) Supports(src, target providers.Language) bool {
	return providers.SupportedPair(src, target)
}

// Translate translates text from src to target using Google Translate. The
// v2 API expects bare language codes, so the region is dropped. An auto src
// leaves Options.Source unset, so the API detects the source language.
func (t *Translator) Translate(ctx context.Context, text string, src, target providers.Language) (string, error) {
	opts := &gtranslate.Options{Format: gtranslate.Text}
	if src != providers.LangAuto {
		opts.Source = language.Make(src.Base())
	}
	ts, err := t.client.Translate(ctx, []string{text}, language.Make(target.Base()), opts)
	if err != nil {
		return "", fmt.Errorf("google: translating: %w", err)
	}
	if len(ts) == 0 {
		return "", fmt.Errorf("google: translate: empty result")
	}
	return ts[0].Text, nil
}
