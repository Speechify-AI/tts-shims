// Package awspolly implements the AWS Polly SynthesizeSpeech (POST /v1/speech)
// dialect. Polly clients send SigV4-signed requests; the shim does not verify
// the signature (it holds the Speechify key server-side and treats caller AWS
// credentials as opaque), so any AWS SDK, CLI, or BYOC client works unchanged.
package awspolly

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
)

// Provider speaks the AWS Polly SynthesizeSpeech dialect.
type Provider struct{}

// New returns the AWS Polly provider.
func New() *Provider { return &Provider{} }

// Name identifies the provider.
func (*Provider) Name() string { return "aws-polly" }

// Route is the Polly SynthesizeSpeech endpoint.
func (*Provider) Route() string { return "POST /v1/speech" }

type request struct {
	Text         string `json:"Text"`
	TextType     string `json:"TextType,omitempty"`
	OutputFormat string `json:"OutputFormat,omitempty"`
	VoiceID      string `json:"VoiceId"`
	Engine       string `json:"Engine,omitempty"`
	SampleRate   string `json:"SampleRate,omitempty"`
	LanguageCode string `json:"LanguageCode,omitempty"`
}

// UpstreamAuth always injects the configured Speechify key; the caller's AWS
// SigV4 credentials authenticate to AWS, not Speechify, and are never reused.
func (*Provider) UpstreamAuth(_ *http.Request, serverKey string) speechify.Auth {
	return speechify.Auth{Bearer: serverKey}
}

// Translate maps the Polly request onto Speechify's stream backend.
func (*Provider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var in request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		return shim.Translated{}, fmt.Errorf("request body is not valid JSON")
	}
	if strings.TrimSpace(in.Text) == "" {
		return shim.Translated{}, fmt.Errorf("Text is required")
	}
	if strings.TrimSpace(in.VoiceID) == "" {
		return shim.Translated{}, fmt.Errorf("VoiceId is required")
	}

	format, err := resolveFormat(in.OutputFormat, in.SampleRate)
	if err != nil {
		return shim.Translated{}, err
	}

	return shim.Translated{
		Request: speechify.Request{
			Input:        in.Text,
			VoiceID:      in.VoiceID,
			Model:        def.Model,
			OutputFormat: format.OutputFormat,
			Language:     in.LanguageCode,
		},
		Format:  format,
		Backend: shim.BackendStream,
	}, nil
}

// RenderSpeech is unused: AWS Polly uses the stream backend.
func (*Provider) RenderSpeech(string, shim.Translated) (string, []byte, error) {
	return "", nil, fmt.Errorf("aws-polly does not use the speech backend")
}

// WriteError renders an AWS-shaped error with an x-amzn-ErrorType header.
func (*Provider) WriteError(w http.ResponseWriter, status int, message string) {
	errType := "ServiceFailureException"
	if status >= 400 && status < 500 {
		errType = "InvalidParameterValue"
	}
	w.Header().Set("Content-Type", "application/x-amz-json-1.0")
	w.Header().Set("x-amzn-ErrorType", errType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"message": message})
}

// resolveFormat combines Polly's OutputFormat and SampleRate. Polly's pcm is
// 16-bit signed mono little-endian and defaults to 16 kHz (not Speechify's
// 24 kHz), so the default is set explicitly; ogg_vorbis maps to Speechify's
// Opus-in-ogg; json/ogg_opus/mulaw/alaw are not served.
func resolveFormat(outputFormat, sampleRate string) (audio.Format, error) {
	switch strings.ToLower(outputFormat) {
	case "", "mp3":
		return audio.MP3(atoiRate(sampleRate, 24000), 128), nil
	case "ogg_vorbis":
		return audio.Ogg(), nil
	case "pcm":
		return audio.PCM(atoiRate(sampleRate, 16000)), nil
	default:
		return audio.Format{}, fmt.Errorf("unsupported OutputFormat %q", outputFormat)
	}
}

func atoiRate(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n
	}
	return def
}
