package sarvam

import (
	"context"
	"fmt"
	"strings"

	"dictation/internal/providers"
)

const providerName = string(providers.ProviderSarvam)

// translateModel is Sarvam's Mayura translation model.
const translateModel = "mayura:v1"

// Translator adapts Sarvam's /translate endpoint to providers.Translator.
type Translator struct {
	opts Options
}

var _ providers.Translator = (*Translator)(nil)

// NewTranslator returns a Sarvam translate adapter.
func NewTranslator(o Options) *Translator {
	return &Translator{opts: o.withDefaults()}
}

// Name returns "sarvam".
func (t *Translator) Name() string {
	return providerName
}

// Supports reports whether the pair is supported.
func (t *Translator) Supports(src, target providers.Language) bool {
	return providers.SupportedPair(src, target)
}

type translateRequest struct {
	Input              string `json:"input"`
	SourceLanguageCode string `json:"source_language_code"`
	TargetLanguageCode string `json:"target_language_code"`
	Model              string `json:"model"`
	Mode               string `json:"mode"`
}

type translateResponse struct {
	TranslatedText     string `json:"translated_text"`
	SourceLanguageCode string `json:"source_language_code"`
}

// Translate translates text from src to target using Sarvam's Mayura model.
func (t *Translator) Translate(ctx context.Context, text string, src, target providers.Language) (string, error) {
	req := translateRequest{
		Input:              text,
		SourceLanguageCode: src.BCP47(),
		TargetLanguageCode: target.BCP47(),
		Model:              translateModel,
		Mode:               "formal",
	}
	var resp translateResponse
	if err := postJSON(ctx, t.opts, "/translate", req, &resp); err != nil {
		return "", err
	}
	if strings.TrimSpace(resp.TranslatedText) == "" {
		return "", fmt.Errorf("sarvam /translate: empty translated_text in response")
	}
	return resp.TranslatedText, nil
}
