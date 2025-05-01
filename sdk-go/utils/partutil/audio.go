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

func MapMimeTypeToAudioFormat(mimeType string) (llmsdk.AudioFormat, error) {
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
