// Package shim contains the provider-agnostic HTTP engine. Each supported TTS
// provider implements Provider to describe its routes, how it authenticates,
// how it translates an incoming request into a Speechify request, and how it
// renders the response. The Handler wires a Provider to the two Speechify
// backends (raw stream vs base64 speech) so all providers share one code path.
package shim

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/Speechify-AI/tts-shims/internal/audio"
	"github.com/Speechify-AI/tts-shims/internal/config"
	"github.com/Speechify-AI/tts-shims/internal/speechify"
	"github.com/Speechify-AI/tts-shims/internal/wav"
)

// Backend selects which Speechify surface a translated request targets.
type Backend int

const (
	// BackendStream targets POST /v1/audio/stream and relays raw audio bytes.
	BackendStream Backend = iota
	// BackendSpeech targets POST /v1/audio/speech and yields base64 audio that
	// the provider re-wraps into its own JSON response envelope.
	BackendSpeech
)

// Translated is the result of a provider mapping an incoming request onto a
// Speechify request plus the response shape the caller expects.
type Translated struct {
	Request speechify.Request
	Format  audio.Format
	Backend Backend
}

// Provider describes one TTS dialect the shim can speak.
type Provider interface {
	// Name is the provider's stable identifier (e.g. "openai").
	Name() string
	// Route is the net/http ServeMux pattern the provider listens on,
	// e.g. "POST /v1/audio/speech" or "POST /v1/text-to-speech/{voice_id}".
	Route() string
	// UpstreamAuth resolves the Speechify credential for a request. Returning an
	// empty Bearer means unauthenticated (Speechify will 401). Providers use this
	// to implement server-key-wins or pass-through policies.
	UpstreamAuth(r *http.Request, serverKey string) speechify.Auth
	// Translate parses the incoming request and produces the Speechify request
	// plus target backend/format. A returned error is rendered via WriteError.
	Translate(r *http.Request, def Defaults) (Translated, error)
	// RenderSpeech re-wraps base64 audio from the Speechify Speech backend into
	// the provider's JSON response envelope. Only called for BackendSpeech.
	RenderSpeech(audioBase64 string, tr Translated) (contentType string, body []byte, err error)
	// WriteError renders an error in the provider's native error shape.
	WriteError(w http.ResponseWriter, status int, message string)
}

// Defaults are provider-independent fallbacks resolved from config.
type Defaults struct {
	Model string
}

// Handler serves a single Provider against the Speechify backends.
type Handler struct {
	provider Provider
	client   *speechify.Client
	cfg      config.Config
}

// NewHandler builds an http.Handler for the given provider.
func NewHandler(p Provider, cfg config.Config) *Handler {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &Handler{
		provider: p,
		client:   speechify.New(cfg.UpstreamBaseURL, &http.Client{Transport: transport}),
		cfg:      cfg,
	}
}

// ServeHTTP translates the request, calls the selected Speechify backend, and
// renders the response in the provider's shape.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tr, err := h.provider.Translate(r, Defaults{Model: h.cfg.DefaultModel})
	if err != nil {
		h.provider.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	auth := h.provider.UpstreamAuth(r, h.cfg.APIKey)

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.RequestTimeout)
	defer cancel()

	switch tr.Backend {
	case BackendSpeech:
		h.serveSpeech(ctx, w, tr, auth)
	default:
		h.serveStream(ctx, w, tr, auth)
	}
}

func (h *Handler) serveStream(ctx context.Context, w http.ResponseWriter, tr Translated, auth speechify.Auth) {
	resp, err := h.client.Stream(ctx, tr.Request, tr.Format.ContentType, auth, h.cfg.SpeechifyVersion)
	if err != nil {
		h.provider.WriteError(w, http.StatusBadGateway, "failed to reach upstream")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.provider.WriteError(w, resp.StatusCode, upstreamMessage(resp.Body, resp.StatusCode))
		return
	}

	w.Header().Set("Content-Type", tr.Format.ContentType)
	if reqID := resp.Header.Get("X-Request-ID"); reqID != "" {
		w.Header().Set("X-Request-ID", reqID)
	}
	w.WriteHeader(http.StatusOK)

	if tr.Format.WrapWAV {
		if _, err := w.Write(wav.Header(tr.Format.Rate, 16, 1)); err != nil {
			return
		}
		flush(w)
	}
	streamCopy(w, resp.Body)
}

func (h *Handler) serveSpeech(ctx context.Context, w http.ResponseWriter, tr Translated, auth speechify.Auth) {
	sr, err := h.client.Speech(ctx, tr.Request, auth, h.cfg.SpeechifyVersion)
	if err != nil {
		if ue, ok := err.(*speechify.UpstreamError); ok {
			h.provider.WriteError(w, ue.Status, ue.Error())
			return
		}
		h.provider.WriteError(w, http.StatusBadGateway, "failed to reach upstream")
		return
	}

	// Speechify returns standard base64; providers re-encode as needed.
	if _, err := base64.StdEncoding.DecodeString(sr.AudioData); err != nil {
		h.provider.WriteError(w, http.StatusBadGateway, "upstream returned invalid audio data")
		return
	}

	contentType, body, err := h.provider.RenderSpeech(sr.AudioData, tr)
	if err != nil {
		h.provider.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func upstreamMessage(r io.Reader, status int) string {
	var e speechify.ErrorEnvelope
	data, _ := io.ReadAll(io.LimitReader(r, 64*1024))
	_ = json.Unmarshal(data, &e)
	if e.Error.Message != "" {
		return e.Error.Message
	}
	return http.StatusText(status)
}

func flush(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// streamCopy pipes the upstream body to the client, flushing after each chunk so
// audio reaches the caller with minimal added latency.
func streamCopy(w http.ResponseWriter, src io.Reader) {
	flusher, canFlush := w.(http.Flusher)
	if !canFlush {
		_, _ = io.Copy(w, src)
		return
	}
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			flusher.Flush()
		}
		if err != nil {
			return
		}
	}
}
