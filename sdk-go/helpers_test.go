package llmsdk_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

func TestMediaPartConstructors(t *testing.T) {
	imageOptions := []llmsdk.ImagePartOption{
		llmsdk.WithImageWidth(640), llmsdk.WithImageHeight(480), llmsdk.WithImageID("image_1"),
	}
	audioOptions := []llmsdk.AudioPartOption{
		llmsdk.WithAudioSampleRate(24000), llmsdk.WithAudioChannels(1),
		llmsdk.WithAudioTranscript("Hello"), llmsdk.WithAudioID("audio_1"),
	}
	fileOptions := []llmsdk.FilePartOption{llmsdk.WithFileFilename("document.pdf")}

	tests := []struct {
		name string
		part llmsdk.Part
		want string
	}{
		{
			name: "image data",
			part: llmsdk.NewImagePart("AAEC", "image/png", imageOptions...),
			want: `{"type":"image","data":"AAEC","mime_type":"image/png","width":640,"height":480,"id":"image_1"}`,
		},
		{
			name: "image URL",
			part: llmsdk.NewImagePartFromURL("https://example.com/image.png", "image/png", imageOptions...),
			want: `{"type":"image","url":"https://example.com/image.png","mime_type":"image/png","width":640,"height":480,"id":"image_1"}`,
		},
		{
			name: "audio data",
			part: llmsdk.NewAudioPart("AAEC", llmsdk.AudioFormatMP3, audioOptions...),
			want: `{"type":"audio","data":"AAEC","format":"mp3","sample_rate":24000,"channels":1,"transcript":"Hello","id":"audio_1"}`,
		},
		{
			name: "audio URL",
			part: llmsdk.NewAudioPartFromURL("https://example.com/audio.mp3", llmsdk.AudioFormatMP3, audioOptions...),
			want: `{"type":"audio","url":"https://example.com/audio.mp3","format":"mp3","sample_rate":24000,"channels":1,"transcript":"Hello","id":"audio_1"}`,
		},
		{
			name: "file data",
			part: llmsdk.NewFilePart("AAEC", "application/pdf", fileOptions...),
			want: `{"type":"file","data":"AAEC","mime_type":"application/pdf","filename":"document.pdf"}`,
		},
		{
			name: "file URL",
			part: llmsdk.NewFilePartFromURL("https://example.com/document.pdf", "application/pdf", fileOptions...),
			want: `{"type":"file","url":"https://example.com/document.pdf","mime_type":"application/pdf","filename":"document.pdf"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.part)
			if err != nil {
				t.Fatal(err)
			}
			var got, want map[string]any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tt.want), &want); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("JSON mismatch (-want +got):\n%s", diff)
			}
			var decoded llmsdk.Part
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.part, decoded); diff != "" {
				t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
