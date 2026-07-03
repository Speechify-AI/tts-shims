// Package speechify is the upstream client for the Speechify TTS API. It exposes
// the two synthesis surfaces the shims target: Stream (raw chunked audio, lowest
// latency) and Speech (a single JSON response carrying base64 audio, used to
// re-wrap into providers whose own response is base64-in-JSON).
package speechify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Request is the Speechify synthesis request body shared by both endpoints.
type Request struct {
	Input        string   `json:"input"`
	VoiceID      string   `json:"voice_id"`
	Model        string   `json:"model,omitempty"`
	OutputFormat string   `json:"output_format,omitempty"`
	AudioFormat  string   `json:"audio_format,omitempty"`
	Language     string   `json:"language,omitempty"`
	Options      *Options `json:"options,omitempty"`
}

// Options mirrors Speechify's synthesis "options" object.
type Options struct {
	LoudnessNormalization bool `json:"loudness_normalization,omitempty"`
	TextNormalization     bool `json:"text_normalization,omitempty"`
}

// SpeechResponse is the JSON body returned by POST /v1/audio/speech.
type SpeechResponse struct {
	AudioData    string `json:"audio_data"`
	AudioFormat  string `json:"audio_format"`
	OutputFormat string `json:"output_format"`
}

// Client calls the Speechify API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client for the given base URL (no trailing slash) using the
// supplied HTTP client. The HTTP client must not set a response timeout, so
// streaming bodies are bounded by the caller's context instead of truncated.
func New(baseURL string, hc *http.Client) *Client {
	return &Client{baseURL: baseURL, http: hc}
}

// Stream calls POST /v1/audio/stream and returns the raw audio response. The
// caller owns resp.Body and must close it. accept is sent as the Accept header
// to negotiate the container when output_format does not fully determine it.
func (c *Client) Stream(ctx context.Context, req Request, accept string, auth Auth, version string) (*http.Response, error) {
	return c.do(ctx, "/v1/audio/stream", req, accept, auth, version)
}

// Speech calls POST /v1/audio/speech and decodes the JSON response carrying
// base64 audio.
func (c *Client) Speech(ctx context.Context, req Request, auth Auth, version string) (*SpeechResponse, error) {
	resp, err := c.do(ctx, "/v1/audio/speech", req, "application/json", auth, version)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &UpstreamError{Status: resp.StatusCode, Body: readErr(resp.Body)}
	}
	var out SpeechResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode speech response: %w", err)
	}
	return &out, nil
}

func (c *Client) do(ctx context.Context, path string, req Request, accept string, auth Auth, version string) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", accept)
	if auth.Bearer != "" {
		httpReq.Header.Set("Authorization", "Bearer "+auth.Bearer)
	}
	if version != "" {
		httpReq.Header.Set("Speechify-Version", version)
	}
	return c.http.Do(httpReq)
}

// Auth carries the upstream Speechify credential.
type Auth struct {
	Bearer string
}

// UpstreamError is returned when Speechify responds with a non-2xx status on the
// JSON Speech endpoint.
type UpstreamError struct {
	Status int
	Body   ErrorEnvelope
}

func (e *UpstreamError) Error() string {
	if e.Body.Error.Message != "" {
		return e.Body.Error.Message
	}
	return http.StatusText(e.Status)
}

// ErrorEnvelope is Speechify's standard error body.
type ErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func readErr(r io.Reader) ErrorEnvelope {
	var e ErrorEnvelope
	data, _ := io.ReadAll(io.LimitReader(r, 64*1024))
	_ = json.Unmarshal(data, &e)
	return e
}
