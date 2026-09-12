package partutil

import (
	"fmt"
	"strings"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

var audioFormatToMimeTypeMap = map[llmsdk.AudioFormat]string{
	llmsdk.AudioFormatWav:      "audio/wav",
	llmsdk.AudioFormatLinear16: "audio/L16",
	llmsdk.AudioFormatFLAC:     "audio/flac",
	llmsdk.AudioFormatMulaw:    "audio/basic",
	llmsdk.AudioFormatAlaw:     "audio/basic",
	llmsdk.AudioFormatMP3:      "audio/mpeg",
	llmsdk.AudioFormatOpus:     `audio/ogg; codecs="opus"`,
	llmsdk.AudioFormatAAC:      "audio/aac",
}

func MapAudioFormatToMimeType(format llmsdk.AudioFormat) string {
	if mimeType, ok := audioFormatToMimeTypeMap[format]; ok {
		return mimeType
	}
	return "application/octet-stream"
}

// mimeTypeToAudioFormatMap accepts the MIME types produced by MapAudioFormatToMimeType
// as well as common aliases returned by providers. Parameters (e.g. codecs) are ignored.
var mimeTypeToAudioFormatMap = map[string]llmsdk.AudioFormat{
	"audio/wav":   llmsdk.AudioFormatWav,
	"audio/x-wav": llmsdk.AudioFormatWav,
	"audio/wave":  llmsdk.AudioFormatWav,
	"audio/l16":   llmsdk.AudioFormatLinear16,
	"audio/pcm":   llmsdk.AudioFormatLinear16,
	"audio/flac":  llmsdk.AudioFormatFLAC,
	"audio/basic": llmsdk.AudioFormatMulaw,
	"audio/mpeg":  llmsdk.AudioFormatMP3,
	"audio/mp3":   llmsdk.AudioFormatMP3,
	"audio/ogg":   llmsdk.AudioFormatOpus,
	"audio/opus":  llmsdk.AudioFormatOpus,
	"audio/aac":   llmsdk.AudioFormatAAC,
}

func MapMimeTypeToAudioFormat(mimeType string) (llmsdk.AudioFormat, error) {
	normalized := mimeType
	// strip out the parts after ;
	if idx := strings.Index(normalized, ";"); idx != -1 {
		normalized = normalized[:idx]
	}
	normalized = strings.ToLower(strings.TrimSpace(normalized))
	if format, ok := mimeTypeToAudioFormatMap[normalized]; ok {
		return format, nil
	}
	return "", fmt.Errorf("unsupported audio format for mime type: %s", mimeType)
}
