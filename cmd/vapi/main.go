// Command vapi runs the Vapi custom-voice TTS shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/vapi"
)

func main() {
	shim.Run(vapi.New())
}
