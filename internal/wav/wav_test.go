package wav

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestHeader(t *testing.T) {
	h := Header(24000, 16, 1)

	if len(h) != HeaderSize {
		t.Fatalf("len = %d, want %d", len(h), HeaderSize)
	}
	if !bytes.Equal(h[0:4], []byte("RIFF")) {
		t.Errorf("bytes 0-4 = %q, want RIFF", h[0:4])
	}
	if !bytes.Equal(h[8:12], []byte("WAVE")) {
		t.Errorf("bytes 8-12 = %q, want WAVE", h[8:12])
	}
	if !bytes.Equal(h[12:16], []byte("fmt ")) {
		t.Errorf("bytes 12-16 = %q, want 'fmt '", h[12:16])
	}
	if !bytes.Equal(h[36:40], []byte("data")) {
		t.Errorf("bytes 36-40 = %q, want data", h[36:40])
	}

	if got := binary.LittleEndian.Uint32(h[4:8]); got != streamingUnknownSize {
		t.Errorf("RIFF size = %#x, want %#x", got, streamingUnknownSize)
	}
	if got := binary.LittleEndian.Uint32(h[40:44]); got != streamingUnknownSize {
		t.Errorf("data size = %#x, want %#x", got, streamingUnknownSize)
	}

	if got := binary.LittleEndian.Uint32(h[16:20]); got != 16 {
		t.Errorf("fmt chunk size = %d, want 16", got)
	}
	if got := binary.LittleEndian.Uint16(h[20:22]); got != 1 {
		t.Errorf("audio format = %d, want 1 (PCM)", got)
	}
	if got := binary.LittleEndian.Uint16(h[22:24]); got != 1 {
		t.Errorf("channels = %d, want 1", got)
	}
	if got := binary.LittleEndian.Uint32(h[24:28]); got != 24000 {
		t.Errorf("sample rate = %d, want 24000", got)
	}
	if got := binary.LittleEndian.Uint32(h[28:32]); got != 48000 {
		t.Errorf("byte rate = %d, want 48000", got)
	}
	if got := binary.LittleEndian.Uint16(h[32:34]); got != 2 {
		t.Errorf("block align = %d, want 2", got)
	}
	if got := binary.LittleEndian.Uint16(h[34:36]); got != 16 {
		t.Errorf("bits per sample = %d, want 16", got)
	}
}

func TestHeaderStereo48k(t *testing.T) {
	h := Header(48000, 16, 2)
	if got := binary.LittleEndian.Uint32(h[28:32]); got != 192000 {
		t.Errorf("byte rate = %d, want 192000 (48000*2*2)", got)
	}
	if got := binary.LittleEndian.Uint16(h[32:34]); got != 4 {
		t.Errorf("block align = %d, want 4 (2ch*16bit/8)", got)
	}
}
