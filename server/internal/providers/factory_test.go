package providers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"dictation/internal/providers"
	"dictation/internal/providers/mocks"
)

func stt(t *testing.T, name string) *mocks.MockSpeechToText {
	m := mocks.NewMockSpeechToText(t)
	m.EXPECT().Name().Return(name).Maybe()
	return m
}

func names(l []providers.SpeechToText) (out []string) {
	for _, p := range l {
		out = append(out, p.Name())
	}
	return
}

func TestPreferFirst(t *testing.T) {
	list := []providers.SpeechToText{stt(t, "google"), stt(t, "google-v2"), stt(t, "sarvam")}
	assert.Equal(t, []string{"sarvam", "google", "google-v2"}, names(providers.PreferFirst(list, "sarvam")))
	assert.Equal(t, []string{"sarvam", "google", "google-v2"}, names(providers.PreferFirst(list, "nope")))
}

func TestOf_PicksOnlyMatchingCapability(t *testing.T) {
	all := []any{"not a provider", mocks.NewMockTranslator(t), stt(t, "google")}
	assert.Len(t, providers.Of[providers.Translator](all), 1)
	assert.Len(t, providers.Of[providers.SpeechToText](all), 1)
}
