// Command awspolly runs the AWS-Polly-compatible TTS shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/awspolly"
)

func main() { shim.Run(awspolly.New()) }
