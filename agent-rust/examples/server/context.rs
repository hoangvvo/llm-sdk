use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct MyContext {
    pub name: Option<String>,
    pub location: Option<String>,
    pub language: Option<String>,
    pub geo_api_key: Option<String>,
    pub tomorrow_api_key: Option<String>,
    pub news_api_key: Option<String>,
    // Client-managed artifacts store (server reads only)
    pub artifacts: Option<Vec<Artifact>>,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum ArtifactKind {
    Markdown,
    Text,
    Code,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct Artifact {
    pub id: String,
    pub title: String,
    pub kind: ArtifactKind,
    pub content: String,
    pub version: Option<i32>,
    pub updated_at: Option<String>,
}
