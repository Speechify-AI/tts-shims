// Command rime runs the Rime-compatible TTS shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/rime"
)

func main() {
	shim.Run(rime.New())
}
