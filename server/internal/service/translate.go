package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dictation/internal/providers"
)

const translateTimeout = 10 * time.Second

// translatorChain tries translators in order until one succeeds.
type translatorChain []providers.Translator

// translate returns text in target using the first translator that succeeds.
// Text already in the target language is returned as is.
func (c translatorChain) translate(ctx context.Context, text string, src, target providers.Language) (string, error) {
	if src != providers.LangAuto && src.Base() == target.Base() {
		return text, nil
	}
	err := ErrUnsupportedLanguage
	for _, tr := range c {
		if !tr.Supports(src, target) {
			continue
		}
		callCtx, cancel := context.WithTimeout(ctx, translateTimeout)
		out, terr := tr.Translate(callCtx, text, src, target)
		cancel()
		if terr == nil && strings.TrimSpace(out) != "" {
			return out, nil
		}
		err = fmt.Errorf("%w: %s: %v", ErrProviderUnavailable, tr.Name(), terr)
	}
	return "", err
}
