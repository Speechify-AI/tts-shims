// Command cartesia runs the Cartesia-compatible TTS shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/cartesia"
)

func main() { shim.Run(cartesia.New()) }
