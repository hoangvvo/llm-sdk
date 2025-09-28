package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	llmagent "github.com/hoangvvo/llm-sdk/agent-go"
	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
)

type SearchWikipediaParams struct {
	Query         string `json:"query"`
	Language      string `json:"language"`
	Limit         int    `json:"limit"`
	ExtractLength int    `json:"extract_length"`
}

type SearchWikipediaTool struct{}

func (t *SearchWikipediaTool) Name() string {
	return "search_wikipedia"
}

func (t *SearchWikipediaTool) Description() string {
	return "Search Wikipedia for information on a topic"
}

func (t *SearchWikipediaTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query or article title",
			},
			"language": map[string]any{
				"type":        "string",
				"description": "Wikipedia language edition",
				"default":     "en",
			},
			"limit": map[string]any{
				"type":        "number",
				"minimum":     1,
				"maximum":     10,
				"description": "Maximum number of results to return",
				"default":     3,
			},
			"extract_length": map[string]any{
				"type":        "number",
				"minimum":     50,
				"maximum":     1200,
				"description": "Number of characters for article extract",
				"default":     500,
			},
		},
		"required":             []string{"query", "language", "limit", "extract_length"},
		"additionalProperties": false,
	}
}

func (t *SearchWikipediaTool) Execute(ctx context.Context, paramsJSON json.RawMessage, _ *MyContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params SearchWikipediaParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	if params.Language == "" {
		params.Language = "en"
	}
	if params.Limit == 0 {
		params.Limit = 3
	}
	if params.ExtractLength == 0 {
		params.ExtractLength = 500
	}

	searchURL := fmt.Sprintf("https://%s.wikipedia.org/w/api.php", params.Language)
	searchParams := url.Values{}
	searchParams.Set("action", "opensearch")
	searchParams.Set("search", params.Query)
	searchParams.Set("limit", fmt.Sprintf("%d", params.Limit))
	searchParams.Set("namespace", "0")
	searchParams.Set("format", "json")

	searchResp, err := http.Get(fmt.Sprintf("%s?%s", searchURL, searchParams.Encode()))
	if err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}
	defer searchResp.Body.Close()

	if searchResp.StatusCode != http.StatusOK {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("Failed to search Wikipedia"),
			},
			IsError: true,
		}, nil
	}

	searchBody, err := io.ReadAll(searchResp.Body)
	if err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	var searchData []any
	if err := json.Unmarshal(searchBody, &searchData); err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	if len(searchData) < 2 {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf(`{"results": [], "query": "%s"}`, params.Query)),
			},
			IsError: false,
		}, nil
	}

	titles := searchData[1].([]any)
	if len(titles) == 0 {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf(`{"results": [], "query": "%s"}`, params.Query)),
			},
			IsError: false,
		}, nil
	}

	titlesList := make([]string, len(titles))
	for i, t := range titles {
		titlesList[i] = t.(string)
	}
	titlesStr := strings.Join(titlesList, "|")

	extractParams := url.Values{}
	extractParams.Set("action", "query")
	extractParams.Set("prop", "extracts")
	extractParams.Set("exintro", "true")
	extractParams.Set("explaintext", "true")
	extractParams.Set("exchars", fmt.Sprintf("%d", params.ExtractLength))
	extractParams.Set("titles", titlesStr)
	extractParams.Set("format", "json")

	extractResp, err := http.Get(fmt.Sprintf("%s?%s", searchURL, extractParams.Encode()))
	if err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}
	defer extractResp.Body.Close()

	if extractResp.StatusCode != http.StatusOK {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Request failed with status %d", extractResp.StatusCode)),
			},
			IsError: true,
		}, nil
	}

	extractBody, err := io.ReadAll(extractResp.Body)
	if err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	var extractData map[string]any
	if err := json.Unmarshal(extractBody, &extractData); err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	results := []map[string]string{}
	if query, ok := extractData["query"].(map[string]any); ok {
		if pages, ok := query["pages"].(map[string]any); ok {
			for pageID, pageData := range pages {
				if pageID != "-1" {
					page := pageData.(map[string]any)
					title := page["title"].(string)
					extract := ""
					if e, ok := page["extract"].(string); ok {
						extract = e
					}
					results = append(results, map[string]string{
						"title":   title,
						"extract": extract,
						"url":     fmt.Sprintf("https://%s.wikipedia.org/wiki/%s", params.Language, strings.ReplaceAll(title, " ", "_")),
					})
				}
			}
		}
	}

	result := map[string]any{
		"results": results,
		"query":   params.Query,
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(resultJSON)),
		},
		IsError: false,
	}, nil
}

type GetNewsParams struct {
	Query    string `json:"query"`
	Category string `json:"category"`
	Country  string `json:"country"`
	Language string `json:"language"`
	SortBy   string `json:"sort_by"`
	Limit    int    `json:"limit"`
}

type GetNewsTool struct{}

func (t *GetNewsTool) Name() string {
	return "get_news"
}

func (t *GetNewsTool) Description() string {
	return "Get current news articles based on search criteria"
}

func (t *GetNewsTool) Parameters() llmsdk.JSONSchema {
	return llmsdk.JSONSchema{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"maxLength":   500,
				"description": "Keywords or phrases to search for",
				"default":     "",
			},
			"category": map[string]any{
				"type":        "string",
				"enum":        []string{"business", "entertainment", "general", "health", "science", "sports", "technology"},
				"default":     "general",
				"description": "News category filter",
			},
			"country": map[string]any{
				"type":        "string",
				"description": "ISO 2-letter country code",
			},
			"language": map[string]any{
				"type":        "string",
				"default":     "en",
				"description": "ISO 2-letter language code",
			},
			"sort_by": map[string]any{
				"type":        "string",
				"enum":        []string{"relevancy", "popularity", "publishedAt"},
				"default":     "publishedAt",
				"description": "Sort order for results",
			},
			"limit": map[string]any{
				"type":        "number",
				"minimum":     1,
				"maximum":     100,
				"default":     5,
				"description": "Number of articles to return",
			},
		},
		"required":             []string{"query", "category", "country", "language", "sort_by", "limit"},
		"additionalProperties": false,
	}
}

func (t *GetNewsTool) Execute(ctx context.Context, paramsJSON json.RawMessage, context *MyContext, _ *llmagent.RunState) (llmagent.AgentToolResult, error) {
	var params GetNewsParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return llmagent.AgentToolResult{}, err
	}

	if params.Category == "" {
		params.Category = "general"
	}
	if params.Language == "" {
		params.Language = "en"
	}
	if params.SortBy == "" {
		params.SortBy = "publishedAt"
	}
	if params.Limit == 0 {
		params.Limit = 5
	}

	apiKey := ""
	if context.NewsAPIKey != nil {
		apiKey = *context.NewsAPIKey
	}
	if apiKey == "" {
		apiKey = os.Getenv("NEWS_API_KEY")
	}

	if apiKey == "" {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart("API key required. Get one free at newsapi.org"),
			},
			IsError: true,
		}, nil
	}

	baseURL := "https://newsapi.org/v2/"
	headers := map[string]string{
		"X-Api-Key": apiKey,
	}

	var endpoint string
	urlParams := url.Values{}

	if params.Query != "" {
		endpoint = "everything"
		urlParams.Set("q", params.Query)
		urlParams.Set("language", params.Language)
		urlParams.Set("sortBy", params.SortBy)
		urlParams.Set("pageSize", fmt.Sprintf("%d", params.Limit))
	} else {
		endpoint = "top-headlines"
		urlParams.Set("category", params.Category)
		if params.Country != "" {
			urlParams.Set("country", params.Country)
		} else {
			urlParams.Set("country", "us")
		}
		urlParams.Set("pageSize", fmt.Sprintf("%d", params.Limit))
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s?%s", baseURL, endpoint, urlParams.Encode()), nil)
	if err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
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
				llmsdk.NewTextPart(fmt.Sprintf("Request failed with status %d", resp.StatusCode)),
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

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return llmagent.AgentToolResult{
			Content: []llmsdk.Part{
				llmsdk.NewTextPart(fmt.Sprintf("Error: %v", err)),
			},
			IsError: true,
		}, nil
	}

	articles := []map[string]any{}
	if articlesData, ok := data["articles"].([]any); ok {
		for i, article := range articlesData {
			if i >= params.Limit {
				break
			}
			a := article.(map[string]any)
			articleMap := map[string]any{
				"title":        a["title"],
				"description":  a["description"],
				"url":          a["url"],
				"published_at": a["publishedAt"],
				"author":       a["author"],
			}
			if source, ok := a["source"].(map[string]any); ok {
				articleMap["source"] = source["name"]
			}
			articles = append(articles, articleMap)
		}
	}

	result := map[string]any{
		"articles":      articles,
		"total_results": data["totalResults"],
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")

	return llmagent.AgentToolResult{
		Content: []llmsdk.Part{
			llmsdk.NewTextPart(string(resultJSON)),
		},
		IsError: false,
	}, nil
}
