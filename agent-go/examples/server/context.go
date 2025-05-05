package main

type MyContext struct {
	Name           *string `json:"name,omitempty"`
	Location       *string `json:"location,omitempty"`
	Language       *string `json:"language,omitempty"`
	GeoAPIKey      *string `json:"geo_api_key,omitempty"`
	TomorrowAPIKey *string `json:"tomorrow_api_key,omitempty"`
	NewsAPIKey     *string `json:"news_api_key,omitempty"`
}