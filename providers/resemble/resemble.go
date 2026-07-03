// Package resemble implements the Resemble AI POST /synthesize dialect, whose
// response carries base64 audio in audio_content. It uses the Speechify Speech
// backend and re-wraps the base64.
package resemble

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Speechify-AI/tts-shims/internal/audio"
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/internal/speechify"
)

// Provider speaks the Resemble AI synthesize dialect.
type Provider struct{}

// New returns the Resemble provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "resemble" }

// Route is the Resemble synthesize endpoint.
func (*Provider) Route() string { return "POST /synthesize" }

type request struct {
	VoiceUUID    string `json:"voice_uuid"`
	Data         string `json:"data"`
	OutputFormat string `json:"output_format"`
	SampleRate   int    `json:"sample_rate"`
}

// UpstreamAuth injects the configured Speechify key; the caller's Resemble
// bearer authenticates to Resemble, not Speechify, and is never reused.
func (*Provider) UpstreamAuth(_ *http.Request, serverKey string) speechify.Auth {
	return speechify.Auth{Bearer: serverKey}
}

// Translate maps the Resemble request onto Speechify's speech (base64) backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if strings.TrimSpace(in.Data) == "" {
		return shim.Translated{}, fmt.Errorf("data is required")
	}
	if strings.TrimSpace(in.VoiceUUID) == "" {
		return shim.Translated{}, fmt.Errorf("voice_uuid is required")
	}

	format, af := resolveFormat(in.OutputFormat, in.SampleRate)

	return shim.Translated{
		Request: speechify.Request{
			Input:       in.Data,
			VoiceID:     in.VoiceUUID,
			Model:       def.Model,
			AudioFormat: af,
		},
		Format:  format,
		Backend: shim.BackendSpeech,
	}, nil
}

// RenderSpeech wraps the base64 audio in Resemble's {"audio_content": "..."}
// envelope; Speechify's base64 is passed through unchanged.
func (*Provider) RenderSpeech(audioBase64 string, _ shim.Translated) (string, []byte, error) {
	body, err := json.Marshal(map[string]any{"audio_content": audioBase64, "success": true})
	if err != nil {
		return "", nil, err
	}
	return "application/json", body, nil
}

// WriteError renders a Resemble-style {"success":false,"message":...} error.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": message})
}

// resolveFormat returns the advertised Format and the Speechify speech
// audio_format that determines the codec inside the base64 payload. Resemble's
// synchronous endpoint is WAV-centric, so an unspecified format yields wav.
func resolveFormat(of string, rate int) (audio.Format, string) {
	switch strings.ToLower(of) {
	case "mp3":
		return audio.MP3(rate, 128), "mp3"
	case "pcm":
		return audio.PCM(rate), "pcm"
	default:
		return audio.WAV(rate), "wav"
	}
}
