package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvider_Methods(t *testing.T) {
	assert.Equal(t, "google", ProviderGoogle.String())
	assert.Equal(t, "google-v2", ProviderGoogleV2.String())
	assert.Equal(t, "sarvam", ProviderSarvam.String())

}

func TestParseProvider(t *testing.T) {
	tests := []struct {
		input   string
		want    Provider
		wantErr bool
	}{
		{input: "google", want: ProviderGoogle},
		{input: "GOOGLE", want: ProviderGoogle},
		{input: "google-v2", want: ProviderGoogleV2},
		{input: "google_v2", want: ProviderGoogleV2},
		{input: "googlev2", want: ProviderGoogleV2},
		{input: "sarvam", want: ProviderSarvam},
		{input: "SARVAM", want: ProviderSarvam},
		{input: "aws", wantErr: true},
		{input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseProvider(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
