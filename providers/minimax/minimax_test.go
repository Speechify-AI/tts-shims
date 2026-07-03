package minimax

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/Speechify-AI/tts-shims/internal/shim"
)

func TestRenderSpeechTranscodesBase64ToHex(t *testing.T) {
	raw := []byte{0xde, 0xad, 0xbe, 0xef}
	b64 := base64.StdEncoding.EncodeToString(raw)

	ct, body, err := (&Provider{}).RenderSpeech(b64, shim.Translated{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	var out struct {
		Data struct {
			Audio string `json:"audio"`
		} `json:"data"`
		BaseResp struct {
			StatusCode int `json:"status_code"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if out.Data.Audio != hex.EncodeToString(raw) {
		t.Errorf("data.audio = %q, want hex %q", out.Data.Audio, hex.EncodeToString(raw))
	}
	if out.BaseResp.StatusCode != 0 {
		t.Errorf("base_resp.status_code = %d, want 0", out.BaseResp.StatusCode)
	}
}
