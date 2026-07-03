// Command minimax runs the MiniMax-T2A-compatible shim.
package main

import (
	"github.com/Speechify-AI/tts-shims/internal/shim"
	"github.com/Speechify-AI/tts-shims/providers/minimax"
)

func main() { shim.Run(minimax.New()) }
