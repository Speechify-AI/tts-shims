// Command openai runs the OpenAI-compatible TTS shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/openai"
)

func main() {
	shim.Run(openai.New())
}
