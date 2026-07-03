// Package googletts implements the Google Cloud Text-to-Speech
// POST /v1/text:synthesize dialect, whose response carries base64 audio in
// audioContent. It uses the Speechify Speech backend and re-wraps the base64.
package googletts

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

// Provider speaks the Google Cloud Text-to-Speech dialect.
type Provider struct{}

// New returns the Google TTS provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "google-tts" }

// Route is the Google synthesize endpoint.
func (*Provider) Route() string { return "POST /v1/text:synthesize" }

type request struct {
	Input struct {
		Text string `json:"text"`
		SSML string `json:"ssml"`
	} `json:"input"`
	Voice struct {
		LanguageCode string `json:"languageCode"`
		Name         string `json:"name"`
	} `json:"voice"`
	AudioConfig struct {
		AudioEncoding   string  `json:"audioEncoding"`
		SampleRateHertz int     `json:"sampleRateHertz"`
		SpeakingRate    float64 `json:"speakingRate"`
	} `json:"audioConfig"`
}

// UpstreamAuth injects the configured Speechify key; the caller's Google OAuth
// token authenticates to Google, not Speechify, and is never reused.
func (*Provider) UpstreamAuth(_ *http.Request, serverKey string) speechify.Auth {
	return speechify.Auth{Bearer: serverKey}
}

// Translate maps the Google request onto Speechify's speech (base64) backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	text := in.Input.Text
	if strings.TrimSpace(text) == "" {
		text = in.Input.SSML
	}
	if strings.TrimSpace(text) == "" {
		return shim.Translated{}, fmt.Errorf("input.text or input.ssml is required")
	}

	format, err := resolveFormat(in.AudioConfig.AudioEncoding, in.AudioConfig.SampleRateHertz)
	if err != nil {
		return shim.Translated{}, err
	}

	return shim.Translated{
		Request: speechify.Request{
			Input:       text,
			VoiceID:     voiceID(in.Voice.Name),
			Model:       def.Model,
			AudioFormat: audioFormat(in.AudioConfig.AudioEncoding),
			Language:    in.Voice.LanguageCode,
		},
		Format:  format,
		Backend: shim.BackendSpeech,
	}, nil
}

// RenderSpeech wraps the base64 audio in Google's {"audioContent": "..."}
// envelope. Speechify already returns standard base64, so it is passed through
// unchanged.
func (*Provider) RenderSpeech(audioBase64 string, _ shim.Translated) (string, []byte, error) {
	body, err := json.Marshal(map[string]string{"audioContent": audioBase64})
	if err != nil {
		return "", nil, err
	}
	return "application/json; charset=utf-8", body, nil
}

// WriteError renders a Google-style {"error":{"code","message","status"}}.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": status, "message": message, "status": googleStatus(status)},
	})
}

func voiceID(name string) string {
	if name == "" {
		return "george"
	}
	return name
}

// audioFormat maps Google's audioEncoding onto Speechify's /v1/audio/speech
// audio_format field (wav|mp3|ogg|aac|pcm). It selects the codec the base64
// response body will actually contain.
func audioFormat(enc string) string {
	switch strings.ToUpper(enc) {
	case "", "MP3":
		return "mp3"
	case "OGG_OPUS":
		return "ogg"
	case "LINEAR16", "PCM":
		return "wav"
	case "MULAW", "ALAW":
		return "pcm"
	default:
		return "mp3"
	}
}

// resolveFormat sets the response Content-Type advertised to the caller. Google
// clients read audioContent regardless, so the content type is informational.
func resolveFormat(enc string, rate int) (audio.Format, error) {
	switch strings.ToUpper(enc) {
	case "", "MP3":
		return audio.MP3(24000, 128), nil
	case "OGG_OPUS":
		return audio.Ogg(), nil
	case "LINEAR16", "PCM":
		return audio.WAV(rate), nil
	case "MULAW", "ALAW":
		return audio.PCM(rate), nil
	default:
		return audio.Format{}, fmt.Errorf("unsupported audioEncoding %q", enc)
	}
}

func googleStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "INVALID_ARGUMENT"
	case http.StatusUnauthorized:
		return "UNAUTHENTICATED"
	case http.StatusNotFound:
		return "NOT_FOUND"
	default:
		return "INTERNAL"
	}
}
