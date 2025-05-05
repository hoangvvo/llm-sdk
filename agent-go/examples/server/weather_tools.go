package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

type GetCoordinatesParams struct {
	Location string `json:"location"`
}

type GetCoordinatesTool struct{}

func (t *GetCoordinatesTool) Name() string {
	return "get_coordinates"
}

func (t *GetCoordinatesTool) Description() string {
	return "Get coordinates (latitude and longitude) from a location name"
}

func (t *GetCoordinatesTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]any{
				"type":        "string",
				"description": "The location name, e.g. Paris, France",
			},
		},
		"required":             []string{"location"},
		"additionalProperties": false,
	}
}

func (t *GetCoordinatesTool) Execute(ctx context.Context, paramsJSON json.RawMessage, context *MyContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params GetCoordinatesParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	apiKey := ""
	if context.GeoAPIKey != nil {
		apiKey = *context.GeoAPIKey
	}
	if apiKey == "" {
		apiKey = os.Getenv("GEO_API_KEY")
	}

	if apiKey == "" {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("API Key not provided. You can also provide the value on the UI with the Context field 'geo_api_key'. Get a free API key at https://geocode.maps.co/"),
			},
			IsError: true,
		}, nil
	}

	u, _ := url.Parse("https://geocode.maps.co/search")
	q := u.Query()
	q.Set("q", params.Location)
	q.Set("api_key", apiKey)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error fetching coordinates: %d %s", resp.StatusCode, resp.Status)),
			},
			IsError: true,
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	if len(items) == 0 {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("No coordinates found for location: %s", params.Location)),
			},
			IsError: true,
		}, nil
	}

	lat := items[0]["lat"]
	lon := items[0]["lon"]

	result := map[string]any{
		"latitude":  lat,
		"longitude": lon,
	}

	resultJSON, _ := json.Marshal(result)

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(resultJSON)),
		},
		IsError: false,
	}, nil
}

type GetWeatherParams struct {
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
	Units     string `json:"units"`
	Timesteps string `json:"timesteps"`
	StartTime string `json:"startTime"`
}

type GetWeatherTool struct{}

func (t *GetWeatherTool) Name() string {
	return "get_weather"
}

func (t *GetWeatherTool) Description() string {
	return "Get current weather from latitude and longitude"
}

func (t *GetWeatherTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"latitude": map[string]any{
				"type":        "string",
				"description": "The latitude",
			},
			"longitude": map[string]any{
				"type":        "string",
				"description": "The longitude",
			},
			"units": map[string]any{
				"type":        "string",
				"enum":        []string{"metric", "imperial"},
				"description": "Units",
			},
			"timesteps": map[string]any{
				"type":        "string",
				"enum":        []string{"current", "1h", "1d"},
				"description": "Timesteps",
			},
			"startTime": map[string]any{
				"type":        "string",
				"description": "Start time in ISO format",
			},
		},
		"required":             []string{"latitude", "longitude", "units", "timesteps", "startTime"},
		"additionalProperties": false,
	}
}

func (t *GetWeatherTool) Execute(ctx context.Context, paramsJSON json.RawMessage, context *MyContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params GetWeatherParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	apiKey := ""
	if context.TomorrowAPIKey != nil {
		apiKey = *context.TomorrowAPIKey
	}
	if apiKey == "" {
		apiKey = os.Getenv("TOMORROW_API_KEY")
	}

	if apiKey == "" {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("API Key not provided. You can also provide the value on the UI with the Context field 'tomorrow_api_key'. Get a free API key at https://tomorrow.io/"),
			},
			IsError: true,
		}, nil
	}

	fields := "temperature,temperatureApparent,humidity"

	u, _ := url.Parse("https://api.tomorrow.io/v4/timelines")
	q := u.Query()
	q.Set("location", fmt.Sprintf("%s,%s", params.Latitude, params.Longitude))
	q.Set("fields", fields)
	q.Set("timesteps", params.Timesteps)
	q.Set("units", params.Units)
	q.Set("startTime", params.StartTime)
	q.Set("apikey", apiKey)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error fetching weather: %d %s", resp.StatusCode, resp.Status)),
			},
			IsError: true,
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(body)),
		},
		IsError: false,
	}, nil
}