// Package lmnt implements the LMNT POST /v1/ai/speech/bytes dialect.
package lmnt

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

// Provider speaks the LMNT text-to-speech dialect.
type Provider struct{}

// New returns the LMNT provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "lmnt" }

// Route is the LMNT speech bytes endpoint.
func (*Provider) Route() string { return "POST /v1/ai/speech/bytes" }

type request struct {
	Voice      string   `json:"voice"`
	Text       string   `json:"text"`
	Model      string   `json:"model"`
	Format     string   `json:"format"`
	SampleRate int      `json:"sample_rate"`
	Language   string   `json:"language"`
	Speed      *float64 `json:"speed"`
}

// UpstreamAuth uses the server key when set; otherwise forwards X-API-Key.
func (*Provider) UpstreamAuth(r *http.Request, serverKey string) speechify.Auth {
	if serverKey != "" {
		return speechify.Auth{Bearer: serverKey}
	}
	if key := r.Header.Get("X-API-Key"); key != "" {
		return speechify.Auth{Bearer: key}
	}
	return speechify.Auth{}
}

// Translate maps the LMNT request onto Speechify's stream backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if strings.TrimSpace(in.Text) == "" {
		return shim.Translated{}, fmt.Errorf("text is required")
	}
	if strings.TrimSpace(in.Voice) == "" {
		return shim.Translated{}, fmt.Errorf("voice is required")
	}

	format, err := resolveFormat(in.Format, in.SampleRate)
	if err != nil {
		return shim.Translated{}, err
	}

	return shim.Translated{
		Request: speechify.Request{
			Input:        ssml.WrapSpeed(in.Text, in.Speed),
			VoiceID:      in.Voice,
			Model:        resolveModel(in.Model, def.Model),
			OutputFormat: format.OutputFormat,
			Language:     in.Language,
		},
		Format:  format,
		Backend: shim.BackendStream,
	}, nil
}

// RenderSpeech is unused: LMNT uses the stream backend.
func (*Provider) RenderSpeech(string, shim.Translated) (string, []byte, error) {
	return "", nil, fmt.Errorf("lmnt does not use the speech backend")
}

// WriteError renders an LMNT-shaped error envelope.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
}

func resolveFormat(format string, sampleRate int) (audio.Format, error) {
	switch strings.ToLower(format) {
	case "":
		return audio.MP3(24000, 128), nil
	case "mp3":
		return audio.MP3(sampleRate, 128), nil
	case "wav":
		return audio.WAV(sampleRate), nil
	case "aac":
		return audio.AAC(), nil
	case "pcm_s16le", "raw":
		return audio.PCM(sampleRate), nil
	case "ulaw":
		return audio.ULaw(), nil
	case "webm":
		return audio.Ogg(), nil
	default:
		return audio.Format{}, fmt.Errorf("unsupported format %q", format)
	}
}

func resolveModel(model, fallback string) string {
	if model == "" {
		return fallback
	}
	return model
}
