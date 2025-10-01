package main

type MyContext struct {
	Name           *string `json:"name,omitempty"`
	Location       *string `json:"location,omitempty"`
	Language       *string `json:"language,omitempty"`
	GeoAPIKey      *string `json:"geo_api_key,omitempty"`
	TomorrowAPIKey *string `json:"tomorrow_api_key,omitempty"`
	NewsAPIKey     *string `json:"news_api_key,omitempty"`
	// Client-managed artifacts store (server reads only)
	Artifacts []Artifact `json:"artifacts,omitempty"`
}

type ArtifactKind string

const (
	ArtifactKindMarkdown ArtifactKind = "markdown"
	ArtifactKindText     ArtifactKind = "text"
	ArtifactKindCode     ArtifactKind = "code"
)

type Artifact struct {
	ID        string       `json:"id"`
	Title     string       `json:"title"`
	Kind      ArtifactKind `json:"kind"`
	Content   string       `json:"content"`
	Version   *int         `json:"version,omitempty"`
	UpdatedAt *string      `json:"updated_at,omitempty"`
}
