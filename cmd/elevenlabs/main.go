// Command elevenlabs runs the ElevenLabs-compatible TTS shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/elevenlabs"
)

func main() { shim.Run(elevenlabs.New()) }
