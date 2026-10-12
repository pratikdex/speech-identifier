package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"dictation/internal/providers"
)

// session maps a provider TranscriptStream to utterances.
type session struct {
	stream providers.TranscriptStream
	// Set only in Translate mode.
	ctx         context.Context
	translators translatorChain
	target      providers.Language

	mu     sync.Mutex // serializes Write and Close
	closed bool
	finals int // only touched by Next
}

func (s *session) Write(pcm []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("session closed")
	}
	return s.stream.Write(pcm)
}

// Next returns the next utterance. IDs are "u1", "u2", …, advancing after
// each final; empty finals are skipped.
func (s *session) Next() (Utterance, error) {
	for {
		t, err := s.stream.Next()
		if errors.Is(err, io.EOF) {
			return Utterance{}, io.EOF
		}
		if err != nil {
			return Utterance{}, fmt.Errorf("%w: %v", ErrConnectionLost, err)
		}
		if t.Final && strings.TrimSpace(t.Text) == "" {
			continue
		}
		u := Utterance{ID: fmt.Sprintf("u%d", s.finals+1), Text: t.Text, Language: t.Language, Final: t.Final, Confidence: t.Confidence}
		if t.Final {
			s.finals++
			if s.translators != nil {
				// A failed translation must not end the session; send the text without it.
				src, perr := providers.ParseLanguage(t.Language)
				if perr != nil {
					src = providers.LangAuto
				}
				if u.Translation, err = s.translators.translate(s.ctx, u.Text, src, s.target); err != nil {
					log.Printf("translate: %v", err)
				}
			}
		}
		return u, nil
	}
}

func (s *session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.stream.Close()
}
