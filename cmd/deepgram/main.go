// Command deepgram runs the Deepgram-compatible TTS shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/deepgram"
)

func main() {
	shim.Run(deepgram.New())
}
