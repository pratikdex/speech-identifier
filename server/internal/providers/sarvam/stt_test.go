package sarvam

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dictation/internal/providers"
)

func TestSTT_StreamsAudioAndReadsTranscripts(t *testing.T) {
	flushGrace = 50 * time.Millisecond
	got := make(chan map[string]any, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/speech-to-text/ws", r.URL.Path)
		assert.Equal(t, "unknown", r.URL.Query().Get("language-code"))
		assert.Equal(t, "codemix", r.URL.Query().Get("mode"))
		assert.Equal(t, "key", r.Header.Get("Api-Subscription-Key"))
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()
		for i := 0; i < 2; i++ { // audio, then flush
			var msg map[string]any
			if conn.ReadJSON(&msg) != nil {
				return
			}
			got <- msg
		}
		_ = conn.WriteJSON(map[string]any{"type": "events", "data": map[string]any{"signal_type": "END_SPEECH"}})
		_ = conn.WriteJSON(map[string]any{"type": "data", "data": map[string]any{"transcript": "kal ki hearing", "language_code": "hi-IN"}})
		time.Sleep(time.Second)
	}))
	defer srv.Close()

	stt := NewSTT(Options{APIKey: "key", WSURL: "ws" + strings.TrimPrefix(srv.URL, "http")})
	stream, err := stt.Open(context.Background(), providers.STTConfig{SampleRate: 16000})
	require.NoError(t, err)

	require.NoError(t, stream.Write([]byte{1, 2}))
	audioMsg := <-got
	data := audioMsg["audio"].(map[string]any)["data"].(string)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte{1, 2}), data)

	require.NoError(t, stream.Close())
	assert.Equal(t, "flush", (<-got)["type"])

	tr, err := stream.Next()
	require.NoError(t, err)
	assert.Equal(t, providers.Transcript{Text: "kal ki hearing", Language: "hi-IN", Final: true, Provider: "sarvam"}, tr)

	_, err = stream.Next()
	assert.True(t, errors.Is(err, io.EOF), "want EOF after close, got %v", err)
}
