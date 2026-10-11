package google

import (
	"context"
	"errors"
	"io"
	"testing"

	speechv2pb "cloud.google.com/go/speech/apiv2/speechpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"dictation/internal/audio"
	"dictation/internal/providers"
)

type mockV2StreamingClient struct {
	mock.Mock
}

func (m *mockV2StreamingClient) Send(req *speechv2pb.StreamingRecognizeRequest) error {
	args := m.Called(req)
	return args.Error(0)
}

func (m *mockV2StreamingClient) Recv() (*speechv2pb.StreamingRecognizeResponse, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*speechv2pb.StreamingRecognizeResponse), args.Error(1)
}

func (m *mockV2StreamingClient) CloseSend() error {
	args := m.Called()
	return args.Error(0)
}

func newTestV2STT(t *testing.T, m *mockV2StreamingClient, opts V2Options) *V2STT {
	t.Helper()
	s := NewV2STT(nil, opts)
	s.open = func(context.Context) (v2StreamingRecognizeClient, error) { return m, nil }
	return s
}

func TestV2STT_Name(t *testing.T) {
	s := NewV2STT(nil, V2Options{Languages: []string{"auto"}})
	assert.Equal(t, "google-v2", s.Name())
}

func TestV2Open_SendsChirpConfig(t *testing.T) {
	m := &mockV2StreamingClient{}
	var sent *speechv2pb.StreamingRecognizeRequest
	m.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		sent = args.Get(0).(*speechv2pb.StreamingRecognizeRequest)
	}).Return(nil).Once()

	opts := V2Options{
		Project:   "my-test-proj",
		Location:  "us-central1",
		Model:     "chirp_2",
		Languages: []string{"auto"},
	}
	s := newTestV2STT(t, m, opts)
	_, err := s.Open(context.Background(), providers.STTConfig{SampleRate: 16000})
	require.NoError(t, err)

	require.NotNil(t, sent)
	assert.Equal(t, "projects/my-test-proj/locations/us-central1/recognizers/_", sent.Recognizer)
	sc := sent.GetStreamingConfig()
	require.NotNil(t, sc)
	assert.True(t, sc.StreamingFeatures.InterimResults)
	assert.Equal(t, "chirp_2", sc.Config.Model)
	assert.Equal(t, []string{"auto"}, sc.Config.LanguageCodes)

	dec := sc.Config.GetExplicitDecodingConfig()
	require.NotNil(t, dec)
	assert.Equal(t, speechv2pb.ExplicitDecodingConfig_LINEAR16, dec.Encoding)
	assert.Equal(t, int32(16000), dec.SampleRateHertz)
	m.AssertExpectations(t)
}

func TestV2Open_SendErrorClosesStream(t *testing.T) {
	m := &mockV2StreamingClient{}
	m.On("Send", mock.Anything).Return(errors.New("send failed")).Once()
	m.On("CloseSend").Return(nil).Once()

	s := newTestV2STT(t, m, V2Options{})
	_, err := s.Open(context.Background(), providers.STTConfig{SampleRate: 16000})
	assert.Error(t, err)
	m.AssertExpectations(t)
}

func TestV2Write_ChunksAudio(t *testing.T) {
	m := &mockV2StreamingClient{}
	// Send 25 KB of audio -> should be chunked into 3 calls (10 KB + 10 KB + 5 KB)
	largePCM := make([]byte, 25*1024)
	var chunkSizes []int

	m.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(0).(*speechv2pb.StreamingRecognizeRequest)
		chunkSizes = append(chunkSizes, len(req.GetAudio()))
	}).Return(nil).Times(3)

	stream := &v2Stream{rc: m, sampleRate: audio.TargetRate}
	err := stream.Write(largePCM)
	require.NoError(t, err)
	assert.Equal(t, []int{10 * 1024, 10 * 1024, 5 * 1024}, chunkSizes)
	m.AssertExpectations(t)
}

func v2Result(text, lang string, final bool) *speechv2pb.StreamingRecognitionResult {
	return &speechv2pb.StreamingRecognitionResult{
		IsFinal:      final,
		LanguageCode: lang,
		Alternatives: []*speechv2pb.SpeechRecognitionAlternative{{Transcript: text, Confidence: 0.95}},
	}
}

func v2Response(results ...*speechv2pb.StreamingRecognitionResult) *speechv2pb.StreamingRecognizeResponse {
	return &speechv2pb.StreamingRecognizeResponse{Results: results}
}

func TestV2Next_ForwardsInterimAndFinalWithLanguage(t *testing.T) {
	m := &mockV2StreamingClient{}
	m.On("Recv").Return(v2Response(v2Result("नमस्ते", "hi-in", false)), nil).Once()
	m.On("Recv").Return(v2Response(v2Result("नमस्ते दुनिया", "hi-in", true)), nil).Once()

	s := &v2Stream{rc: m, sampleRate: audio.TargetRate}

	got, err := s.Next()
	require.NoError(t, err)
	assert.Equal(t, providers.Transcript{Text: "नमस्ते", Language: "hi-IN", Final: false, Provider: "google-v2"}, got)

	got, err = s.Next()
	require.NoError(t, err)
	assert.Equal(t, providers.Transcript{Text: "नमस्ते दुनिया", Language: "hi-IN", Final: true, Provider: "google-v2"}, got)
	m.AssertExpectations(t)
}

func TestV2Next_Errors(t *testing.T) {
	internal := status.Error(codes.Internal, "internal error")
	tests := []struct {
		name    string
		recvErr error
		wantErr error
	}{
		{name: "io.EOF", recvErr: io.EOF, wantErr: io.EOF},
		{name: "context canceled maps to io.EOF", recvErr: status.Error(codes.Canceled, "canceled"), wantErr: io.EOF},
		{name: "other grpc error", recvErr: internal, wantErr: internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockV2StreamingClient{}
			m.On("Recv").Return(nil, tt.recvErr).Once()

			s := &v2Stream{rc: m, sampleRate: audio.TargetRate}
			_, err := s.Next()

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestV2Close(t *testing.T) {
	m := &mockV2StreamingClient{}
	m.On("CloseSend").Return(nil).Once()

	s := &v2Stream{rc: m, sampleRate: audio.TargetRate}
	assert.NoError(t, s.Close())
	m.AssertExpectations(t)
}
