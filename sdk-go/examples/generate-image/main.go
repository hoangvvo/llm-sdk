package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"

	"github.com/qeesung/image2ascii/convert"
)

func main() {
	model := examples.GetModel("google", "gemini-2.0-flash-exp-image-generation")

	log.Println("Requesting image generation...")
	response, err := model.Generate(context.Background(), &llmsdk.LanguageModelInput{
		Modalities: []llmsdk.Modality{llmsdk.ModalityText, llmsdk.ModalityImage},
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart(
					"A bright, sunlit green hill with a single large, leafy tree, " +
						"fluffy clouds drifting across a deep blue sky, painted in the warm, " +
						"detailed, hand-painted style of a Studio Ghibli landscape, soft colors, " +
						"gentle light, and a sense of quiet wonder.",
				),
			),
		},
	})
	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	// Generation response is intentionally not printed to keep output concise

	for _, part := range response.Content {
		if part.ImagePart != nil {
			mime := part.ImagePart.MimeType
			ext := "png"
			if mime != "" {
				if sp := strings.SplitN(mime, "/", 2); len(sp) == 2 && sp[1] != "" {
					ext = sp[1]
				}
			}
			fileName := fmt.Sprintf("image.%s", ext)

			data, err := base64.StdEncoding.DecodeString(part.ImagePart.ImageData)
			if err != nil {
				log.Fatalf("Failed to decode image data: %v", err)
			}

			if err := os.WriteFile(fileName, data, 0o644); err != nil {
				log.Fatalf("Failed to write image file: %v", err)
			}
			log.Println("Saved image to", fileName)

			log.Println("Rendering image to terminal...")
			converter := convert.NewImageConverter()
			// Basic defaults; tune as needed
			opts := convert.DefaultOptions
			opts.FixedWidth = 80
			opts.Colored = true
			ascii := converter.ImageFile2ASCIIString(fileName, &opts)
			fmt.Println(ascii)

			fmt.Println("---")
			_ = openFile(fileName)

			time.Sleep(5 * time.Second)
			_ = os.Remove(fileName)
			log.Println("Done.")
			return
		}
	}

	log.Println("Image part not found in response")
}

func openFile(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Start()
	case "linux":
		return exec.Command("xdg-open", path).Start()
	case "windows":
		return exec.Command("cmd", "/C", "start", "", path).Start()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}
