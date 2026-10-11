package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLanguage_Methods(t *testing.T) {
	tests := []struct {
		lang       Language
		wantString string
		wantBCP47  string
		wantBase   string
		wantName   string
		supported  bool
	}{
		{lang: LangEnglish, wantString: "en-IN", wantBCP47: "en-IN", wantBase: "en", wantName: "English", supported: true},
		{lang: LangHindi, wantString: "hi-IN", wantBCP47: "hi-IN", wantBase: "hi", wantName: "Hindi", supported: true},
		{lang: LangMalayalam, wantString: "ml-IN", wantBCP47: "ml-IN", wantBase: "ml", wantName: "Malayalam", supported: true},
		{lang: LangKannada, wantString: "kn-IN", wantBCP47: "kn-IN", wantBase: "kn", wantName: "Kannada", supported: true},
		{lang: LangTelugu, wantString: "te-IN", wantBCP47: "te-IN", wantBase: "te", wantName: "Telugu", supported: true},
		{lang: LangMarathi, wantString: "mr-IN", wantBCP47: "mr-IN", wantBase: "mr", wantName: "Marathi", supported: true},
		{lang: LangAuto, wantString: "auto", wantBCP47: "auto", wantBase: "auto", wantName: "Auto", supported: false},
	}

	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			assert.Equal(t, tt.wantString, tt.lang.String())
			assert.Equal(t, tt.wantBCP47, tt.lang.BCP47())
			assert.Equal(t, tt.wantBase, tt.lang.Base())
			assert.Equal(t, tt.wantName, tt.lang.Name())
			assert.Equal(t, tt.supported, tt.lang.IsSupported())
		})
	}
}

func TestParseLanguage(t *testing.T) {
	tests := []struct {
		input   string
		want    Language
		wantErr bool
	}{
		{input: "hi", want: LangHindi},
		{input: "hi-IN", want: LangHindi},
		{input: "hin", want: LangHindi},
		{input: "hi-Deva", want: LangHindi},
		{input: "en", want: LangEnglish},
		{input: "en-US", want: LangEnglish},
		{input: "en-IN", want: LangEnglish},
		{input: "eng", want: LangEnglish},
		{input: "ml", want: LangMalayalam},
		{input: "kn", want: LangKannada},
		{input: "te", want: LangTelugu},
		{input: "mr", want: LangMarathi},
		{input: "auto", want: LangAuto},
		{input: "AUTO", want: LangAuto},
		{input: "", want: LangAuto},
		{input: "french", wantErr: true},
		{input: "es-ES", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLanguage(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
