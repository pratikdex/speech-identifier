// Package server serves dictation sessions over WebSocket.
//
// Protocol: the client's first text frame is the config (see parseConfig),
// then it streams binary 16-bit PCM frames and sends "STOP" when done. The
// server sends utterance events as they arrive, then "done" (or an error)
// and closes the connection.
package server

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"dictation/internal/service"
)

const writeWait = 10 * time.Second

const (
	configWait   = 10 * time.Second // the client must send its config frame within this
	maxFrameSize = 1 << 20          // ~10s of 48 kHz PCM; larger frames are rejected
)

// NewHandler returns an http.Handler serving sessions at /ws. Browsers must
// connect from allowedOrigin (clients that send no Origin header, such as
// tests and CLIs, are always allowed). Cancelling the requests' base context
// (server shutdown) closes open sessions.
func NewHandler(speech service.Speech, allowedOrigin string) http.Handler {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  8192,
		WriteBufferSize: 8192,
		CheckOrigin: func(r *http.Request) bool {
			o := r.Header.Get("Origin")
			return o == "" || o == allowedOrigin
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade: %v", err)
			return
		}
		serve(r.Context(), conn, speech, r.URL.Query().Get("sample_rate"))
	})
	return mux
}

func serve(ctx context.Context, conn *websocket.Conn, speech service.Speech, sampleRate string) {
	defer conn.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { <-ctx.Done(); conn.Close() }() // unblock reads on shutdown

	conn.SetReadLimit(maxFrameSize)
	_ = conn.SetReadDeadline(time.Now().Add(configWait))
	opts, err := readConfig(conn, sampleRate)
	_ = conn.SetReadDeadline(time.Time{}) // audio may pause for as long as the user is silent
	if err != nil {
		log.Printf("rejecting session: %v", err)
		finish(conn, event{Type: "error", Code: ErrorCodeBadConfig.String()})
		return
	}
	sess, err := speech.Start(ctx, opts)
	if err != nil {
		log.Printf("starting session: %v", err)
		finish(conn, event{Type: "error", Code: errorCode(err)})
		return
	}

	written := make(chan struct{})
	go func() {
		defer close(written)
		writeUtterances(conn, sess)
	}()

	readAudio(conn, sess)
	_ = sess.Close() // ends the writer's Next with io.EOF
	<-written
}

// readConfig reads the first frame and the sample_rate query value
// (missing means 16000).
func readConfig(conn *websocket.Conn, rawRate string) (service.SessionOptions, error) {
	msgType, data, err := conn.ReadMessage()
	if err != nil {
		return service.SessionOptions{}, err
	}
	if msgType != websocket.TextMessage {
		return service.SessionOptions{}, errors.New("first frame is not text")
	}
	cfg, err := parseConfig(data)
	if err != nil {
		return service.SessionOptions{}, err
	}
	rate := 16000
	if rawRate != "" {
		if rate, err = strconv.Atoi(rawRate); err != nil || rate < 8000 || rate > 192000 {
			return service.SessionOptions{}, errors.New("invalid sample_rate")
		}
	}
	return service.SessionOptions{SampleRate: rate, Mode: cfg.Mode, Target: cfg.Target}, nil
}

// readAudio forwards audio frames to the session until the client sends
// STOP or the connection closes. Other text frames are ignored.
func readAudio(conn *websocket.Conn, sess service.Session) {
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if msgType == websocket.BinaryMessage {
			if err := sess.Write(data); err != nil {
				log.Printf("session write: %v", err)
				return
			}
		} else if string(data) == "STOP" {
			return
		}
	}
}

// writeUtterances sends each utterance to the client, then done or an error.
func writeUtterances(conn *websocket.Conn, sess service.Session) {
	for {
		u, err := sess.Next()
		if errors.Is(err, io.EOF) {
			finish(conn, event{Type: "done"})
			return
		}
		if err != nil {
			log.Printf("session: %v", err)
			finish(conn, event{Type: "error", Code: errorCode(err)})
			return
		}
		ev := utteranceEvent{Type: "utterance", ID: u.ID, Text: u.Text, Lang: u.Language, Final: u.Final, Translation: u.Translation, Confidence: u.Confidence}
		if err := writeJSON(conn, ev); err != nil {
			conn.Close()
			return
		}
	}
}

func writeJSON(conn *websocket.Conn, v any) error {
	_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
	return conn.WriteJSON(v)
}

// finish sends a last event and a close frame, then closes the connection.
func finish(conn *websocket.Conn, ev event) {
	_ = writeJSON(conn, ev)
	_ = conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(writeWait))
	conn.Close()
}
