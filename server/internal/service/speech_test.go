package service

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"dictation/internal/providers"
	"dictation/internal/providers/mocks"
)

func stt(t *testing.T, name string, stream providers.TranscriptStream, openErr error) *mocks.MockSpeechToText {
	m := mocks.NewMockSpeechToText(t)
	m.EXPECT().Name().Return(name).Maybe()
	m.EXPECT().Open(mock.Anything, mock.Anything).Return(stream, openErr).Maybe()
	return m
}

func streamOf(t *testing.T, ts ...providers.Transcript) *mocks.MockTranscriptStream {
	m := mocks.NewMockTranscriptStream(t)
	for _, tr := range ts {
		m.EXPECT().Next().Return(tr, nil).Once()
	}
	m.EXPECT().Next().Return(providers.Transcript{}, io.EOF).Maybe()
	return m
}

func translator(t *testing.T, name string, out string, err error) *mocks.MockTranslator {
	m := mocks.NewMockTranslator(t)
	m.EXPECT().Name().Return(name).Maybe()
	m.EXPECT().Supports(mock.Anything, mock.Anything).Return(true).Maybe()
	m.EXPECT().Translate(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(out, err).Maybe()
	return m
}

func drain(t *testing.T, s Session) []Utterance {
	var out []Utterance
	for {
		u, err := s.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		require.NoError(t, err)
		out = append(out, u)
	}
}

func TestStart_FallsBackWhenOpenFails(t *testing.T) {
	stream := streamOf(t, providers.Transcript{Text: "ok", Final: true})
	svc := New(Config{STT: []providers.SpeechToText{
		stt(t, "google", nil, errors.New("down")),
		stt(t, "sarvam", stream, nil),
	}})
	sess, err := svc.Start(context.Background(), SessionOptions{SampleRate: 16000})
	require.NoError(t, err)
	assert.Equal(t, "ok", drain(t, sess)[0].Text)
}

func TestStart_NoProviderIsUnavailable(t *testing.T) {
	_, err := New(Config{}).Start(context.Background(), SessionOptions{})
	assert.ErrorIs(t, err, ErrProviderUnavailable)
}

func TestStart_TranslateWithoutTranslatorsUnsupported(t *testing.T) {
	_, err := New(Config{}).Start(context.Background(), SessionOptions{Mode: ModeTranslate, Target: "en-IN"})
	assert.ErrorIs(t, err, ErrUnsupportedLanguage)
}

func TestSession_IDsAndEmptyFinals(t *testing.T) {
	stream := streamOf(t,
		providers.Transcript{Text: "kal ki"},
		providers.Transcript{Text: "kal ki hearing", Final: true},
		providers.Transcript{Text: "  ", Final: true},
		providers.Transcript{Text: "aur", Final: true},
	)
	sess, err := New(Config{STT: []providers.SpeechToText{stt(t, "google", stream, nil)}}).
		Start(context.Background(), SessionOptions{})
	require.NoError(t, err)
	us := drain(t, sess)
	require.Len(t, us, 3)
	assert.Equal(t, []string{"u1", "u1", "u2"}, []string{us[0].ID, us[1].ID, us[2].ID})
}

func TestTranslate_SameLanguageSkipsCall(t *testing.T) {
	tr := mocks.NewMockTranslator(t) // no expectations: any call fails the test
	out, err := translatorChain{tr}.translate(context.Background(), "hello", "en-IN", "en-IN")
	require.NoError(t, err)
	assert.Equal(t, "hello", out)
}

func TestTranslate_FallsBackOnErrorOrBlank(t *testing.T) {
	svc := translatorChain{
		translator(t, "google", "", errors.New("down")),
		translator(t, "x", "  ", nil),
		translator(t, "sarvam", "Keep the documents ready", nil),
	}
	out, err := svc.translate(context.Background(), "documents ready rakhna", "hi-IN", "en-IN")
	require.NoError(t, err)
	assert.Equal(t, "Keep the documents ready", out)
}

func TestTranslate_AllFailIsUnavailable(t *testing.T) {
	svc := translatorChain{translator(t, "google", "", errors.New("down"))}
	_, err := svc.translate(context.Background(), "x", "hi-IN", "en-IN")
	assert.ErrorIs(t, err, ErrProviderUnavailable)
}

func TestSession_TranslateFailureKeepsSessionAlive(t *testing.T) {
	stream := streamOf(t,
		providers.Transcript{Text: "namaste", Language: "hi-IN", Final: true},
		providers.Transcript{Text: "phir se", Language: "hi-IN", Final: true},
	)
	svc := New(Config{
		STT:         []providers.SpeechToText{stt(t, "google", stream, nil)},
		Translators: []providers.Translator{translator(t, "google", "", errors.New("down"))},
	})
	sess, err := svc.Start(context.Background(), SessionOptions{Mode: ModeTranslate, Target: "en-IN"})
	require.NoError(t, err)
	us := drain(t, sess)
	require.Len(t, us, 2)
	assert.Equal(t, "namaste", us[0].Text)
	assert.Empty(t, us[0].Translation)
}
