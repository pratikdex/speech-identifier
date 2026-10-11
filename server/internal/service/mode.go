package service

import (
	"fmt"
	"strings"
)

// Mode selects what a session produces.
type Mode int

const (
	ModeTranscribe Mode = iota
	ModeTranslate
)

// String returns the name of the mode ("transcribe" or "translate").
func (m Mode) String() string {
	switch m {
	case ModeTranscribe:
		return "transcribe"
	case ModeTranslate:
		return "translate"
	default:
		return "unknown"
	}
}

// ParseMode parses a mode string into a Mode enum.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "transcribe":
		return ModeTranscribe, nil
	case "translate":
		return ModeTranslate, nil
	default:
		return Mode(0), fmt.Errorf("unknown mode %q", s)
	}
}
