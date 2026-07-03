// Package hume implements the Hume POST /v0/tts/file dialect.
package hume

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

// Provider speaks the Hume text-to-speech dialect.
type Provider struct{}

// New returns the Hume provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "hume" }

// Route is the Hume TTS file endpoint.
func (*Provider) Route() string { return "POST /v0/tts/file" }

type request struct {
	Utterances []utterance  `json:"utterances"`
	Format     *formatField `json:"format"`
}

type utterance struct {
	Text        string      `json:"text"`
	Description string      `json:"description,omitempty"`
	Speed       *float64    `json:"speed"`
	Voice       *voiceField `json:"voice"`
}

type voiceField struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type formatField struct {
	Type string `json:"type"`
}

// UpstreamAuth uses the server key when set; otherwise forwards X-Hume-Api-Key.
func (*Provider) UpstreamAuth(r *http.Request, serverKey string) speechify.Auth {
	if serverKey != "" {
		return speechify.Auth{Bearer: serverKey}
	}
	if key := r.Header.Get("X-Hume-Api-Key"); key != "" {
		return speechify.Auth{Bearer: key}
	}
	return speechify.Auth{}
}

// Translate maps the Hume request onto Speechify's stream backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if len(in.Utterances) == 0 {
		return shim.Translated{}, fmt.Errorf("utterances is required")
	}

	text, err := joinedText(in.Utterances)
	if err != nil {
		return shim.Translated{}, err
	}
	format, err := resolveFormat(in.Format)
	if err != nil {
		return shim.Translated{}, err
	}

	return shim.Translated{
		Request: speechify.Request{
			Input:        ssml.WrapSpeed(text, in.Utterances[0].Speed),
			VoiceID:      resolveVoice(in.Utterances[0].Voice),
			Model:        def.Model,
			OutputFormat: format.OutputFormat,
		},
		Format:  format,
		Backend: shim.BackendStream,
	}, nil
}

// RenderSpeech is unused: Hume uses the stream backend.
func (*Provider) RenderSpeech(string, shim.Translated) (string, []byte, error) {
	return "", nil, fmt.Errorf("hume does not use the speech backend")
}

// WriteError renders a Hume-shaped error envelope.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"message": message})
}

func joinedText(utterances []utterance) (string, error) {
	parts := make([]string, 0, len(utterances))
	for _, u := range utterances {
		text := strings.TrimSpace(u.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("utterances.text is required")
	}
	return strings.Join(parts, " "), nil
}

func resolveVoice(voice *voiceField) string {
	if voice == nil {
		return "george"
	}
	if voice.ID != "" {
		return voice.ID
	}
	if voice.Name != "" {
		return voice.Name
	}
	return "george"
}

func resolveFormat(format *formatField) (audio.Format, error) {
	if format == nil {
		return audio.MP3(24000, 128), nil
	}
	switch strings.ToLower(format.Type) {
	case "", "mp3":
		return audio.MP3(24000, 128), nil
	case "wav":
		return audio.WAV(24000), nil
	case "pcm":
		return audio.PCM(24000), nil
	default:
		return audio.Format{}, fmt.Errorf("unsupported format.type %q", format.Type)
	}
}
