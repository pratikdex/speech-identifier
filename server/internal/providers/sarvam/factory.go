package sarvam

import (
	"context"
	"log"
	"os"
)

// Factory is the providers.Factory for Sarvam STT and translate. It is
// disabled unless SARVAM_API_KEY is set.
func Factory(context.Context) ([]any, error) {
	key := os.Getenv("SARVAM_API_KEY")
	if key == "" {
		log.Println("sarvam disabled: SARVAM_API_KEY not set")
		return nil, nil
	}
	o := Options{APIKey: key}
	return []any{NewSTT(o), NewTranslator(o)}, nil
}
