// Package google adapts Google Cloud Speech-to-Text (apiv1 streaming) to
// providers.SpeechToText.
package google

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	speech "cloud.google.com/go/speech/apiv1"
	"cloud.google.com/go/speech/apiv1/speechpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"dictation/internal/audio"
	"dictation/internal/providers"
)

const providerName = string(providers.ProviderGoogle)

// maxLanguages is Google's limit: one primary language plus up to three
// alternative_language_codes.
const maxLanguages = 4

// streamingRecognizeClient is a local interface that wraps the methods we need
// from speechpb.Speech_StreamingRecognizeClient to enable easier testing
type streamingRecognizeClient interface {
	Send(*speechpb.StreamingRecognizeRequest) error
	Recv() (*speechpb.StreamingRecognizeResponse, error)
	CloseSend() error
}

// STT implements providers.SpeechToText for Google Speech-to-Text, with
// automatic language detection across the configured languages.
type STT struct {
	languages []string
	open      func(ctx context.Context) (streamingRecognizeClient, error)
}

var _ providers.SpeechToText = (*STT)(nil)

// NewSTT returns a Google STT provider. languages[0] is the primary
// language and the rest (at most three) are alternatives Google may detect.
func NewSTT(client *speech.Client, languages []string) (*STT, error) {
	if len(languages) == 0 || len(languages) > maxLanguages {
		return nil, fmt.Errorf("google: need 1 to %d languages, got %d", maxLanguages, len(languages))
	}
	return &STT{
		languages: append([]string(nil), languages...),
		open: func(ctx context.Context) (streamingRecognizeClient, error) {
			return client.StreamingRecognize(ctx)
		},
	}, nil
}

// Name returns "google".
func (s *STT) Name() string {
	return providerName
}

// Open starts a streaming recognition session. Audio is resampled to
// audio.TargetRate before it is sent.
func (s *STT) Open(ctx context.Context, cfg providers.STTConfig) (providers.TranscriptStream, error) {
	rc, err := s.open(ctx)
	if err != nil {
		return nil, err
	}

	req := &speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					Encoding:                 speechpb.RecognitionConfig_LINEAR16,
					SampleRateHertz:          audio.TargetRate,
					LanguageCode:             s.languages[0],
					AlternativeLanguageCodes: s.languages[1:],
				},
				InterimResults: true,
			},
		},
	}
	if err := rc.Send(req); err != nil {
		_ = rc.CloseSend()
		return nil, fmt.Errorf("google: sending RecognitionConfig: %w", err)
	}

	return &stream{rc: rc, sampleRate: cfg.SampleRate}, nil
}

// stream implements providers.TranscriptStream over a Google streaming
// recognize call. Write and Close may run concurrently with Next.
type stream struct {
	rc         streamingRecognizeClient
	sampleRate int

	// pending holds transcripts decoded from a response but not yet
	// returned. Only Next touches it.
	pending []providers.Transcript
}

// Write sends 16-bit PCM, resampling it to audio.TargetRate if needed.
func (s *stream) Write(pcm []byte) error {
	if s.sampleRate != audio.TargetRate {
		var err error
		pcm, err = audio.Resample(pcm, s.sampleRate, audio.TargetRate)
		if err != nil {
			return fmt.Errorf("google: resampling audio: %w", err)
		}
	}
	return s.rc.Send(&speechpb.StreamingRecognizeRequest{
		StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{AudioContent: pcm},
	})
}

// Next returns the next interim or final transcript. It returns io.EOF when
// the stream ends normally or its context is canceled.
func (s *stream) Next() (providers.Transcript, error) {
	for len(s.pending) == 0 {
		resp, err := s.rc.Recv()
		if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
			return providers.Transcript{}, io.EOF
		}
		if err != nil {
			return providers.Transcript{}, fmt.Errorf("google: receiving transcription: %w", err)
		}
		s.pending = transcripts(resp)
	}
	t := s.pending[0]
	s.pending = s.pending[1:]
	return t, nil
}

// Close half-closes the stream; Google then flushes its last results and
// Next returns io.EOF.
func (s *stream) Close() error {
	return s.rc.CloseSend()
}

// transcripts decodes a response: each final result becomes its own
// transcript, and the interim results (consecutive pieces of the current
// hypothesis) are joined into one.
func transcripts(resp *speechpb.StreamingRecognizeResponse) []providers.Transcript {
	var out []providers.Transcript
	var interim strings.Builder
	interimLang := ""
	for _, r := range resp.GetResults() {
		if len(r.GetAlternatives()) == 0 {
			continue
		}
		text := r.GetAlternatives()[0].GetTranscript()
		if r.GetIsFinal() {
			out = append(out, providers.Transcript{
				Text:     text,
				Language: normalizeLang(r.GetLanguageCode()),
				Final:    true,
				Provider: providerName,
			})
			continue
		}
		if interimLang == "" {
			interimLang = normalizeLang(r.GetLanguageCode())
		}
		interim.WriteString(text)
	}
	if interim.Len() > 0 {
		out = append(out, providers.Transcript{
			Text:     interim.String(),
			Language: interimLang,
			Provider: providerName,
		})
	}
	return out
}

// normalizeLang canonicalizes a BCP-47 tag as Google returns it ("hi-in")
// to the form used inside the server ("hi-IN").
func normalizeLang(s string) string {
	lang, region, ok := strings.Cut(s, "-")
	if !ok {
		return strings.ToLower(s)
	}
	return strings.ToLower(lang) + "-" + strings.ToUpper(region)
}
