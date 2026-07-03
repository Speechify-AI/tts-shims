// Package cartesia implements the Cartesia POST /tts/bytes dialect: transcript,
// a nested voice object, and a nested output_format object.
package cartesia

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

// Provider speaks the Cartesia text-to-speech dialect.
type Provider struct{}

// New returns the Cartesia provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "cartesia" }

// Route is the Cartesia raw-bytes endpoint.
func (*Provider) Route() string { return "POST /tts/bytes" }

type voice struct {
	Mode string `json:"mode"`
	ID   string `json:"id"`
}

type outputFormat struct {
	Container  string `json:"container"`
	Encoding   string `json:"encoding,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
	BitRate    int    `json:"bit_rate,omitempty"`
}

type genConfig struct {
	Speed *float64 `json:"speed,omitempty"`
}

type request struct {
	ModelID          string        `json:"model_id,omitempty"`
	Transcript       string        `json:"transcript"`
	Voice            voice         `json:"voice"`
	OutputFormat     *outputFormat `json:"output_format,omitempty"`
	Language         string        `json:"language,omitempty"`
	GenerationConfig *genConfig    `json:"generation_config,omitempty"`
	Speed            *float64      `json:"speed,omitempty"`
}

// UpstreamAuth uses the server key when set; otherwise forwards the caller's
// X-API-Key (or Bearer) as a Speechify bearer token. The server key wins so
// BYOC placeholder keys cannot override it.
func (*Provider) UpstreamAuth(r *http.Request, serverKey string) speechify.Auth {
	if serverKey != "" {
		return speechify.Auth{Bearer: serverKey}
	}
	if key := r.Header.Get("X-API-Key"); key != "" {
		return speechify.Auth{Bearer: key}
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return speechify.Auth{Bearer: strings.TrimPrefix(auth, "Bearer ")}
	}
	return speechify.Auth{}
}

// Translate maps the Cartesia request onto Speechify's stream backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if strings.TrimSpace(in.Transcript) == "" {
		return shim.Translated{}, fmt.Errorf("transcript is required")
	}
	if strings.TrimSpace(in.Voice.ID) == "" {
		return shim.Translated{}, fmt.Errorf("voice.id is required")
	}

	format, err := resolveFormat(in.OutputFormat)
	if err != nil {
		return shim.Translated{}, err
	}

	var speed *float64
	if in.GenerationConfig != nil && in.GenerationConfig.Speed != nil {
		speed = in.GenerationConfig.Speed
	} else {
		speed = in.Speed
	}

	return shim.Translated{
		Request: speechify.Request{
			Input:        ssml.WrapSpeed(in.Transcript, speed),
			VoiceID:      in.Voice.ID,
			Model:        resolveModel(in.ModelID, def.Model),
			OutputFormat: format.OutputFormat,
			Language:     in.Language,
		},
		Format:  format,
		Backend: shim.BackendStream,
	}, nil
}

// RenderSpeech is unused: Cartesia uses the stream backend.
func (*Provider) RenderSpeech(string, shim.Translated) (string, []byte, error) {
	return "", nil, fmt.Errorf("cartesia does not use the speech backend")
}

// WriteError renders a Cartesia-shaped {"error":...} message.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
}

var speechifyModels = map[string]struct{}{
	"simba-english": {}, "simba-multilingual": {}, "simba-3.0": {},
}

func resolveModel(model, fallback string) string {
	if model == "" {
		return fallback
	}
	if _, ok := speechifyModels[model]; ok {
		return model
	}
	return fallback
}

// resolveFormat collapses Cartesia's nested output_format object onto a Speechify
// format. A "wav" container is served as PCM wrapped in a streaming WAV header
// (Speechify's stream endpoint rejects wav_*); pcm_mulaw maps to ulaw; other raw
// PCM encodings pass through at the nearest supported rate.
func resolveFormat(of *outputFormat) (audio.Format, error) {
	if of == nil {
		return audio.MP3(24000, 128), nil
	}
	switch strings.ToLower(of.Container) {
	case "mp3":
		return audio.MP3(of.SampleRate, of.BitRate/1000), nil
	case "raw":
		if strings.EqualFold(of.Encoding, "pcm_mulaw") {
			return audio.ULaw(), nil
		}
		return audio.PCM(of.SampleRate), nil
	case "wav":
		return audio.WAV(of.SampleRate), nil
	default:
		return audio.Format{}, fmt.Errorf("unsupported output_format.container %q", of.Container)
	}
}
