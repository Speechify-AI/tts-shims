// Package elevenlabs implements the ElevenLabs POST /v1/text-to-speech/{voice_id}
// dialect: voice_id in the path, output_format in the query, xi-api-key auth.
package elevenlabs

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

// Provider speaks the ElevenLabs text-to-speech dialect.
type Provider struct{}

// New returns the ElevenLabs provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "elevenlabs" }

// Route is the ElevenLabs speech endpoint with a path voice_id.
func (*Provider) Route() string { return "POST /v1/text-to-speech/{voice_id}" }

type request struct {
	Text          string `json:"text"`
	ModelID       string `json:"model_id,omitempty"`
	LanguageCode  string `json:"language_code,omitempty"`
	VoiceSettings *struct {
		Speed *float64 `json:"speed,omitempty"`
	} `json:"voice_settings,omitempty"`
}

var modelAliases = map[string]string{
	"eleven_v3": "simba-english", "eleven_multilingual_v2": "simba-multilingual",
	"eleven_multilingual_v1": "simba-multilingual", "eleven_flash_v2_5": "simba-english",
	"eleven_flash_v2": "simba-english", "eleven_turbo_v2_5": "simba-english",
	"eleven_turbo_v2": "simba-english", "eleven_monolingual_v1": "simba-english",
}

// UpstreamAuth uses the server key when set; otherwise forwards the caller's
// xi-api-key as a Speechify bearer token (pass-through mode). The server key
// wins so BYOC placeholder keys cannot override it.
func (*Provider) UpstreamAuth(r *http.Request, serverKey string) speechify.Auth {
	if serverKey != "" {
		return speechify.Auth{Bearer: serverKey}
	}
	if key := r.Header.Get("xi-api-key"); key != "" {
		return speechify.Auth{Bearer: key}
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return speechify.Auth{Bearer: strings.TrimPrefix(auth, "Bearer ")}
	}
	return speechify.Auth{}
}

// Translate maps the ElevenLabs request onto Speechify's stream backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	voiceID := r.PathValue("voice_id")
	if strings.TrimSpace(voiceID) == "" {
		return shim.Translated{}, fmt.Errorf("voice_id path parameter is required")
	}
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if strings.TrimSpace(in.Text) == "" {
		return shim.Translated{}, fmt.Errorf("text is required")
	}

	format, err := resolveFormat(r.URL.Query().Get("output_format"))
	if err != nil {
		return shim.Translated{}, err
	}

	var speed *float64
	if in.VoiceSettings != nil {
		speed = in.VoiceSettings.Speed
	}

	return shim.Translated{
		Request: speechify.Request{
			Input:        ssml.WrapSpeed(in.Text, speed),
			VoiceID:      voiceID,
			Model:        resolveModel(in.ModelID, def.Model),
			OutputFormat: format.OutputFormat,
			Language:     in.LanguageCode,
		},
		Format:  format,
		Backend: shim.BackendStream,
	}, nil
}

// RenderSpeech is unused: ElevenLabs uses the stream backend.
func (*Provider) RenderSpeech(string, shim.Translated) (string, []byte, error) {
	return "", nil, fmt.Errorf("elevenlabs does not use the speech backend")
}

// WriteError renders an ElevenLabs-shaped {"detail":{"message":...}} error.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"detail": map[string]any{"message": message},
	})
}

func resolveModel(model, fallback string) string {
	if model == "" {
		return fallback
	}
	if id, ok := modelAliases[strings.ToLower(model)]; ok {
		return id
	}
	return fallback
}

// resolveFormat maps ElevenLabs output_format (codec_sampleRate[_bitrate]) onto a
// Speechify format. 44.1 kHz mp3 collapses onto Speechify's 24 kHz tiers, opus
// routes to ogg, alaw degrades to ulaw, and wav_* is served as PCM wrapped in a
// streaming WAV header.
func resolveFormat(of string) (audio.Format, error) {
	of = strings.ToLower(of)
	if of == "" {
		of = "mp3_44100_128"
	}
	switch {
	case strings.HasPrefix(of, "mp3_"):
		return audio.MP3(sampleRateOf(of, 1), bitrateOf(of)), nil
	case strings.HasPrefix(of, "opus_"):
		return audio.Ogg(), nil
	case strings.HasPrefix(of, "pcm_"):
		return audio.PCM(sampleRateOf(of, 1)), nil
	case strings.HasPrefix(of, "wav_"):
		return audio.WAV(sampleRateOf(of, 1)), nil
	case of == "ulaw_8000", of == "alaw_8000":
		return audio.ULaw(), nil
	default:
		return audio.Format{}, fmt.Errorf("unsupported output_format %q", of)
	}
}

// sampleRateOf extracts the numeric sample rate at underscore-separated index i
// (e.g. "mp3_44100_128" -> 44100 at i=1).
func sampleRateOf(of string, i int) int {
	parts := strings.Split(of, "_")
	if i < len(parts) {
		return atoi(parts[i])
	}
	return 0
}

// bitrateOf extracts the kbps from an ElevenLabs mp3 format string
// (e.g. "mp3_44100_128" -> 128).
func bitrateOf(of string) int {
	parts := strings.Split(of, "_")
	if len(parts) >= 3 {
		return atoi(parts[2])
	}
	return 128
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
