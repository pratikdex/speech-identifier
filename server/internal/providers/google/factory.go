package google

import (
	"context"
	"fmt"
	"log"
	"os"

	speech "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/translate"

	"dictation/internal/providers"
)

// Factory returns a providers.Factory for Google: speech-to-text v1
// (languages[0] is the primary, the rest alternatives), v2 (Chirp) and
// Translate. It is disabled unless GOOGLE_APPLICATION_CREDENTIALS is set.
func Factory(languages []string) providers.Factory {
	return func(ctx context.Context) ([]any, error) {
		if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
			log.Println("google disabled: GOOGLE_APPLICATION_CREDENTIALS not set")
			return nil, nil
		}
		speechClient, err := speech.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("google speech client: %w", err)
		}
		v1, err := NewSTT(speechClient, languages)
		if err != nil {
			return nil, err
		}
		out := []any{v1}

		v2Opts := V2OptionsFromEnv()
		if v2Client, err := NewV2Client(ctx, v2Opts); err != nil {
			log.Printf("google v2 client warning: %v", err) // v1 still works
		} else {
			out = append(out, NewV2STT(v2Client, v2Opts))
		}

		translateClient, err := translate.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("google translate client: %w", err)
		}
		return append(out, NewTranslator(translateClient)), nil
	}
}
