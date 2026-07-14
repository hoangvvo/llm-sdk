package llmsdk

import (
	"encoding/json"
	"testing"
)

func TestWebSearchToolJSONRoundTrip(t *testing.T) {
	city := "San Francisco"
	region := "California"
	country := "US"
	timezone := "America/Los_Angeles"
	tool := NewWebSearchTool(
		WithWebSearchAllowedDomains("example.com"),
		WithWebSearchUserLocation(WebSearchUserLocation{
			City:     &city,
			Region:   &region,
			Country:  &country,
			Timezone: &timezone,
		}),
	)

	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded Tool
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.Type() != ToolTypeWebSearch {
		t.Fatalf("Type() = %q, want %q", decoded.Type(), ToolTypeWebSearch)
	}
	if decoded.WebSearchTool == nil {
		t.Fatal("WebSearchTool is nil")
	}
	if len(decoded.WebSearchTool.AllowedDomains) != 1 || decoded.WebSearchTool.AllowedDomains[0] != "example.com" {
		t.Fatalf("AllowedDomains = %v, want [example.com]", decoded.WebSearchTool.AllowedDomains)
	}
	if decoded.WebSearchTool.UserLocation == nil || decoded.WebSearchTool.UserLocation.Country == nil || *decoded.WebSearchTool.UserLocation.Country != "US" {
		t.Fatalf("UserLocation = %#v, want country US", decoded.WebSearchTool.UserLocation)
	}
}
