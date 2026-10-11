package sarvam

import (
	"context"
	"dictation/internal/providers"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslate_RequestShapeAndAuth(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("api-subscription-key")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"translated_text":      "Keep the documents ready",
			"source_language_code": "hi-IN",
		})
	}))
	defer srv.Close()

	tr := NewTranslator(Options{APIKey: "test-key", BaseURL: srv.URL})

	out, err := tr.Translate(context.Background(), "दस्तावेज़ तैयार रखें", "hi-IN", "en-IN")
	require.NoError(t, err)

	assert.Equal(t, "Keep the documents ready", out)
	assert.Equal(t, "/translate", gotPath)
	assert.Equal(t, "test-key", gotAuth)
	assert.Equal(t, "दस्तावेज़ तैयार रखें", gotBody["input"])
	assert.Equal(t, "hi-IN", gotBody["source_language_code"])
	assert.Equal(t, "en-IN", gotBody["target_language_code"])
	assert.Equal(t, "mayura:v1", gotBody["model"])
	assert.Equal(t, "formal", gotBody["mode"])
}

func TestTranslate_Non2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	tr := NewTranslator(Options{APIKey: "test-key", BaseURL: srv.URL})

	_, err := tr.Translate(context.Background(), "hello", "en-IN", "hi-IN")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestTranslate_EmptyTranslationIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"translated_text":      "",
			"source_language_code": "hi-IN",
		})
	}))
	defer srv.Close()

	tr := NewTranslator(Options{APIKey: "test-key", BaseURL: srv.URL})

	_, err := tr.Translate(context.Background(), "hello", "en-IN", "hi-IN")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestTranslate_OversizedErrorBodyIsCapped(t *testing.T) {
	bigBody := strings.Repeat("x", 4096)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(bigBody))
	}))
	defer srv.Close()

	tr := NewTranslator(Options{APIKey: "test-key", BaseURL: srv.URL})

	_, err := tr.Translate(context.Background(), "hello", "en-IN", "hi-IN")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	// Only a capped prefix of the oversized body should appear in the
	// error message, not all 4096 bytes.
	assert.Less(t, len(err.Error()), len(bigBody))
}

func TestSupports(t *testing.T) {
	tr := NewTranslator(Options{APIKey: "test-key"})

	assert.Equal(t, "sarvam", tr.Name())

	cases := []struct {
		src, target string
		want        bool
	}{
		{"hi-IN", "en-IN", true},
		{"mr-IN", "kn-IN", true},
		{"hi-IN", "hi-IN", false},
		{"ta-IN", "en-IN", false},
		{"auto", "en-IN", true},
		{"en-IN", "auto", false},
		{"auto", "fr-FR", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, tr.Supports(providers.Language(c.src), providers.Language(c.target)), "%s -> %s", c.src, c.target)
	}
}

func TestTranslate_AutoSourceIsSent(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"translated_text": "hello", "source_language_code": "hi-IN"})
	}))
	defer srv.Close()

	tr := NewTranslator(Options{APIKey: "test-key", BaseURL: srv.URL})
	out, err := tr.Translate(context.Background(), "नमस्ते", "auto", "en-IN")
	require.NoError(t, err)
	assert.Equal(t, "hello", out)
	assert.Equal(t, "auto", gotBody["source_language_code"])
}
