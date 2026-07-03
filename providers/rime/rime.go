// Package rime implements the Rime POST /v1/rime-tts dialect.
package rime

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

// Provider speaks the Rime text-to-speech dialect.
type Provider struct{}

// New returns the Rime provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "rime" }

// Route is the Rime TTS endpoint.
func (*Provider) Route() string { return "POST /v1/rime-tts" }

type request struct {
	Speaker      string   `json:"speaker"`
	Text         string   `json:"text"`
	ModelID      string   `json:"modelId"`
	Lang         string   `json:"lang"`
	SamplingRate int      `json:"samplingRate"`
	SpeedAlpha   *float64 `json:"speedAlpha"`
}

// UpstreamAuth uses the server key when set; otherwise forwards Bearer auth.
func (*Provider) UpstreamAuth(r *http.Request, serverKey string) speechify.Auth {
	if serverKey != "" {
		return speechify.Auth{Bearer: serverKey}
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return speechify.Auth{Bearer: strings.TrimPrefix(auth, "Bearer ")}
	}
	return speechify.Auth{}
}

// Translate maps the Rime request onto Speechify's stream backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if strings.TrimSpace(in.Text) == "" {
		return shim.Translated{}, fmt.Errorf("text is required")
	}
	if strings.TrimSpace(in.Speaker) == "" {
		return shim.Translated{}, fmt.Errorf("speaker is required")
	}

	format := resolveFormat(r.Header.Get("Accept"), in.SamplingRate)

	return shim.Translated{
		Request: speechify.Request{
			Input:        ssml.WrapSpeed(in.Text, in.SpeedAlpha),
			VoiceID:      in.Speaker,
			Model:        resolveModel(in.ModelID, def.Model),
			OutputFormat: format.OutputFormat,
			Language:     in.Lang,
		},
		Format:  format,
		Backend: shim.BackendStream,
	}, nil
}

// RenderSpeech is unused: Rime uses the stream backend.
func (*Provider) RenderSpeech(string, shim.Translated) (string, []byte, error) {
	return "", nil, fmt.Errorf("rime does not use the speech backend")
}

// WriteError renders a Rime-shaped error envelope.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
}

func resolveFormat(accept string, sampleRate int) audio.Format {
	switch strings.ToLower(accept) {
	case "audio/mpeg":
		return audio.MP3(sampleRate, 128)
	case "audio/wav":
		return audio.WAV(sampleRate)
	case "audio/pcm", "audio/l16":
		return audio.PCM(sampleRate)
	case "audio/ogg":
		return audio.Ogg()
	default:
		return audio.MP3(defaultRate(sampleRate), 128)
	}
}

var speechifyModels = map[string]struct{}{
	"simba-english": {}, "simba-multilingual": {}, "simba-3.0": {},
}

// resolveModel passes through native Speechify models and falls back to the
// default for any provider-specific model id (e.g. Rime's "mistv2"), which
// Speechify would otherwise reject.
func resolveModel(model, fallback string) string {
	if model == "" {
		return fallback
	}
	if _, ok := speechifyModels[model]; ok {
		return model
	}
	return fallback
}

func defaultRate(rate int) int {
	if rate <= 0 {
		return 24000
	}
	return rate
}
