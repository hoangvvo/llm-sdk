package main

import (
    "context"
    "encoding/base64"
    "fmt"
    "log"
    "os"
    "strings"
    "time"
    "runtime"
    "os/exec"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/sanity-io/litter"
)

func main() {
	model := examples.GetModel("google", "gemini-2.0-flash-exp-image-generation")

	response, err := model.Generate(context.Background(), &llmsdk.LanguageModelInput{
		Modalities: []llmsdk.Modality{llmsdk.ModalityText, llmsdk.ModalityImage},
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Generate an image of a sunset over the ocean"),
			),
		},
	})
	if err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	litter.Dump(response)

	for _, part := range response.Content {
		if part.ImagePart != nil {
			mime := part.ImagePart.MimeType
			ext := "png"
			if mime != "" {
				if sp := strings.SplitN(mime, "/", 2); len(sp) == 2 && sp[1] != "" {
					ext = sp[1]
				}
			}
			fileName := fmt.Sprintf("sunset.%s", ext)

			data, err := base64.StdEncoding.DecodeString(part.ImagePart.ImageData)
			if err != nil {
				log.Fatalf("Failed to decode image data: %v", err)
			}

            if err := os.WriteFile(fileName, data, 0o644); err != nil {
                log.Fatalf("Failed to write image file: %v", err)
            }
            fmt.Println("Saved image to", fileName)
            // Try to open with default image viewer
            if err := openFile(fileName); err != nil {
                log.Printf("Failed to open image: %v", err)
            }
            // Cleanup after a short delay similar to JS example
            time.Sleep(5 * time.Second)
            _ = os.Remove(fileName)
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
