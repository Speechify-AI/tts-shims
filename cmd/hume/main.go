// Command hume runs the Hume-compatible TTS shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/hume"
)

func main() {
	shim.Run(hume.New())
}
