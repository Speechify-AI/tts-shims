// Package inworld implements the Inworld TTS POST /tts/v1/voice dialect, whose
// response carries base64 audio in audioContent. It uses the Speechify Speech
// backend and re-wraps the base64.
package inworld

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

// Provider speaks the Inworld TTS dialect.
type Provider struct{}

// New returns the Inworld provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "inworld" }

// Route is the Inworld synthesize endpoint.
func (*Provider) Route() string { return "POST /tts/v1/voice" }

type request struct {
	Text        string `json:"text"`
	VoiceID     string `json:"voiceId"`
	ModelID     string `json:"modelId"`
	AudioConfig struct {
		AudioEncoding   string `json:"audioEncoding"`
		SampleRateHertz int    `json:"sampleRateHertz"`
	} `json:"audioConfig"`
	Language string `json:"language"`
}

// UpstreamAuth injects the configured Speechify key; the caller's Inworld Basic
// credential authenticates to Inworld, not Speechify, and is never reused.
func (*Provider) UpstreamAuth(_ *http.Request, serverKey string) speechify.Auth {
	return speechify.Auth{Bearer: serverKey}
}

// Translate maps the Inworld request onto Speechify's speech (base64) backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if strings.TrimSpace(in.Text) == "" {
		return shim.Translated{}, fmt.Errorf("text is required")
	}

	format, af := resolveFormat(in.AudioConfig.AudioEncoding, in.AudioConfig.SampleRateHertz)

	return shim.Translated{
		Request: speechify.Request{
			Input:       in.Text,
			VoiceID:     voiceID(in.VoiceID),
			Model:       def.Model,
			AudioFormat: af,
			Language:    in.Language,
		},
		Format:  format,
		Backend: shim.BackendSpeech,
	}, nil
}

// RenderSpeech wraps the base64 audio in Inworld's {"audioContent": "..."}
// envelope; Speechify's base64 is passed through unchanged.
func (*Provider) RenderSpeech(audioBase64 string, _ shim.Translated) (string, []byte, error) {
	body, err := json.Marshal(map[string]string{"audioContent": audioBase64})
	if err != nil {
		return "", nil, err
	}
	return "application/json", body, nil
}

// WriteError renders an Inworld-style {"code","message"} error.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": status, "message": message})
}

func voiceID(id string) string {
	if id == "" {
		return "george"
	}
	return id
}

// resolveFormat returns the advertised Format and the Speechify speech
// audio_format that determines the codec inside the base64 payload.
func resolveFormat(enc string, rate int) (audio.Format, string) {
	switch strings.ToUpper(enc) {
	case "MP3":
		return audio.MP3(24000, 128), "mp3"
	case "OGG_OPUS":
		return audio.Ogg(), "ogg"
	case "MULAW", "ALAW":
		return audio.PCM(rate), "pcm"
	default:
		return audio.WAV(rate), "wav"
	}
}
