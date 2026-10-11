package sarvam

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"dictation/internal/audio"
	"dictation/internal/providers"
)

// flushGrace is how long Close waits for the last transcript after flushing.
var flushGrace = 2 * time.Second

// STT streams audio to Sarvam's Saaras model. It auto-detects the language
// and writes code-mixed speech (e.g. Hinglish) naturally. Results are
// final-only: one transcript per pause.
type STT struct{ o Options }

func NewSTT(o Options) *STT { return &STT{o: o.withDefaults()} }

func (s *STT) Name() string { return providers.ProviderSarvam.String() }

func (s *STT) Open(ctx context.Context, cfg providers.STTConfig) (providers.TranscriptStream, error) {
	q := url.Values{
		"language-code":     {"unknown"}, // auto-detect
		"model":             {"saaras:v4"},
		"mode":              {"codemix"},
		"sample_rate":       {"16000"},
		"input_audio_codec": {"pcm_s16le"},
		"flush_signal":      {"true"},
	}
	header := http.Header{"Api-Subscription-Key": {s.o.APIKey}}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, s.o.WSURL+"/speech-to-text/ws?"+q.Encode(), header)
	if err != nil {
		return nil, fmt.Errorf("sarvam stt: %w", err)
	}
	return &sttStream{conn: conn, sampleRate: cfg.SampleRate}, nil
}

type sttStream struct {
	conn       *websocket.Conn
	sampleRate int
	closing    atomic.Bool
}

type audioMessage struct {
	Audio struct {
		Data       string `json:"data"`
		SampleRate string `json:"sample_rate"`
		Encoding   string `json:"encoding"`
	} `json:"audio"`
}

func (s *sttStream) Write(pcm []byte) error {
	if s.sampleRate != audio.TargetRate {
		var err error
		if pcm, err = audio.Resample(pcm, s.sampleRate, audio.TargetRate); err != nil {
			return err
		}
	}
	var msg audioMessage
	msg.Audio.Data = base64.StdEncoding.EncodeToString(pcm)
	msg.Audio.SampleRate = "16000"
	msg.Audio.Encoding = "audio/wav"
	return s.conn.WriteJSON(msg)
}

func (s *sttStream) Next() (providers.Transcript, error) {
	for {
		var msg struct {
			Type string `json:"type"`
			Data struct {
				Transcript   string `json:"transcript"`
				LanguageCode string `json:"language_code"`
				Error        string `json:"error"`
			} `json:"data"`
		}
		if err := s.conn.ReadJSON(&msg); err != nil {
			if s.closing.Load() || websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return providers.Transcript{}, io.EOF
			}
			return providers.Transcript{}, fmt.Errorf("sarvam stt: %w", err)
		}
		switch msg.Type {
		case "data":
			return providers.Transcript{Text: msg.Data.Transcript, Language: msg.Data.LanguageCode, Final: true, Provider: "sarvam"}, nil
		case "error":
			return providers.Transcript{}, fmt.Errorf("sarvam stt: %s", msg.Data.Error)
		}
		// other message types (VAD events) are ignored
	}
}

// Close flushes buffered audio, then closes the connection after a short
// grace period so Next can still receive the last transcript.
func (s *sttStream) Close() error {
	if s.closing.Swap(true) {
		return nil
	}
	err := s.conn.WriteJSON(map[string]string{"type": "flush"})
	time.AfterFunc(flushGrace, func() { s.conn.Close() })
	return err
}
