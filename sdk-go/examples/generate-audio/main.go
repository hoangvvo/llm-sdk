package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
)

func main() {
	model := examples.GetModel("openai", "gpt-4o-audio-preview")

	response, err := model.Generate(context.Background(), &llmsdk.LanguageModelInput{
		Extra: map[string]any{
			"audio": map[string]any{
				"voice":  "alloy",
				"format": "mp3",
			},
		},
		Modalities: []llmsdk.Modality{llmsdk.ModalityText, llmsdk.ModalityAudio},
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Is a golden retriever a good family dog?"),
			),
		},
	})

	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	fmt.Println(examples.ToJSONString(response))

	// Find and play audio part
	for _, part := range response.Content {
		if part.Type() == llmsdk.PartTypeAudio && part.AudioPart != nil {
			audioData, err := base64.StdEncoding.DecodeString(part.AudioPart.AudioData)
			if err != nil {
				log.Printf("Failed to decode audio data: %v", err)
				continue
			}

			// Decode MP3 audio data
			decoder, err := mp3.NewDecoder(bytes.NewReader(audioData))
			if err != nil {
				log.Printf("Failed to create MP3 decoder: %v", err)
				continue
			}

			// Initialize audio context for MP3 playback
			op := &oto.NewContextOptions{
				SampleRate:   decoder.SampleRate(),
				ChannelCount: 2, // MP3 is typically stereo
				Format:       oto.FormatSignedInt16LE,
			}

			otoContext, ready, err := oto.NewContext(op)
			if err != nil {
				log.Printf("Failed to create audio context: %v", err)
				continue
			}
			<-ready

			audioPlayer := otoContext.NewPlayer(decoder)
			fmt.Printf("Playing MP3 audio (sample rate: %d, channels: %d)\n",
				decoder.SampleRate(), 2)

			audioPlayer.Play()

			// Wait for playback to complete
			for audioPlayer.IsPlaying() {
			}

			audioPlayer.Close()
			fmt.Println("Audio playback finished")
		}
	}
}
