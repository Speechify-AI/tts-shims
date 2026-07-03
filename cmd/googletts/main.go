// Command googletts runs the Google-Cloud-TTS-compatible shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/googletts"
)

func main() { shim.Run(googletts.New()) }
