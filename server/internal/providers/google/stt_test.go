package google

import (
	"context"
	"errors"
	"io"
	"testing"

	"cloud.google.com/go/speech/apiv1/speechpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"dictation/internal/audio"
	"dictation/internal/providers"
)

var testLanguages = []string{"en-IN", "hi-IN", "ml-IN", "kn-IN"}

// newTestSTT returns an STT whose streams are the given mock.
func newTestSTT(t *testing.T, m *mockstreamingRecognizeClient) *STT {
	t.Helper()
	s, err := NewSTT(nil, testLanguages)
	require.NoError(t, err)
	s.open = func(context.Context) (streamingRecognizeClient, error) { return m, nil }
	return s
}

func TestNewSTT_RejectsFiveLanguages(t *testing.T) {
	_, err := NewSTT(nil, []string{"en-IN", "hi-IN", "ml-IN", "kn-IN", "te-IN"})
	assert.Error(t, err)
}

func TestNewSTT_RejectsNoLanguages(t *testing.T) {
	_, err := NewSTT(nil, nil)
	assert.Error(t, err)
}

func TestSTT_Name(t *testing.T) {
	s, err := NewSTT(nil, testLanguages)
	require.NoError(t, err)

	assert.Equal(t, "google", s.Name())
}

func TestOpen_SendsPrimaryAndAlternatives(t *testing.T) {
	m := newMockstreamingRecognizeClient(t)
	var sent *speechpb.StreamingRecognizeRequest
	m.EXPECT().Send(mock.Anything).Run(func(req *speechpb.StreamingRecognizeRequest) { sent = req }).Return(nil).Once()

	_, err := newTestSTT(t, m).Open(context.Background(), providers.STTConfig{SampleRate: 48000})
	require.NoError(t, err)

	require.NotNil(t, sent)
	sc := sent.GetStreamingConfig()
	require.NotNil(t, sc)
	assert.True(t, sc.InterimResults)
	assert.Equal(t, speechpb.RecognitionConfig_LINEAR16, sc.Config.Encoding)
	assert.Equal(t, int32(16000), sc.Config.SampleRateHertz)
	assert.Equal(t, "en-IN", sc.Config.LanguageCode)
	assert.Equal(t, []string{"hi-IN", "ml-IN", "kn-IN"}, sc.Config.AlternativeLanguageCodes)
}

func TestOpen_ConfigSendErrorClosesStream(t *testing.T) {
	m := newMockstreamingRecognizeClient(t)
	m.EXPECT().Send(mock.Anything).Return(errors.New("send failed")).Once()
	m.EXPECT().CloseSend().Return(nil).Once()

	_, err := newTestSTT(t, m).Open(context.Background(), providers.STTConfig{SampleRate: 16000})
	assert.Error(t, err)
}

func TestWrite(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate int
		pcm        []byte
		sendErr    error
		wantLen    int // expected AudioContent length; delta 64 bytes for resampling
		wantErr    bool
	}{
		{name: "passes 16 kHz through", sampleRate: audio.TargetRate, pcm: []byte("test audio data!"), wantLen: 16},
		{name: "resamples 48 kHz to 16 kHz", sampleRate: 48000, pcm: make([]byte, 48000*2), wantLen: 16000 * 2},
		{name: "send error", sampleRate: audio.TargetRate, pcm: []byte("ab"), sendErr: errors.New("send failed"), wantLen: 2, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockstreamingRecognizeClient(t)
			var sent *speechpb.StreamingRecognizeRequest
			m.EXPECT().Send(mock.Anything).Run(func(req *speechpb.StreamingRecognizeRequest) { sent = req }).Return(tt.sendErr).Once()

			s := &stream{rc: m, sampleRate: tt.sampleRate}
			err := s.Write(tt.pcm)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			require.NotNil(t, sent)
			assert.InDelta(t, tt.wantLen, len(sent.GetAudioContent()), 64)
		})
	}
}

func TestWrite_ResampleErrorDoesNotSend(t *testing.T) {
	m := newMockstreamingRecognizeClient(t)
	s := &stream{rc: m, sampleRate: 48000}

	assert.Error(t, s.Write([]byte{1, 2, 3}))
}

func result(text, lang string, final bool) *speechpb.StreamingRecognitionResult {
	return &speechpb.StreamingRecognitionResult{
		IsFinal:      final,
		LanguageCode: lang,
		Alternatives: []*speechpb.SpeechRecognitionAlternative{{Transcript: text, Confidence: 0.9}},
	}
}

func response(results ...*speechpb.StreamingRecognitionResult) *speechpb.StreamingRecognizeResponse {
	return &speechpb.StreamingRecognizeResponse{Results: results}
}

func TestNext_ForwardsInterimAndFinalWithLanguage(t *testing.T) {
	m := newMockstreamingRecognizeClient(t)
	m.EXPECT().Recv().Return(response(result("नमस्ते", "hi-in", false)), nil).Once()
	m.EXPECT().Recv().Return(response(result("नमस्ते दुनिया", "hi-in", true)), nil).Once()

	s := &stream{rc: m, sampleRate: audio.TargetRate}

	got, err := s.Next()
	require.NoError(t, err)
	assert.Equal(t, providers.Transcript{Text: "नमस्ते", Language: "hi-IN", Final: false, Provider: "google"}, got)

	got, err = s.Next()
	require.NoError(t, err)
	assert.Equal(t, providers.Transcript{Text: "नमस्ते दुनिया", Language: "hi-IN", Confidence: 0.9, Final: true, Provider: "google"}, got)
}

func TestNext_JoinsInterimResultsAndQueuesFinals(t *testing.T) {
	m := newMockstreamingRecognizeClient(t)
	m.EXPECT().Recv().Return(response(result("hello", "en-in", false), result(" world", "en-in", false)), nil).Once()
	m.EXPECT().Recv().Return(response(result("one", "en-in", true), result("two", "en-in", true)), nil).Once()

	s := &stream{rc: m, sampleRate: audio.TargetRate}

	got, err := s.Next()
	require.NoError(t, err)
	assert.Equal(t, providers.Transcript{Text: "hello world", Language: "en-IN", Provider: "google"}, got)

	got, err = s.Next()
	require.NoError(t, err)
	assert.Equal(t, "one", got.Text)
	assert.True(t, got.Final)

	got, err = s.Next()
	require.NoError(t, err)
	assert.Equal(t, "two", got.Text)
	assert.True(t, got.Final)
}

func TestNext_SkipsResponsesWithoutAlternatives(t *testing.T) {
	m := newMockstreamingRecognizeClient(t)
	m.EXPECT().Recv().Return(response(&speechpb.StreamingRecognitionResult{IsFinal: true}), nil).Once()
	m.EXPECT().Recv().Return(response(), nil).Once()
	m.EXPECT().Recv().Return(response(result("test", "en-in", true)), nil).Once()

	s := &stream{rc: m, sampleRate: audio.TargetRate}

	got, err := s.Next()
	require.NoError(t, err)
	assert.Equal(t, "test", got.Text)
}

func TestNext_Errors(t *testing.T) {
	nwErr := errors.New("network error")
	internal := status.Error(codes.Internal, "internal error")
	tests := []struct {
		name    string
		recvErr error
		wantErr error
	}{
		{name: "io.EOF", recvErr: io.EOF, wantErr: io.EOF},
		{name: "context canceled maps to io.EOF", recvErr: status.Error(codes.Canceled, "context canceled"), wantErr: io.EOF},
		{name: "other grpc error", recvErr: internal, wantErr: internal},
		{name: "generic error", recvErr: nwErr, wantErr: nwErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockstreamingRecognizeClient(t)
			m.EXPECT().Recv().Return(nil, tt.recvErr).Once()

			s := &stream{rc: m, sampleRate: audio.TargetRate}
			_, err := s.Next()

			assert.ErrorIs(t, err, tt.wantErr)
			if tt.wantErr != io.EOF {
				assert.NotErrorIs(t, err, io.EOF)
			}
		})
	}
}

func TestClose(t *testing.T) {
	closeErr := errors.New("close failed")
	for _, wantErr := range []error{nil, closeErr} {
		m := newMockstreamingRecognizeClient(t)
		m.EXPECT().CloseSend().Return(wantErr).Once()

		s := &stream{rc: m, sampleRate: audio.TargetRate}
		assert.Equal(t, wantErr, s.Close())
	}
}

func TestNormalizeLang(t *testing.T) {
	tests := map[string]string{
		"hi-in": "hi-IN",
		"en":    "en-IN",
		"fr-fr": "fr-FR",
		"EN-in": "en-IN",
		"kn-IN": "kn-IN",
		"":      "",
	}
	for in, want := range tests {
		assert.Equal(t, want, normalizeLang(in), in)
	}
}
