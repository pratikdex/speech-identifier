package server

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dictation/internal/service"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		raw     string
		want    clientConfig
		wantErr bool
	}{
		{raw: `{"type":"config","mode":"transcribe"}`, want: clientConfig{Mode: service.ModeTranscribe}},
		{raw: `{"type":"config","mode":"transcribe","target":"hi"}`, want: clientConfig{Mode: service.ModeTranscribe}},
		{raw: `{"type":"config","mode":"translate","target":"hi"}`, want: clientConfig{Mode: service.ModeTranslate, Target: "hi-IN"}},
		{raw: `{"type":"config","mode":"translate"}`, wantErr: true},
		{raw: `{"type":"config","mode":"translate","target":"xx"}`, wantErr: true},
		{raw: `{"type":"config","mode":"dance"}`, wantErr: true},
		{raw: `{"type":"hello","mode":"transcribe"}`, wantErr: true},
		{raw: `START`, wantErr: true},
	}
	for _, tt := range tests {
		got, err := parseConfig([]byte(tt.raw))
		if tt.wantErr {
			assert.Error(t, err, tt.raw)
			continue
		}
		require.NoError(t, err, tt.raw)
		assert.Equal(t, tt.want, got, tt.raw)
	}
}

func TestParseConfig_AllTargets(t *testing.T) {
	for _, code := range []string{"en", "hi", "ml", "kn", "te", "mr"} {
		got, err := parseConfig(fmt.Appendf(nil, `{"type":"config","mode":"translate","target":%q}`, code))
		require.NoError(t, err, code)
		assert.Equal(t, code+"-IN", got.Target.String())
	}
}

func TestErrorCode(t *testing.T) {
	assert.Equal(t, "unsupported_language", errorCode(service.ErrUnsupportedLanguage))
	assert.Equal(t, "connection_lost", errorCode(fmt.Errorf("%w: eof", service.ErrConnectionLost)))
	assert.Equal(t, "provider_unavailable", errorCode(errors.New("other")))
}
