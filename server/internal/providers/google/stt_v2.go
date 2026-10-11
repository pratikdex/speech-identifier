// Package google adapts Google Cloud Speech-to-Text (apiv2 streaming with Chirp) to
// providers.SpeechToText.
package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	speechv2 "cloud.google.com/go/speech/apiv2"
	speechv2pb "cloud.google.com/go/speech/apiv2/speechpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"dictation/internal/audio"
	"dictation/internal/providers"
)

const defaultV2Location = "us-central1"
const defaultV2Model = "chirp_2"

// maxAudioChunkBytes is the maximum audio payload per StreamingRecognizeRequest
// allowed by Google STT v2 (limit is 15 KB).
const maxAudioChunkBytes = 10 * 1024

// v2StreamingRecognizeClient wraps the methods needed from speechv2pb.Speech_StreamingRecognizeClient.
type v2StreamingRecognizeClient interface {
	Send(*speechv2pb.StreamingRecognizeRequest) error
	Recv() (*speechv2pb.StreamingRecognizeResponse, error)
	CloseSend() error
}

// V2Options configures the Google STT v2 provider.
type V2Options struct {
	Project   string
	Location  string
	Model     string
	Languages []string
}

// withDefaults fills empty fields with defaults for Chirp.
func (o V2Options) withDefaults() V2Options {
	if o.Location == "" {
		o.Location = defaultV2Location
	}
	if o.Model == "" {
		o.Model = defaultV2Model
	}
	if o.Project == "" {
		o.Project = "_"
	}
	if len(o.Languages) == 0 {
		o.Languages = []string{"auto"}
	}
	return o
}

// V2OptionsFromEnv reads V2Options from the environment. Call it once at
// startup; the adapter itself never reads the environment.
func V2OptionsFromEnv() V2Options {
	o := V2Options{
		Location: firstEnv("GOOGLE_CLOUD_LOCATION", "GCP_LOCATION"),
		Model:    os.Getenv("GOOGLE_STT_V2_MODEL"),
		Project:  resolveProject(),
	}
	if langs := os.Getenv("GOOGLE_STT_V2_LANGUAGES"); langs != "" {
		o.Languages = strings.Split(langs, ",")
	}
	return o
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func resolveProject() string {
	if p := os.Getenv("GOOGLE_CLOUD_PROJECT"); p != "" {
		return p
	}
	if p := os.Getenv("GCP_PROJECT"); p != "" {
		return p
	}
	if credPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); credPath != "" {
		if data, err := os.ReadFile(credPath); err == nil {
			var creds struct {
				ProjectID      string `json:"project_id"`
				QuotaProjectID string `json:"quota_project_id"`
			}
			if err := json.Unmarshal(data, &creds); err == nil {
				if creds.ProjectID != "" {
					return creds.ProjectID
				}
				if creds.QuotaProjectID != "" {
					return creds.QuotaProjectID
				}
			}
		}
	}
	return ""
}

// NewV2Client creates a speechv2.Client configured for the specified location's regional endpoint.
func NewV2Client(ctx context.Context, o V2Options) (*speechv2.Client, error) {
	o = o.withDefaults()
	endpoint := fmt.Sprintf("%s-speech.googleapis.com:443", o.Location)
	return speechv2.NewClient(ctx, option.WithEndpoint(endpoint))
}

// V2STT implements providers.SpeechToText for Google Speech-to-Text v2 (Chirp),
// with automatic language detection.
type V2STT struct {
	opts V2Options
	open func(ctx context.Context) (v2StreamingRecognizeClient, error)
}

var _ providers.SpeechToText = (*V2STT)(nil)

// NewV2STT returns a Google STT v2 provider.
func NewV2STT(client *speechv2.Client, o V2Options) *V2STT {
	opts := o.withDefaults()
	return &V2STT{
		opts: opts,
		open: func(ctx context.Context) (v2StreamingRecognizeClient, error) {
			return client.StreamingRecognize(ctx)
		},
	}
}

// Name returns "google-v2".
func (s *V2STT) Name() string {
	return providers.ProviderGoogleV2.String()
}

// Open starts a streaming recognition session using v2 Chirp model.
func (s *V2STT) Open(ctx context.Context, cfg providers.STTConfig) (providers.TranscriptStream, error) {
	rc, err := s.open(ctx)
	if err != nil {
		return nil, err
	}

	langs := s.opts.Languages

	recognizer := fmt.Sprintf("projects/%s/locations/%s/recognizers/_", s.opts.Project, s.opts.Location)

	req := &speechv2pb.StreamingRecognizeRequest{
		Recognizer: recognizer,
		StreamingRequest: &speechv2pb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechv2pb.StreamingRecognitionConfig{
				Config: &speechv2pb.RecognitionConfig{
					Model:         s.opts.Model,
					LanguageCodes: langs,
					DecodingConfig: &speechv2pb.RecognitionConfig_ExplicitDecodingConfig{
						ExplicitDecodingConfig: &speechv2pb.ExplicitDecodingConfig{
							Encoding:          speechv2pb.ExplicitDecodingConfig_LINEAR16,
							SampleRateHertz:   int32(audio.TargetRate),
							AudioChannelCount: 1,
						},
					},
				},
				StreamingFeatures: &speechv2pb.StreamingRecognitionFeatures{
					InterimResults: true,
				},
			},
		},
	}

	if err := rc.Send(req); err != nil {
		_ = rc.CloseSend()
		return nil, fmt.Errorf("google-v2: sending RecognitionConfig: %w", err)
	}

	return &v2Stream{rc: rc, sampleRate: cfg.SampleRate}, nil
}

type v2Stream struct {
	rc         v2StreamingRecognizeClient
	sampleRate int
	pending    []providers.Transcript
}

func (s *v2Stream) Write(pcm []byte) error {
	if s.sampleRate != audio.TargetRate {
		var err error
		pcm, err = audio.Resample(pcm, s.sampleRate, audio.TargetRate)
		if err != nil {
			return fmt.Errorf("google-v2: resampling audio: %w", err)
		}
	}

	for len(pcm) > 0 {
		chunk := pcm
		if len(chunk) > maxAudioChunkBytes {
			chunk = chunk[:maxAudioChunkBytes]
		}
		pcm = pcm[len(chunk):]
		err := s.rc.Send(&speechv2pb.StreamingRecognizeRequest{
			StreamingRequest: &speechv2pb.StreamingRecognizeRequest_Audio{
				Audio: chunk,
			},
		})
		if err != nil {
			return fmt.Errorf("google-v2: sending audio chunk: %w", err)
		}
	}
	return nil
}

func (s *v2Stream) Next() (providers.Transcript, error) {
	for len(s.pending) == 0 {
		resp, err := s.rc.Recv()
		if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
			return providers.Transcript{}, io.EOF
		}
		if err != nil {
			return providers.Transcript{}, fmt.Errorf("google-v2: receiving transcription: %w", err)
		}
		s.pending = v2Transcripts(resp)
	}
	t := s.pending[0]
	s.pending = s.pending[1:]
	return t, nil
}

func (s *v2Stream) Close() error {
	return s.rc.CloseSend()
}

func v2Transcripts(resp *speechv2pb.StreamingRecognizeResponse) []providers.Transcript {
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
				Language: normalizeV2Lang(r.GetLanguageCode()),
				Final:    true,
				Provider: providers.ProviderGoogleV2.String(),
			})
			continue
		}
		if interimLang == "" {
			interimLang = normalizeV2Lang(r.GetLanguageCode())
		}
		interim.WriteString(text)
	}
	if interim.Len() > 0 {
		out = append(out, providers.Transcript{
			Text:     interim.String(),
			Language: interimLang,
			Provider: providers.ProviderGoogleV2.String(),
		})
	}
	return out
}

func normalizeV2Lang(s string) string {
	lang, err := providers.ParseLanguage(s)
	if err == nil && lang.IsSupported() {
		return lang.BCP47()
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	l, region, ok := strings.Cut(s, "-")
	if !ok {
		return strings.ToLower(l)
	}
	return strings.ToLower(l) + "-" + strings.ToUpper(region)
}
