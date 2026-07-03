// Command lmnt runs the LMNT-compatible TTS shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/lmnt"
)

func main() {
	shim.Run(lmnt.New())
}
