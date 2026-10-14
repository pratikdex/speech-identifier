package speech_identifier

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/pratikdex/speech-identifier/providers"
)

// WebSocketRequest represents an audio data message sent from the client to the server.
// It contains raw audio bytes that will be forwarded to the Google Speech API.
type WebSocketRequest struct {
	Buf []byte `json:"buf"`
}

// WebSocketResponse represents a transcription result sent from the server to the client.
// It contains the final transcribed text and confidence score from the transcription provider.
type WebSocketResponse struct {
	Sentence   string  `json:"sentence"`
	Confidence float32 `json:"confidence"`
}

// WebConn represents a WebSocket connection that bridges client audio data
// with speech transcription providers. It manages bidirectional
// communication between the WebSocket client and the transcription service.
type WebConn struct {
	conn          *websocket.Conn
	log           *log.Logger
	wg            sync.WaitGroup
	session       providers.Session
	writerStarted bool
	writerMutex   sync.Mutex
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  8192,
		WriteBufferSize: 8192,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Printf("WebSocket upgrade failed: %v\n", err)
		return
	}

	s.log.Println("Creating provider selector...", r.URL.Query())
	sampleRate, _ := strconv.Atoi(r.URL.Query().Get("sample_rate"))
	if sampleRate == 0 {
		sampleRate = 16000
	}
	config := providers.SessionConfig{
		SampleRate:     sampleRate,
		LanguageCode:   "en-US",
		InterimResults: true,
	}

	ctx := context.Background()
	sess, err := s.provider.NewSession(ctx, config)
	if err != nil {
		s.log.Printf("Failed to create session for provider %s: %v", s.provider.Name(), err)
		return
	}

	webConn := &WebConn{
		conn:    conn,
		log:     s.log,
		session: sess,
	}

	// Register connection for tracking
	s.addConn(webConn)
	defer s.removeConn(webConn)
	webConn.Start()
}

func (wc *WebConn) Start() {
	defer wc.conn.Close()

	wc.reader()
	wc.log.Println("Closing transcription session...")
	// Close session, which will cancel context and allow writer to exit
	// Important to call this _after_ wc.reader() exits.
	wc.session.Close()
	wc.wg.Wait()
}

// Stop gracefully closes the WebSocket connection and waits for all
// goroutines to finish. This method is safe to call multiple times.
func (wc *WebConn) Stop() {
	// Close the connection, which will cause reader() to exit
	wc.conn.Close()
	// Wait for all goroutines to finish
	wc.wg.Wait()
}

func (wc *WebConn) startWriter() {
	wc.writerMutex.Lock()
	defer wc.writerMutex.Unlock()

	if wc.writerStarted {
		return
	}

	wc.writerStarted = true
	wc.wg.Add(1)
	go func() {
		defer wc.wg.Done()
		wc.writer()
	}()
}

func (wc *WebConn) reader() {
	for {
		_, res, err := wc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				wc.log.Printf("WebSocket read error: %v\n", err)
			}
			break
		}

		if bytes.Compare(res, []byte("START")) == 0 {
			wc.startWriter()
			continue
		}
		if bytes.Compare(res, []byte("STOP")) == 0 {
			continue
		}

		// Send audio bytes to transcription session
		if err := wc.session.SendAudio(res); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			wc.log.Printf("session.SendAudio error: %v\n", err)
		}
	}
}

func (wc *WebConn) writer() {
	for {
		result, err := wc.session.ReceiveTranscription()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			wc.log.Printf("session.ReceiveTranscription error: %v\n", err)
			return
		}

		response := WebSocketResponse{
			Sentence:   result.Text,
			Confidence: result.Confidence,
		}

		if err := wc.conn.WriteJSON(response); err != nil {
			wc.log.Printf("WebSocket write error: %v\n", err)
			return
		}
	}
}
