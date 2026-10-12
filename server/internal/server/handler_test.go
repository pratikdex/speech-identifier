package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dictation/internal/service"
)

// fakeSpeech hands out a fakeSession that emits one final utterance per Write.
type fakeSpeech struct {
	sess     *fakeSession
	startErr error
}

func (f *fakeSpeech) Start(context.Context, service.SessionOptions) (service.Session, error) {
	if f.startErr != nil {
		return nil, f.startErr
	}
	return f.sess, nil
}

type fakeSession struct {
	out    chan service.Utterance
	once   sync.Once
	closed chan struct{}
}

func newFakeSession() *fakeSession {
	return &fakeSession{out: make(chan service.Utterance, 8), closed: make(chan struct{})}
}

func (s *fakeSession) Write([]byte) error {
	s.out <- service.Utterance{ID: "u1", Text: "hello", Language: "en-IN", Final: true}
	return nil
}

func (s *fakeSession) Next() (service.Utterance, error) {
	select {
	case u := <-s.out:
		return u, nil
	case <-s.closed:
		select {
		case u := <-s.out:
			return u, nil
		default:
			return service.Utterance{}, io.EOF
		}
	}
}

func (s *fakeSession) Close() error { s.once.Do(func() { close(s.closed) }); return nil }

func dial(t *testing.T, speech service.Speech) *websocket.Conn {
	ts := httptest.NewServer(NewHandler(speech, "http://localhost:3000"))
	t.Cleanup(ts.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http")+"/ws", nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func readEvent(t *testing.T, conn *websocket.Conn) map[string]any {
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	var ev map[string]any
	require.NoError(t, conn.ReadJSON(&ev))
	return ev
}

func TestHandler_TranscribeThenDone(t *testing.T) {
	conn := dial(t, &fakeSpeech{sess: newFakeSession()})
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"config","mode":"transcribe"}`)))
	require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, []byte{0, 0}))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("STOP")))

	ev := readEvent(t, conn)
	assert.Equal(t, "utterance", ev["type"])
	assert.Equal(t, "hello", ev["text"])
	assert.Equal(t, "done", readEvent(t, conn)["type"])
}

func TestHandler_BadConfig(t *testing.T) {
	conn := dial(t, &fakeSpeech{sess: newFakeSession()})
	require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, []byte{0, 0}))
	assert.Equal(t, map[string]any{"type": "error", "code": "bad_config"}, readEvent(t, conn))
}

func TestHandler_StartError(t *testing.T) {
	conn := dial(t, &fakeSpeech{startErr: errors.New("down")})
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"config","mode":"transcribe"}`)))
	assert.Equal(t, map[string]any{"type": "error", "code": "provider_unavailable"}, readEvent(t, conn))
}

func TestHandler_ClientDisconnectClosesSession(t *testing.T) {
	sess := newFakeSession()
	conn := dial(t, &fakeSpeech{sess: sess})
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"config","mode":"transcribe"}`)))
	require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, []byte{0, 0}))
	readEvent(t, conn)
	require.NoError(t, conn.Close())

	select {
	case <-sess.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("session not closed after client disconnect")
	}
}

func TestHandler_RejectsForeignOrigin(t *testing.T) {
	ts := httptest.NewServer(NewHandler(&fakeSpeech{sess: newFakeSession()}, "http://localhost:3000"))
	t.Cleanup(ts.Close)
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	_, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": {"http://evil.example"}})
	assert.Error(t, err)
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": {"http://localhost:3000"}})
	require.NoError(t, err)
	conn.Close()
}

func TestHandler_RejectsOutOfRangeSampleRate(t *testing.T) {
	ts := httptest.NewServer(NewHandler(&fakeSpeech{sess: newFakeSession()}, "http://localhost:3000"))
	t.Cleanup(ts.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http")+"/ws?sample_rate=1", nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"config","mode":"transcribe"}`)))
	assert.Equal(t, map[string]any{"type": "error", "code": "bad_config"}, readEvent(t, conn))
}
