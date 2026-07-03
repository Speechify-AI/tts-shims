// Package audio defines the shared vocabulary for describing a synthesized
// audio format: the Speechify output_format value to request and the MIME type
// to advertise back to the caller. Every provider maps its own format enum onto
// a Format so the backends and handler stay provider-agnostic.
package audio

import "fmt"

// Format pairs a Speechify stream output_format with the response Content-Type
// the shim advertises. When WrapWAV is true the upstream body is raw PCM and the
// handler must prepend a streaming WAV header (see internal/wav); Rate is that
// PCM sample rate so the header can be built to match.
type Format struct {
	OutputFormat string
	ContentType  string
	WrapWAV      bool
	Rate         int
}

// PCMRates is the set of sample rates Speechify's pcm_* output_format supports.
var PCMRates = []int{8000, 16000, 22050, 24000, 44100, 48000}

// MP3 returns an mp3 Format at the nearest supported Speechify rate/bitrate.
// Speechify offers mp3 at 22050 and 24000 Hz with bitrate tiers 32/64/96/128/192
// kbps; any other requested rate collapses onto 24000 Hz.
func MP3(sampleRate, bitrateKbps int) Format {
	rate := 24000
	if sampleRate == 22050 {
		rate = 22050
	}
	kbps := nearestBitrate(bitrateKbps)
	return Format{OutputFormat: fmt.Sprintf("mp3_%d_%d", rate, kbps), ContentType: "audio/mpeg"}
}

// PCM returns a raw-PCM Format snapped to the nearest supported Speechify rate.
func PCM(sampleRate int) Format {
	rate := NearestPCMRate(sampleRate, 24000)
	return Format{
		OutputFormat: fmt.Sprintf("pcm_%d", rate),
		ContentType:  fmt.Sprintf("audio/L16; rate=%d; channels=1", rate),
		Rate:         rate,
	}
}

// WAV returns a Format that requests raw PCM but is flagged for WAV wrapping, so
// the handler emits a real streaming .wav. Speechify's stream endpoint rejects
// wav_* (422), so this is the only way to serve WAV over the streaming path.
func WAV(sampleRate int) Format {
	rate := NearestPCMRate(sampleRate, 24000)
	return Format{
		OutputFormat: fmt.Sprintf("pcm_%d", rate),
		ContentType:  "audio/wav",
		WrapWAV:      true,
		Rate:         rate,
	}
}

// Ogg returns an Opus-in-ogg Format. Speechify's only ogg codec is Opus, so any
// ogg/opus/vorbis request maps here.
func Ogg() Format {
	return Format{OutputFormat: "ogg_24000", ContentType: "audio/ogg"}
}

// AAC returns an aac Format.
func AAC() Format {
	return Format{OutputFormat: "aac_24000", ContentType: "audio/aac"}
}

// ULaw returns an 8 kHz mu-law Format (telephony).
func ULaw() Format {
	return Format{OutputFormat: "ulaw_8000", ContentType: "audio/basic"}
}

// NearestPCMRate snaps want to the closest supported Speechify pcm rate,
// returning def when want is non-positive.
func NearestPCMRate(want, def int) int {
	if want <= 0 {
		return def
	}
	best := PCMRates[0]
	bestDiff := abs(want - best)
	for _, r := range PCMRates[1:] {
		if d := abs(want - r); d < bestDiff {
			best, bestDiff = r, d
		}
	}
	return best
}

func nearestBitrate(kbps int) int {
	switch {
	case kbps <= 0:
		return 128
	case kbps <= 32:
		return 32
	case kbps <= 64:
		return 64
	case kbps <= 96:
		return 96
	case kbps <= 128:
		return 128
	default:
		return 192
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
