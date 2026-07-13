package vapi

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Speechify-AI/tts-shims/internal/shim"
)

func TestTranslateVoiceRequest(t *testing.T) {
	body := `{"message":{"type":"voice-request","text":"Hello from Vapi","sampleRate":24000,"timestamp":1720000000000,"call":{},"assistant":{}}}`
	req := httptest.NewRequest("POST", "/synthesize", strings.NewReader(body))

	tr, err := (&Provider{}).Translate(req, shim.Defaults{Model: "simba-english"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.Backend != shim.BackendStream {
		t.Fatalf("backend = %v, want stream", tr.Backend)
	}
	if tr.Request.Input != "Hello from Vapi" {
		t.Errorf("input = %q", tr.Request.Input)
	}
	if tr.Request.VoiceID != "geffen_32" {
		t.Errorf("voice_id = %q, want geffen_32", tr.Request.VoiceID)
	}
	if tr.Request.Model != "simba-3.2" {
		t.Errorf("model = %q, want simba-3.2", tr.Request.Model)
	}
	if tr.Request.OutputFormat != "pcm_24000" {
		t.Errorf("output_format = %q, want pcm_24000", tr.Request.OutputFormat)
	}
	if tr.Format.ContentType != "application/octet-stream" {
		t.Errorf("content-type = %q, want application/octet-stream", tr.Format.ContentType)
	}
}

func TestTranslateHonorsConfiguredModel(t *testing.T) {
	req := httptest.NewRequest("POST", "/synthesize", strings.NewReader(`{"message":{"type":"voice-request","text":"hi","sampleRate":16000}}`))

	tr, err := (&Provider{}).Translate(req, shim.Defaults{Model: "custom-model"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.Request.Model != "custom-model" {
		t.Errorf("model = %q, want custom-model", tr.Request.Model)
	}
	if tr.Request.OutputFormat != "pcm_16000" {
		t.Errorf("output_format = %q, want pcm_16000", tr.Request.OutputFormat)
	}
}

func TestTranslateRejectsWrongMessageType(t *testing.T) {
	req := httptest.NewRequest("POST", "/synthesize", strings.NewReader(`{"message":{"type":"status-update","text":"hi","sampleRate":24000}}`))

	_, err := (&Provider{}).Translate(req, shim.Defaults{Model: "simba-english"})
	if err == nil || !strings.Contains(err.Error(), "message.type") {
		t.Fatalf("err = %v, want message.type error", err)
	}
}

func TestTranslateRejectsUnsupportedSampleRate(t *testing.T) {
	req := httptest.NewRequest("POST", "/synthesize", strings.NewReader(`{"message":{"type":"voice-request","text":"hi","sampleRate":44100}}`))

	_, err := (&Provider{}).Translate(req, shim.Defaults{Model: "simba-english"})
	if err == nil || !strings.Contains(err.Error(), "sampleRate") {
		t.Fatalf("err = %v, want sampleRate error", err)
	}
}
