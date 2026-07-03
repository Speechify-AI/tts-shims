// Command fish runs the Fish-compatible TTS shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/fish"
)

func main() {
	shim.Run(fish.New())
}
