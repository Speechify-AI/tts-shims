// Package openai implements the OpenAI /v1/audio/speech dialect.
package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Speechify-AI/tts-shims/internal/audio"
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/internal/speechify"
	"github.com/Speechify-AI/tts-shims/internal/ssml"
)

// Provider speaks the OpenAI text-to-speech dialect.
type Provider struct{}

// New returns the OpenAI provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "openai" }

// Route is the OpenAI speech endpoint.
func (*Provider) Route() string { return "POST /v1/audio/speech" }

type request struct {
	Model          string   `json:"model"`
	Input          string   `json:"input"`
	Voice          string   `json:"voice"`
	Instructions   string   `json:"instructions,omitempty"`
	ResponseFormat string   `json:"response_format,omitempty"`
	Speed          *float64 `json:"speed,omitempty"`
}

var voiceAliases = map[string]string{
	"alloy": "george", "ash": "george", "ballad": "george", "coral": "george",
	"echo": "george", "fable": "george", "nova": "george", "onyx": "george",
	"sage": "george", "shimmer": "george", "verse": "george", "marin": "george", "cedar": "george",
}

var modelAliases = map[string]string{
	"tts-1": "simba-english", "tts-1-hd": "simba-english", "gpt-4o-mini-tts": "simba-english",
}

// UpstreamAuth uses the server key when set; otherwise forwards the caller's
// Authorization header (pass-through mode).
func (*Provider) UpstreamAuth(r *http.Request, serverKey string) speechify.Auth {
	if serverKey != "" {
		return speechify.Auth{Bearer: serverKey}
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return speechify.Auth{Bearer: strings.TrimPrefix(auth, "Bearer ")}
	}
	return speechify.Auth{}
}

// Translate maps the OpenAI request onto Speechify's stream backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if strings.TrimSpace(in.Input) == "" {
		return shim.Translated{}, fmt.Errorf("input is required")
	}

	format, err := resolveFormat(in.ResponseFormat)
	if err != nil {
		return shim.Translated{}, err
	}

	return shim.Translated{
		Request: speechify.Request{
			Input:        ssml.WrapSpeed(in.Input, in.Speed),
			VoiceID:      resolveVoice(in.Voice, def),
			Model:        resolveModel(in.Model, def.Model),
			OutputFormat: format.OutputFormat,
		},
		Format:  format,
		Backend: shim.BackendStream,
	}, nil
}

// RenderSpeech is unused: OpenAI uses the stream backend.
func (*Provider) RenderSpeech(string, shim.Translated) (string, []byte, error) {
	return "", nil, fmt.Errorf("openai does not use the speech backend")
}

// WriteError renders an OpenAI-shaped error envelope.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"message": message, "type": errType(status)},
	})
}

func resolveVoice(voice string, _ shim.Defaults) string {
	if voice == "" {
		return "george"
	}
	if id, ok := voiceAliases[strings.ToLower(voice)]; ok {
		return id
	}
	return voice
}

func resolveModel(model, fallback string) string {
	if model == "" {
		return fallback
	}
	if id, ok := modelAliases[strings.ToLower(model)]; ok {
		return id
	}
	return model
}

// resolveFormat maps OpenAI response_format onto a Speechify format. opus routes
// to Speechify's Opus-in-ogg; flac and wav are served as PCM wrapped in a
// streaming WAV header (Speechify's stream endpoint rejects wav_*).
func resolveFormat(rf string) (audio.Format, error) {
	switch strings.ToLower(rf) {
	case "", "mp3":
		return audio.MP3(24000, 128), nil
	case "opus":
		return audio.Ogg(), nil
	case "aac":
		return audio.AAC(), nil
	case "wav", "flac":
		return audio.WAV(24000), nil
	case "pcm":
		return audio.PCM(24000), nil
	default:
		return audio.Format{}, fmt.Errorf("unsupported response_format %q", rf)
	}
}

func errType(status int) string {
	if status >= 400 && status < 500 {
		return "invalid_request_error"
	}
	return "api_error"
}
