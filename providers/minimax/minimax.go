// Package minimax implements the MiniMax T2A v2 POST /v1/t2a_v2 dialect, whose
// response carries the audio as a HEX string in data.audio. It uses the
// Speechify Speech backend and transcodes Speechify's base64 to hex.
package minimax

import (
	"encoding/base64"
	"encoding/hex"
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

// Provider speaks the MiniMax T2A v2 dialect.
type Provider struct{}

// New returns the MiniMax provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "minimax" }

// Route is the MiniMax T2A v2 endpoint.
func (*Provider) Route() string { return "POST /v1/t2a_v2" }

type request struct {
	Model        string `json:"model"`
	Text         string `json:"text"`
	VoiceSetting struct {
		VoiceID string   `json:"voice_id"`
		Speed   *float64 `json:"speed"`
	} `json:"voice_setting"`
	AudioSetting struct {
		SampleRate int    `json:"sample_rate"`
		Bitrate    int    `json:"bitrate"`
		Format     string `json:"format"`
	} `json:"audio_setting"`
	LanguageBoost string `json:"language_boost,omitempty"`
}

// UpstreamAuth injects the configured Speechify key; the caller's MiniMax bearer
// authenticates to MiniMax, not Speechify, and is never reused.
func (*Provider) UpstreamAuth(_ *http.Request, serverKey string) speechify.Auth {
	return speechify.Auth{Bearer: serverKey}
}

// Translate maps the MiniMax request onto Speechify's speech (base64) backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if strings.TrimSpace(in.Text) == "" {
		return shim.Translated{}, fmt.Errorf("text is required")
	}

	format := resolveFormat(in.AudioSetting.Format)

	return shim.Translated{
		Request: speechify.Request{
			Input:       ssml.WrapSpeed(in.Text, in.VoiceSetting.Speed),
			VoiceID:     voiceID(in.VoiceSetting.VoiceID),
			Model:       def.Model,
			AudioFormat: format.speechifyAudioFormat,
			Language:    in.LanguageBoost,
		},
		Format:  format.format,
		Backend: shim.BackendSpeech,
	}, nil
}

// RenderSpeech transcodes Speechify's base64 audio to the hex string MiniMax
// clients expect in data.audio, then wraps it in the T2A v2 response envelope.
func (*Provider) RenderSpeech(audioBase64 string, _ shim.Translated) (string, []byte, error) {
	raw, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		return "", nil, fmt.Errorf("decode upstream audio: %w", err)
	}
	body, err := json.Marshal(map[string]any{
		"data": map[string]any{
			"audio":  hex.EncodeToString(raw),
			"status": 2,
		},
		"base_resp": map[string]any{"status_code": 0, "status_msg": "success"},
	})
	if err != nil {
		return "", nil, err
	}
	return "application/json", body, nil
}

// WriteError renders a MiniMax-style {"base_resp":{"status_code","status_msg"}}.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"base_resp": map[string]any{"status_code": 2013, "status_msg": message},
	})
}

func voiceID(id string) string {
	if id == "" {
		return "george"
	}
	return id
}

type minimaxFormat struct {
	format               audio.Format
	speechifyAudioFormat string
}

// resolveFormat maps MiniMax audio_setting.format onto the Speechify speech
// endpoint audio_format that determines the codec inside the hex payload.
func resolveFormat(f string) minimaxFormat {
	switch strings.ToLower(f) {
	case "pcm":
		return minimaxFormat{audio.PCM(24000), "pcm"}
	case "flac", "wav":
		return minimaxFormat{audio.WAV(24000), "wav"}
	default:
		return minimaxFormat{audio.MP3(24000, 128), "mp3"}
	}
}
