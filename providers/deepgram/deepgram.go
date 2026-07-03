// Package deepgram implements the Deepgram POST /v1/speak dialect.
package deepgram

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Speechify-AI/tts-shims/internal/audio"
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/internal/speechify"
	"github.com/Speechify-AI/tts-shims/internal/ssml"
)

// Provider speaks the Deepgram text-to-speech dialect.
type Provider struct{}

// New returns the Deepgram provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "deepgram" }

// Route is the Deepgram speak endpoint.
func (*Provider) Route() string { return "POST /v1/speak" }

type request struct {
	Text string `json:"text"`
}

// UpstreamAuth uses the server key when set; otherwise forwards a Token header.
func (*Provider) UpstreamAuth(r *http.Request, serverKey string) speechify.Auth {
	if serverKey != "" {
		return speechify.Auth{Bearer: serverKey}
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Token ") {
		return speechify.Auth{Bearer: strings.TrimPrefix(auth, "Token ")}
	}
	return speechify.Auth{}
}

// Translate maps the Deepgram request onto Speechify's stream backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if strings.TrimSpace(in.Text) == "" {
		return shim.Translated{}, fmt.Errorf("text is required")
	}

	format, err := resolveFormat(r)
	if err != nil {
		return shim.Translated{}, err
	}

	return shim.Translated{
		Request: speechify.Request{
			Input:        ssml.WrapSpeed(in.Text, nil),
			VoiceID:      "george",
			Model:        def.Model,
			OutputFormat: format.OutputFormat,
		},
		Format:  format,
		Backend: shim.BackendStream,
	}, nil
}

// RenderSpeech is unused: Deepgram uses the stream backend.
func (*Provider) RenderSpeech(string, shim.Translated) (string, []byte, error) {
	return "", nil, fmt.Errorf("deepgram does not use the speech backend")
}

// WriteError renders a Deepgram-shaped error envelope.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"err_code": errCode(status), "err_msg": message})
}

func resolveFormat(r *http.Request) (audio.Format, error) {
	q := r.URL.Query()
	encoding := strings.ToLower(q.Get("encoding"))
	container := strings.ToLower(q.Get("container"))
	sampleRate := atoi(q.Get("sample_rate"), 24000)

	if container == "wav" && (encoding == "linear16" || encoding == "pcm") {
		return audio.WAV(sampleRate), nil
	}

	switch encoding {
	case "":
		return audio.MP3(24000, 128), nil
	case "linear16", "pcm":
		return audio.PCM(sampleRate), nil
	case "mp3":
		return audio.MP3(sampleRate, 128), nil
	case "opus":
		return audio.Ogg(), nil
	case "aac":
		return audio.AAC(), nil
	case "mulaw", "alaw":
		return audio.ULaw(), nil
	default:
		return audio.Format{}, fmt.Errorf("unsupported encoding %q", encoding)
	}
}

func atoi(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return def
}

func errCode(status int) string {
	if status >= 400 && status < 500 {
		return "BAD_REQUEST"
	}
	return "INTERNAL"
}
