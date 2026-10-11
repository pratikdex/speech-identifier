// Package sarvam adapts Sarvam AI's translate and streaming speech-to-text APIs to the providers package's interfaces.
package sarvam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultTimeout is the HTTP client timeout used when Options.HTTP is unset.
const defaultTimeout = 15 * time.Second

// defaultBaseURL is Sarvam's REST API base URL.
const defaultBaseURL = "https://api.sarvam.ai"

// defaultWSURL is Sarvam's streaming API base URL.
const defaultWSURL = "wss://api.sarvam.ai"

// maxErrBody caps how much of a non-2xx response body is included in error
// messages, so a huge error page doesn't flood logs.
const maxErrBody = 512

// Options configures the Sarvam adapters (translate, streaming STT).
type Options struct {
	APIKey  string
	BaseURL string       // default "https://api.sarvam.ai"
	WSURL   string       // default "wss://api.sarvam.ai"
	HTTP    *http.Client // default: 15 s timeout
}

// withDefaults returns o with empty fields filled in with their defaults.
func (o Options) withDefaults() Options {
	if o.BaseURL == "" {
		o.BaseURL = defaultBaseURL
	}
	if o.WSURL == "" {
		o.WSURL = defaultWSURL
	}
	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: defaultTimeout}
	}
	return o
}

// postJSON POSTs body as JSON to o.BaseURL+path, authenticated with
// o.APIKey, and decodes the JSON response into out. A non-2xx response is
// returned as an error that includes the status code and a capped prefix of
// the response body.
func postJSON(ctx context.Context, o Options, path string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("sarvam %s: encoding request: %w", path, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.BaseURL+path, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("sarvam %s: building request: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-subscription-key", o.APIKey)

	resp, err := o.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("sarvam %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited := io.LimitReader(resp.Body, maxErrBody)
		respBody, _ := io.ReadAll(limited)
		return fmt.Errorf("sarvam %s: %d %s", path, resp.StatusCode, respBody)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("sarvam %s: decoding response: %w", path, err)
		}
	}
	return nil
}
