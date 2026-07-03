// Command resemble runs the Resemble-AI-compatible shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/resemble"
)

func main() { shim.Run(resemble.New()) }
