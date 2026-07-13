// Package vapi implements Vapi's custom-voice TTS webhook dialect.
package vapi

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Speechify-AI/tts-shims/internal/audio"
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/internal/speechify"
)

const (
	defaultVoice = "geffen_32"
	defaultModel = "simba-3.2"
)

// Provider speaks Vapi's custom-voice request shape.
type Provider struct{}

// New returns the Vapi custom-voice provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "vapi" }

// Route is the Vapi custom-voice endpoint. Configure Vapi's voice.server.url to
// the deployed base URL plus /synthesize.
func (*Provider) Route() string { return "POST /synthesize" }

type request struct {
	Message message `json:"message"`
}

type message struct {
	Type       string `json:"type"`
	Text       string `json:"text"`
	SampleRate int    `json:"sampleRate"`
}

// UpstreamAuth uses only the server key. Vapi's X-VAPI-SECRET authenticates the
// caller to the shim, but it is not a Speechify credential.
func (*Provider) UpstreamAuth(_ *http.Request, serverKey string) speechify.Auth {
	if serverKey != "" {
		return speechify.Auth{Bearer: serverKey}
	}
	return speechify.Auth{}
}

// Translate maps Vapi's voice-request message onto Speechify's stream backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	if err := validateSecret(r); err != nil {
		return shim.Translated{}, err
	}

	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if in.Message.Type != "voice-request" {
		return shim.Translated{}, fmt.Errorf("message.type must be voice-request")
	}
	text := strings.TrimSpace(in.Message.Text)
	if text == "" {
		return shim.Translated{}, fmt.Errorf("message.text is required")
	}

	format, err := resolveFormat(in.Message.SampleRate)
	if err != nil {
		return shim.Translated{}, err
	}

	return shim.Translated{
		Request: speechify.Request{
			Input:        text,
			VoiceID:      defaultVoice,
			Model:        resolveModel(def.Model),
			OutputFormat: format.OutputFormat,
		},
		Format:  format,
		Backend: shim.BackendStream,
	}, nil
}

// RenderSpeech is unused: Vapi uses the stream backend.
func (*Provider) RenderSpeech(string, shim.Translated) (string, []byte, error) {
	return "", nil, fmt.Errorf("vapi does not use the speech backend")
}

// WriteError renders a compact JSON error envelope for Vapi/custom-voice calls.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"message": message, "type": errType(status)},
	})
}

func resolveFormat(sampleRate int) (audio.Format, error) {
	switch sampleRate {
	case 8000, 16000, 22050, 24000:
		return audio.Format{
			OutputFormat: fmt.Sprintf("pcm_%d", sampleRate),
			ContentType:  "application/octet-stream",
			Rate:         sampleRate,
		}, nil
	case 0:
		return audio.Format{}, fmt.Errorf("message.sampleRate is required")
	default:
		return audio.Format{}, fmt.Errorf("unsupported message.sampleRate %d", sampleRate)
	}
}

func resolveModel(model string) string {
	if model == "" || model == "simba-english" {
		return defaultModel
	}
	return model
}

func validateSecret(r *http.Request) error {
	want := os.Getenv("SHIM_VAPI_SECRET")
	if want == "" {
		want = os.Getenv("VAPI_SECRET")
	}
	if want == "" {
		return nil
	}
	got := r.Header.Get("X-VAPI-SECRET")
	if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return fmt.Errorf("invalid X-VAPI-SECRET")
	}
	return nil
}

func errType(status int) string {
	if status >= 400 && status < 500 {
		return "invalid_request_error"
	}
	return "api_error"
}
