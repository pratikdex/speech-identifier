package providers

import (
	"errors"
	"strings"
)

// Provider represents a speech-to-text or translation provider.
type Provider string

const (
	ProviderGoogle   Provider = "google"
	ProviderGoogleV2 Provider = "google-v2"
	ProviderSarvam   Provider = "sarvam"
)

// String returns the string representation of the provider.
func (p Provider) String() string {
	return string(p)
}

// ErrInvalidProvider indicates an unrecognized provider name.
var ErrInvalidProvider = errors.New("invalid provider")

// ParseProvider converts a string into a known Provider enum.
func ParseProvider(s string) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "google":
		return ProviderGoogle, nil
	case "google-v2", "google_v2", "googlev2":
		return ProviderGoogleV2, nil
	case "sarvam":
		return ProviderSarvam, nil
	default:
		return "", ErrInvalidProvider
	}
}
