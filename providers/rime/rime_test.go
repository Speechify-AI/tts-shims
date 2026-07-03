package rime

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Speechify-AI/tts-shims/internal/shim"
)

func TestProviderModelIDDoesNotLeakToSpeechify(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/rime-tts",
		strings.NewReader(`{"speaker":"george","text":"hi","modelId":"mistv2"}`))
	tr, err := (&Provider{}).Translate(req, shim.Defaults{Model: "simba-english"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.Request.Model != "simba-english" {
		t.Errorf("model = %q, want simba-english (mistv2 must not leak)", tr.Request.Model)
	}
	if tr.Request.VoiceID != "george" {
		t.Errorf("voice_id = %q, want george", tr.Request.VoiceID)
	}
}

func TestNativeSpeechifyModelPassesThrough(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/rime-tts",
		strings.NewReader(`{"speaker":"george","text":"hi","modelId":"simba-3.0"}`))
	tr, _ := (&Provider{}).Translate(req, shim.Defaults{Model: "simba-english"})
	if tr.Request.Model != "simba-3.0" {
		t.Errorf("model = %q, want simba-3.0", tr.Request.Model)
	}
}

func TestAcceptHeaderSelectsFormat(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/rime-tts",
		strings.NewReader(`{"speaker":"george","text":"hi","samplingRate":16000}`))
	req.Header.Set("Accept", "audio/wav")
	tr, _ := (&Provider{}).Translate(req, shim.Defaults{Model: "simba-english"})
	if tr.Format.ContentType != "audio/wav" {
		t.Errorf("content-type = %q, want audio/wav", tr.Format.ContentType)
	}
	if !tr.Format.WrapWAV {
		t.Errorf("WrapWAV should be set for audio/wav")
	}
}
