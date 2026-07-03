// Package fish implements the Fish POST /v1/tts dialect.
package fish

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

// Provider speaks the Fish text-to-speech dialect.
type Provider struct{}

// New returns the Fish provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "fish" }

// Route is the Fish TTS endpoint.
func (*Provider) Route() string { return "POST /v1/tts" }

type request struct {
	Text       string `json:"text"`
	Reference  string `json:"reference_id"`
	Format     string `json:"format"`
	MP3Bitrate int    `json:"mp3_bitrate"`
	SampleRate int    `json:"sample_rate"`
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

// Translate maps the Fish request onto Speechify's stream backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if strings.TrimSpace(in.Text) == "" {
		return shim.Translated{}, fmt.Errorf("text is required")
	}

	format, err := resolveFormat(in.Format, in.SampleRate, in.MP3Bitrate)
	if err != nil {
		return shim.Translated{}, err
	}

	return shim.Translated{
		Request: speechify.Request{
			Input:        ssml.WrapSpeed(in.Text, nil),
			VoiceID:      resolveVoice(in.Reference),
			Model:        def.Model,
			OutputFormat: format.OutputFormat,
		},
		Format:  format,
		Backend: shim.BackendStream,
	}, nil
}

// RenderSpeech is unused: Fish uses the stream backend.
func (*Provider) RenderSpeech(string, shim.Translated) (string, []byte, error) {
	return "", nil, fmt.Errorf("fish does not use the speech backend")
}

// WriteError renders a Fish-shaped error envelope.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
}

func resolveVoice(referenceID string) string {
	if referenceID == "" {
		return "george"
	}
	return referenceID
}

func resolveFormat(format string, sampleRate, mp3Bitrate int) (audio.Format, error) {
	switch strings.ToLower(format) {
	case "":
		return audio.MP3(24000, 128), nil
	case "mp3":
		return audio.MP3(sampleRate, bitrateKbps(mp3Bitrate)), nil
	case "wav":
		return audio.WAV(sampleRate), nil
	case "pcm":
		return audio.PCM(sampleRate), nil
	case "opus":
		return audio.Ogg(), nil
	default:
		return audio.Format{}, fmt.Errorf("unsupported format %q", format)
	}
}

func bitrateKbps(bitrate int) int {
	if bitrate <= 0 {
		return 128
	}
	return bitrate / 1000
}
