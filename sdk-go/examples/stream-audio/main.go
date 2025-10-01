package main

// Requires ffplay (https://ffmpeg.org/) on PATH.
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/examples"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
	"github.com/sanity-io/litter"
)

func main() {
	model := examples.GetModel("openai-chat-completion", "gpt-4o-audio-preview")

	stream, err := model.Stream(context.Background(), &llmsdk.LanguageModelInput{
		Modalities: []llmsdk.Modality{llmsdk.ModalityText, llmsdk.ModalityAudio},
		Audio: &llmsdk.AudioOptions{
			Format: ptr.To(llmsdk.AudioFormatLinear16),
			Voice:  ptr.To("alloy"),
		},
		Messages: []llmsdk.Message{
			llmsdk.NewUserMessage(
				llmsdk.NewTextPart("Is a golden retriever a good family dog?"),
			),
		},
	})
	if err != nil {
		log.Fatalf("Stream failed: %v", err)
	}

	var (
		ffplayCmd   *exec.Cmd
		ffplayStdin io.WriteCloser
		sampleRate  int
		channels    int
	)

	accumulator := llmsdk.NewStreamAccumulator()

	for stream.Next() {
		current := stream.Current()
		litter.Dump(redactPartial(current))

		if current.Delta != nil && current.Delta.Part.AudioPartDelta != nil {
			delta := current.Delta.Part.AudioPartDelta
			if delta.Format != nil && *delta.Format != llmsdk.AudioFormatLinear16 {
				log.Fatalf("unsupported audio format: %s", *delta.Format)
			}
			if delta.Data != nil {
				pcm, err := base64.StdEncoding.DecodeString(*delta.Data)
				if err != nil {
					log.Fatalf("Failed to decode audio: %v", err)
				}

				if sampleRate == 0 {
					if delta.SampleRate != nil {
						sampleRate = int(*delta.SampleRate)
					} else {
						sampleRate = 24_000
					}
				}
				if channels == 0 {
					if delta.Channels != nil {
						channels = int(*delta.Channels)
					} else {
						channels = 1
					}
				}

				if ffplayStdin == nil {
					ffplayCmd, ffplayStdin = startFfplay(sampleRate, channels)
					log.Printf(
						"Streaming audio with ffplay (%d Hz, %d channel%s).",
						sampleRate,
						channels,
						pluralSuffix(channels),
					)
				}

				if _, err := ffplayStdin.Write(pcm); err != nil {
					log.Fatalf("Failed to write audio: %v", err)
				}
			}
		}

		if err := accumulator.AddPartial(*current); err != nil {
			log.Printf("Failed to add partial: %v", err)
		}
	}

	if err := stream.Err(); err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	if ffplayStdin != nil {
		finishFfplay(ffplayCmd, ffplayStdin)
	}

	finalResponse, err := accumulator.ComputeResponse()
	if err != nil {
		log.Fatalf("Failed to compute response: %v", err)
	}

	litter.Dump(finalResponse)
}

func startFfplay(sampleRate, channels int) (*exec.Cmd, io.WriteCloser) {
	args := []string{
		"-loglevel", "error",
		"-autoexit",
		"-nodisp",
		"-f", "s16le",
		"-ar", strconv.Itoa(sampleRate),
		"-i", "pipe:0",
		"-af", fmt.Sprintf("aformat=channel_layouts=%s", channelLayout(channels)),
	}

	cmd := exec.Command("ffplay", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to open ffplay stdin: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start ffplay: %v", err)
	}

	return cmd, stdin
}

func finishFfplay(cmd *exec.Cmd, stdin io.WriteCloser) {
	if err := stdin.Close(); err != nil {
		log.Fatalf("Failed to close ffplay stdin: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		log.Fatalf("ffplay exited with error: %v", err)
	}
}

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func channelLayout(channels int) string {
	if channels <= 1 {
		return "mono"
	}
	return "stereo"
}

func redactPartial(partial *llmsdk.PartialModelResponse) any {
	if partial == nil {
		return nil
	}

	bytes, err := json.Marshal(partial)
	if err != nil {
		return partial
	}

	var data any
	if err := json.Unmarshal(bytes, &data); err != nil {
		return partial
	}

	return redactAudioFields(data)
}

func redactAudioFields(value any) any {
	switch v := value.(type) {
	case map[string]any:
		if raw, ok := v["data"].(string); ok {
			decoded, err := base64.StdEncoding.DecodeString(raw)
			if err == nil {
				v["data"] = fmt.Sprintf("[%d bytes]", len(decoded))
			} else {
				v["data"] = "[invalid data]"
			}
		}
		for key, val := range v {
			v[key] = redactAudioFields(val)
		}
		return v
	case []any:
		for i, item := range v {
			v[i] = redactAudioFields(item)
		}
		return v
	default:
		return value
	}
}
