package partutil

import (
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

func TestMapMimeTypeToAudioFormatNormalizesParametersAndCase(t *testing.T) {
	tests := map[string]llmsdk.AudioFormat{
		"audio/l16; rate=24000; channels=1": llmsdk.AudioFormatLinear16,
	}

	for mimeType, expected := range tests {
		t.Run(mimeType, func(t *testing.T) {
			actual, err := MapMimeTypeToAudioFormat(mimeType)
			if err != nil {
				t.Fatalf("MapMimeTypeToAudioFormat returned an error: %v", err)
			}
			if actual != expected {
				t.Fatalf("MapMimeTypeToAudioFormat() = %q, want %q", actual, expected)
			}
		})
	}
}
