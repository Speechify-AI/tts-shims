package shim_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Speechify-AI/tts-shims/internal/audio"
	"github.com/Speechify-AI/tts-shims/internal/config"
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/internal/speechify"
)

// streamProvider is a minimal Provider that targets the stream backend.
type streamProvider struct{ format audio.Format }

func (streamProvider) Name() string  { return "test-stream" }
func (streamProvider) Route() string { return "POST /x" }
func (streamProvider) UpstreamAuth(_ *http.Request, k string) speechify.Auth {
	return speechify.Auth{Bearer: k}
}
func (p streamProvider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	var body struct {
		Text string `json:"text"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Text == "" {
		return shim.Translated{}, errBad("text required")
	}
	return shim.Translated{
		Request: speechify.Request{Input: body.Text, VoiceID: "george", Model: def.Model, OutputFormat: p.format.OutputFormat},
		Format:  p.format,
		Backend: shim.BackendStream,
	}, nil
}
func (streamProvider) RenderSpeech(string, shim.Translated) (string, []byte, error) {
	return "", nil, errBad("unused")
}
func (streamProvider) WriteError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// speechProvider targets the base64 speech backend and upper-cases the base64
// so the test can prove RenderSpeech output is what reaches the client.
type speechProvider struct{}

func (speechProvider) Name() string  { return "test-speech" }
func (speechProvider) Route() string { return "POST /y" }
func (speechProvider) UpstreamAuth(_ *http.Request, k string) speechify.Auth {
	return speechify.Auth{Bearer: k}
}
func (speechProvider) Translate(r *http.Request, def shim.Defaults) (shim.Translated, error) {
	return shim.Translated{
		Request: speechify.Request{Input: "hi", VoiceID: "george", AudioFormat: "mp3"},
		Format:  audio.MP3(24000, 128),
		Backend: shim.BackendSpeech,
	}, nil
}
func (speechProvider) RenderSpeech(b64 string, _ shim.Translated) (string, []byte, error) {
	return "application/json", []byte(`{"wrapped":"` + b64 + `"}`), nil
}
func (speechProvider) WriteError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

type badErr string

func (b badErr) Error() string { return string(b) }
func errBad(s string) error    { return badErr(s) }

func cfg(url string) config.Config {
	return config.Config{UpstreamBaseURL: url, APIKey: "sk_server", DefaultModel: "simba-english", RequestTimeout: 5_000_000_000}
}

func TestStreamBackendPassesThroughAndWrapsWAV(t *testing.T) {
	pcm := bytes.Repeat([]byte{0x01, 0x02}, 50)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/stream" {
			t.Errorf("path=%q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk_server" {
			t.Errorf("auth=%q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(200)
		_, _ = w.Write(pcm)
	}))
	defer up.Close()

	h := shim.NewHandler(streamProvider{format: audio.WAV(24000)}, cfg(up.URL))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/x", strings.NewReader(`{"text":"hi"}`)))

	res := rec.Result()
	if res.StatusCode != 200 {
		t.Fatalf("status=%d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "audio/wav" {
		t.Errorf("content-type=%q", ct)
	}
	body, _ := io.ReadAll(res.Body)
	if !bytes.Equal(body[0:4], []byte("RIFF")) {
		t.Errorf("missing WAV header: %x", body[0:4])
	}
	if !bytes.Equal(body[44:], pcm) {
		t.Errorf("pcm payload mismatch")
	}
}

func TestSpeechBackendReWrapsBase64(t *testing.T) {
	audioBytes := []byte("hello-audio")
	b64 := base64.StdEncoding.EncodeToString(audioBytes)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" {
			t.Errorf("path=%q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"audio_data": b64, "audio_format": "mp3", "billable_characters_count": 2,
			"speech_marks": map[string]any{"chunks": []any{}, "end": 0, "end_time": 0, "start": 0, "start_time": 0, "type": "word", "value": ""},
		})
	}))
	defer up.Close()

	h := shim.NewHandler(speechProvider{}, cfg(up.URL))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/y", strings.NewReader(`{}`)))

	res := rec.Result()
	if res.StatusCode != 200 {
		t.Fatalf("status=%d", res.StatusCode)
	}
	var out struct {
		Wrapped string `json:"wrapped"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	if out.Wrapped != b64 {
		t.Errorf("wrapped base64 = %q, want %q", out.Wrapped, b64)
	}
}

func TestTranslateErrorRendered(t *testing.T) {
	h := shim.NewHandler(streamProvider{format: audio.MP3(24000, 128)}, cfg("http://127.0.0.1:1"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/x", strings.NewReader(`{}`)))
	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Result().StatusCode)
	}
}
