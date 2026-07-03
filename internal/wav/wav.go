// Package wav builds a canonical PCM WAV (RIFF/WAVE) header suitable for
// streaming, where the total byte count is not known when the header is written.
package wav

import "encoding/binary"

// streamingUnknownSize is written into the RIFF and data chunk size fields when
// the total length is not yet known. 0xFFFFFFFF is the conventional
// "size unknown / stream until EOF" sentinel understood by ffmpeg, browsers and
// most media players, which decode using the format fields and read until the
// connection closes.
const streamingUnknownSize = 0xFFFFFFFF

// HeaderSize is the byte length of a canonical PCM WAV header.
const HeaderSize = 44

// Header returns a 44-byte little-endian PCM WAV header for the given format.
// sampleRate is in Hz, bitsPerSample is typically 16, and channels is 1 for
// mono. The chunk size fields are set to the streaming-unknown sentinel so the
// header can be emitted before the audio body length is known.
func Header(sampleRate, bitsPerSample, channels int) []byte {
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8

	h := make([]byte, HeaderSize)

	copy(h[0:4], "RIFF")
	binary.LittleEndian.PutUint32(h[4:8], streamingUnknownSize)
	copy(h[8:12], "WAVE")

	copy(h[12:16], "fmt ")
	binary.LittleEndian.PutUint32(h[16:20], 16)
	binary.LittleEndian.PutUint16(h[20:22], 1)
	binary.LittleEndian.PutUint16(h[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(h[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(h[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(h[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(h[34:36], uint16(bitsPerSample))

	copy(h[36:40], "data")
	binary.LittleEndian.PutUint32(h[40:44], streamingUnknownSize)

	return h
}
