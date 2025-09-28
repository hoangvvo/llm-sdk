package main

// Requires ffplay (https://ffmpeg.org/) on PATH.
import (
	"context"
	"encoding/base64"
	"log"
	"os/exec"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/sanity-io/litter"
)

func main() {
	model := examples.GetModel("openai-chat-completion", "gpt-4o-audio-preview")

	response, err := model.Generate(context.Background(), &llmsdk.LanguageModelInput{
		Modalities: []llmsdk.Modality{llmsdk.ModalityText, llmsdk.ModalityAudio},
		Audio: &llmsdk.AudioOptions{
			Format: ptr.To(llmsdk.AudioFormatMP3),
			Voice:  ptr.To("alloy"),
		},
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Is a golden retriever a good family dog?"),
			),
		},
	})
	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	litter.Dump(response)

	for _, part := range response.Content {
		if part.AudioPart == nil {
			continue
		}

		audioData, err := base64.StdEncoding.DecodeString(part.AudioPart.AudioData)
		if err != nil {
			log.Fatalf("Failed to decode audio data: %v", err)
		}

		if err := play(audioData); err != nil {
			log.Fatalf("ffplay failed: %v", err)
		}

		return
	}

	log.Fatal("Audio part not found in response")
}

func play(audio []byte) error {
	cmd := exec.Command("ffplay", "-autoexit", "-nodisp", "-loglevel", "error", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := stdin.Write(audio); err != nil {
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}

	return cmd.Wait()
}
