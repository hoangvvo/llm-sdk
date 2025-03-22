package llmsdk

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
)

// base64ToInt16Samples converts base64 string to int16 samples
func base64ToInt16Samples(b64 string) ([]int16, error) {
	bytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(bytes)%2 != 0 {
		return nil, fmt.Errorf("base64 data length is not a multiple of 2")
	}

	samples := make([]int16, len(bytes)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(bytes[i*2:]))
	}

	return samples, nil
}

// int16SamplesToBase64 converts int16 samples to base64 string
func int16SamplesToBase64(samples []int16) string {
	bytes := make([]byte, len(samples)*2)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(bytes[i*2:], uint16(sample))
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

// concatenateB64AudioChunks concatenates audio data chunks into a single base64 string
func concatenateB64AudioChunks(chunks []string) (string, error) {
	if len(chunks) == 0 {
		return "", nil
	}

	var allSamples []int16
	for _, chunk := range chunks {
		samples, err := base64ToInt16Samples(chunk)
		if err != nil {
			return "", NewInvariantError("", fmt.Sprintf("Failed to decode audio chunk: %s", err.Error()))
		}
		allSamples = append(allSamples, samples...)
	}

	return int16SamplesToBase64(allSamples), nil
}

var audioFormatToMimeTypeMap = map[AudioFormat]string{
	AudioFormatWav:      "audio/wav",
	AudioFormatLinear16: "audio/L16",
	AudioFormatFLAC:     "audio/flac",
	AudioFormatMulaw:    "audio/basic",
	AudioFormatAlaw:     "audio/basic",
	AudioFormatMP3:      "audio/mpeg",
	AudioFormatOpus:     `audio/ogg; codecs="opus"`,
	AudioFormatAAC:      "audio/aac",
}

func MapAudioFormatToMimeType(format AudioFormat) string {
	if mimeType, ok := audioFormatToMimeTypeMap[format]; ok {
		return mimeType
	}
	return "application/octet-stream"
}

func MapMimeTypeToAudioFormat(mimeType string) (AudioFormat, error) {
	// strip out the parts after ;
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	for format, mt := range audioFormatToMimeTypeMap {
		if mimeType == mt {
			return format, nil
		}
	}
	return "", fmt.Errorf("unsupported audio format for mime type: %s", mimeType)
}
