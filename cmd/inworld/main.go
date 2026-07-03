// Command inworld runs the Inworld-TTS-compatible shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/inworld"
)

func main() { shim.Run(inworld.New()) }
