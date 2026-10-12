package providers

import "context"

// STTConfig configures a streaming speech-to-text session.
type STTConfig struct {
	SampleRate int
}

// Transcript is a single transcription result emitted by a TranscriptStream.
type Transcript struct {
	Text       string
	Language   string  // detected BCP-47, e.g. "hi-IN"
	Confidence float32 // of Text, 0..1 on finals; 0 when the provider gives none
	Final      bool
	Provider   string
}

// SpeechToText opens streaming transcription sessions.
type SpeechToText interface {
	Name() string
	Open(ctx context.Context, cfg STTConfig) (TranscriptStream, error)
}

// TranscriptStream handles streaming transcription for a single session.
type TranscriptStream interface {
	Write(pcm []byte) error    // 16-bit PCM at cfg.SampleRate
	Next() (Transcript, error) // blocks; io.EOF when the stream ends
	Close() error
}
