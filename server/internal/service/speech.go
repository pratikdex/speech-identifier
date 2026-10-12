// Package service is the Speech facade: it opens a transcription session on
// the first working STT provider and, in Translate mode, translates each
// final utterance with the first working translator.
package service

import (
	"context"
	"errors"
	"fmt"

	"dictation/internal/providers"
)

var (
	ErrUnsupportedLanguage = errors.New("unsupported language")
	ErrProviderUnavailable = errors.New("provider unavailable")
	ErrConnectionLost      = errors.New("connection lost")
)

// SessionOptions configures a streaming session.
type SessionOptions struct {
	SampleRate int
	Mode       Mode
	Target     providers.Language // required when Mode == ModeTranslate
}

// Utterance is one transcription result. Interim and final results for the
// same speech share an ID; a final one replaces the interim text.
type Utterance struct {
	ID          string
	Text        string
	Language    string // detected
	Final       bool
	Translation string // set on finals in Translate mode
	Confidence  float32
}

// Session is a streaming transcription session.
type Session interface {
	Write(pcm []byte) error
	Next() (Utterance, error) // io.EOF at end
	Close() error
}

// Speech is the facade the WebSocket layer depends on.
type Speech interface {
	Start(ctx context.Context, opts SessionOptions) (Session, error)
}

// Config lists providers in priority order.
type Config struct {
	STT         []providers.SpeechToText
	Translators []providers.Translator
}

// Service implements Speech.
type Service struct{ cfg Config }

func New(cfg Config) *Service { return &Service{cfg: cfg} }

// Start opens a session on the first STT provider that opens successfully.
func (s *Service) Start(ctx context.Context, opts SessionOptions) (Session, error) {
	if opts.Mode == ModeTranslate && len(s.cfg.Translators) == 0 {
		return nil, ErrUnsupportedLanguage
	}
	lastErr := errors.New("no STT provider configured")
	for _, stt := range s.cfg.STT {
		stream, err := stt.Open(ctx, providers.STTConfig{SampleRate: opts.SampleRate})
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", stt.Name(), err)
			continue
		}
		sess := &session{stream: stream}
		if opts.Mode == ModeTranslate {
			sess.ctx, sess.translators, sess.target = ctx, translatorChain(s.cfg.Translators), opts.Target
		}
		return sess, nil
	}
	return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, lastErr)
}
