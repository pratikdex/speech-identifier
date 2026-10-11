package google

import (
	"context"
	"dictation/internal/providers"
	"errors"
	"testing"

	gtranslate "cloud.google.com/go/translate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestTranslator_NameAndSupports(t *testing.T) {
	tr := NewTranslator(nil)
	assert.Equal(t, "google", tr.Name())

	tests := []struct {
		name string
		src  string
		dst  string
		want bool
	}{
		{name: "hi to en", src: "hi-IN", dst: "en-IN", want: true},
		{name: "mr to en", src: "mr-IN", dst: "en-IN", want: true},
		{name: "same language", src: "hi-IN", dst: "hi-IN", want: false},
		{name: "auto source", src: "auto", dst: "en-IN", want: true},
		{name: "auto target", src: "en-IN", dst: "auto", want: false},
		{name: "unsupported source", src: "fr-FR", dst: "en-IN", want: false},
		{name: "unsupported target", src: "en-IN", dst: "fr-FR", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tr.Supports(providers.Language(tt.src), providers.Language(tt.dst)))
		})
	}
}

func TestTranslate_StripsRegionAndUsesTextFormat(t *testing.T) {
	m := newMocktranslateClient(t)
	m.EXPECT().
		Translate(mock.Anything, []string{"नमस्ते"}, language.English, mock.MatchedBy(func(opts *gtranslate.Options) bool {
			return opts.Source == language.Hindi && opts.Format == gtranslate.Text
		})).
		Return([]gtranslate.Translation{{Text: "hello"}}, nil).
		Once()

	tr := NewTranslator(m)
	got, err := tr.Translate(context.Background(), "नमस्ते", "hi-IN", "en-IN")
	require.NoError(t, err)
	assert.Equal(t, "hello", got)
}

func TestTranslate_ClientErrorPropagates(t *testing.T) {
	m := newMocktranslateClient(t)
	wantErr := errors.New("translate api failed")
	m.EXPECT().Translate(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, wantErr).Once()

	tr := NewTranslator(m)
	_, err := tr.Translate(context.Background(), "hello", "en-IN", "hi-IN")
	assert.ErrorIs(t, err, wantErr)
}

func TestTranslate_EmptyResultIsError(t *testing.T) {
	m := newMocktranslateClient(t)
	m.EXPECT().Translate(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()

	tr := NewTranslator(m)
	_, err := tr.Translate(context.Background(), "hello", "en-IN", "hi-IN")
	assert.Error(t, err)
}

func TestTranslate_AutoSourceOmitsSource(t *testing.T) {
	m := newMocktranslateClient(t)
	m.EXPECT().
		Translate(mock.Anything, []string{"नमस्ते"}, language.English, mock.MatchedBy(func(opts *gtranslate.Options) bool {
			return opts.Source == language.Und && opts.Format == gtranslate.Text
		})).
		Return([]gtranslate.Translation{{Text: "hello"}}, nil).
		Once()

	tr := NewTranslator(m)
	got, err := tr.Translate(context.Background(), "नमस्ते", "auto", "en-IN")
	require.NoError(t, err)
	assert.Equal(t, "hello", got)
}
