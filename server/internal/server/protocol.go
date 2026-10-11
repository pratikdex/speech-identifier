package server

import (
	"encoding/json"
	"errors"
	"fmt"

	"dictation/internal/providers"
	"dictation/internal/service"
)

// ErrorCode represents an error code sent to the client.
type ErrorCode string

const (
	ErrorCodeUnsupportedLanguage ErrorCode = "unsupported_language"
	ErrorCodeConnectionLost      ErrorCode = "connection_lost"
	ErrorCodeProviderUnavailable ErrorCode = "provider_unavailable"
	ErrorCodeBadConfig           ErrorCode = "bad_config"
)

// String returns the string representation of the error code.
func (e ErrorCode) String() string {
	return string(e)
}

// clientConfig is the validated first frame of a session.
type clientConfig struct {
	Mode   service.Mode
	Target providers.Language // set only in ModeTranslate
}

// parseConfig validates {"type":"config","mode":"transcribe"|"translate","target":"en|hi|ml|kn|te|mr"}.
// target is required in translate mode and ignored otherwise.
func parseConfig(raw []byte) (clientConfig, error) {
	var msg struct {
		Type   string `json:"type"`
		Mode   string `json:"mode"`
		Target string `json:"target"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return clientConfig{}, fmt.Errorf("config: %w", err)
	}
	if msg.Type != "config" {
		return clientConfig{}, fmt.Errorf("config: unexpected type %q", msg.Type)
	}

	mode, err := service.ParseMode(msg.Mode)
	if err != nil {
		return clientConfig{}, fmt.Errorf("config: %w", err)
	}

	switch mode {
	case service.ModeTranscribe:
		return clientConfig{Mode: service.ModeTranscribe}, nil
	case service.ModeTranslate:
		lang, err := providers.ParseLanguage(msg.Target)
		if err != nil || !lang.IsSupported() {
			return clientConfig{}, fmt.Errorf("config: unknown target %q", msg.Target)
		}
		return clientConfig{Mode: service.ModeTranslate, Target: lang}, nil
	default:
		return clientConfig{}, fmt.Errorf("config: unknown mode %q", msg.Mode)
	}
}

// errorCode maps a service error to the code sent to the client.
func errorCode(err error) string {
	switch {
	case errors.Is(err, service.ErrUnsupportedLanguage):
		return ErrorCodeUnsupportedLanguage.String()
	case errors.Is(err, service.ErrConnectionLost):
		return ErrorCodeConnectionLost.String()
	default:
		return ErrorCodeProviderUnavailable.String()
	}
}

// Server → client events.

type utteranceEvent struct {
	Type        string `json:"type"` // "utterance"
	ID          string `json:"id"`
	Text        string `json:"text"`
	Lang        string `json:"lang"`
	Final       bool   `json:"final"`
	Translation string `json:"translation,omitempty"`
}

type event struct {
	Type string `json:"type"` // "error" or "done"
	Code string `json:"code,omitempty"`
}
