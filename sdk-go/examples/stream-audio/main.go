package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/ebitengine/oto/v3"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
)

func main() {
	model := examples.GetModel("openai", "gpt-4o-audio-preview")

	response, err := model.Stream(context.Background(), &llmsdk.LanguageModelInput{
		Extra: map[string]any{
			"audio": map[string]any{
				"voice":  "alloy",
				"format": "pcm16",
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
		log.Fatalf("Stream failed: %v", err)
	}

	// Create a pipe for streaming audio
	audioReader, audioWriter := io.Pipe()
	var otoContext *oto.Context
	var audioPlayer *oto.Player
	var wg sync.WaitGroup
	var audioInitialized bool

	accumulator := llmsdk.NewStreamAccumulator()

	for response.Next() {
		current := response.Current()
		fmt.Println(examples.ToJSONString(current))

		if current.Delta != nil && current.Delta.Part.AudioPartDelta != nil {
			audioDelta := current.Delta.Part.AudioPartDelta
			if audioDelta.AudioData != nil {
				audioData, err := base64.StdEncoding.DecodeString(*audioDelta.AudioData)
				if err != nil {
					log.Printf("Failed to decode audio chunk: %v", err)
					continue
				}

				// Initialize audio context and player on first chunk
				if !audioInitialized {
					sampleRate := 24000
					if audioDelta.SampleRate != nil {
						sampleRate = int(*audioDelta.SampleRate)
					}

					channels := 1
					if audioDelta.Channels != nil {
						channels = int(*audioDelta.Channels)
					}

					op := &oto.NewContextOptions{
						SampleRate:   sampleRate,
						ChannelCount: channels,
						Format:       oto.FormatSignedInt16LE,
					}

					var ready chan struct{}
					otoContext, ready, err = oto.NewContext(op)
					if err != nil {
						log.Printf("Failed to create audio context: %v", err)
						continue
					}
					<-ready

					audioPlayer = otoContext.NewPlayer(audioReader)
					fmt.Printf("Initialized audio playback (sample rate: %d, channels: %d)\n", sampleRate, channels)

					// Start playback in a goroutine
					wg.Add(1)
					go func() {
						defer wg.Done()
						audioPlayer.Play()
						fmt.Println("Audio playback started")
					}()

					audioInitialized = true
				}

				// Stream audio chunk immediately
				_, err = audioWriter.Write(audioData)
				if err != nil {
					log.Printf("Failed to write audio chunk: %v", err)
				}
			}
		}

		if err := accumulator.AddPartial(*current); err != nil {
			log.Printf("Failed to add partial: %v", err)
		}
	}

	if err := response.Err(); err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	// Close the writer to signal end of audio stream
	if audioInitialized {
		audioWriter.Close()
		fmt.Println("Waiting for audio playback to finish...")
		wg.Wait()
		audioPlayer.Close()
		fmt.Println("Audio playback finished")
	}

	finalResponse, err := accumulator.ComputeResponse()
	if err != nil {
		log.Fatalf("Failed to compute response: %v", err)
	}

	fmt.Printf("Final response: %s\n", examples.ToJSONString(finalResponse))
}
