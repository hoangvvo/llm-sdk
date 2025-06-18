use crate::context::MyContext;
use futures::future::BoxFuture;
use llm_agent::{AgentTool, AgentToolResult};
use llm_sdk::{JSONSchema, Part};
use serde::Deserialize;
use serde_json::Value;
use std::error::Error;

// Information Tools
#[derive(Deserialize)]
struct SearchWikipediaParams {
    query: String,
    #[serde(default = "default_language")]
    language: String,
    #[serde(default = "default_limit")]
    limit: u32,
    #[serde(default = "default_extract_length")]
    extract_length: u32,
}

fn default_language() -> String {
    "en".to_string()
}

fn default_limit() -> u32 {
    3
}

fn default_extract_length() -> u32 {
    500
}

pub struct SearchWikipediaTool;

impl AgentTool<MyContext> for SearchWikipediaTool {
    fn name(&self) -> String {
        "search_wikipedia".to_string()
    }

    fn description(&self) -> String {
        "Search Wikipedia for information on a topic".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "query": {
                    "type": "string",
                    "description": "Search query or article title"
                },
                "language": {
                    "type": "string",
                    "description": "Wikipedia language edition",
                    "default": "en"
                },
                "limit": {
                    "type": "number",
                    "description": "Maximum number of results to return",
                    "minimum": 1,
                    "maximum": 10,
                    "default": 3
                },
                "extract_length": {
                    "type": "number",
                    "description": "Number of characters for article extract",
                    "minimum": 50,
                    "maximum": 1200,
                    "default": 500
                }
            },
            "required": ["query", "language", "limit", "extract_length"],
            "additionalProperties": false
        })
    }

    fn execute<'a>(
        &'a self,
        args: Value,
        _context: &'a MyContext,
        _state: &'a llm_agent::RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn Error + Send + Sync>>> {
        Box::pin(async move {
            let params: SearchWikipediaParams = serde_json::from_value(args)?;

            let client = reqwest::Client::new();

            // Search for pages
            let search_url = format!("https://{}.wikipedia.org/w/api.php", params.language);
            let search_params = [
                ("action", "opensearch"),
                ("search", &params.query),
                ("limit", &params.limit.to_string()),
                ("namespace", "0"),
                ("format", "json"),
            ];

            let search_response = client.get(&search_url).query(&search_params).send().await?;

            if !search_response.status().is_success() {
                return Ok(AgentToolResult {
                    content: vec![Part::text("Failed to search Wikipedia".to_string())],
                    is_error: true,
                });
            }

            let search_data: Vec<Value> = search_response.json().await?;

            if search_data.len() < 2 || search_data[1].as_array().map_or(true, |a| a.is_empty()) {
                return Ok(AgentToolResult {
                    content: vec![Part::text(serde_json::to_string(&serde_json::json!({
                        "results": [],
                        "query": params.query
                    }))?)],
                    is_error: false,
                });
            }

            let titles = search_data[1]
                .as_array()
                .unwrap()
                .iter()
                .filter_map(|v| v.as_str())
                .collect::<Vec<_>>()
                .join("|");

            // Get extracts for found pages
            let extract_params = [
                ("action", "query"),
                ("prop", "extracts"),
                ("exintro", "true"),
                ("explaintext", "true"),
                ("exchars", &params.extract_length.to_string()),
                ("titles", &titles),
                ("format", "json"),
            ];

            let extract_response = client
                .get(&search_url)
                .query(&extract_params)
                .send()
                .await?;

            if !extract_response.status().is_success() {
                return Ok(AgentToolResult {
                    content: vec![Part::text(format!(
                        "Request failed with status {}",
                        extract_response.status().as_u16()
                    ))],
                    is_error: true,
                });
            }

            let extract_data: Value = extract_response.json().await?;

            let mut results = Vec::new();

            if let Some(pages) = extract_data
                .get("query")
                .and_then(|q| q.get("pages"))
                .and_then(|p| p.as_object())
            {
                for (page_id, page) in pages {
                    if page_id != "-1" {
                        if let Some(title) = page.get("title").and_then(|t| t.as_str()) {
                            let extract =
                                page.get("extract").and_then(|e| e.as_str()).unwrap_or("");
                            let url = format!(
                                "https://{}.wikipedia.org/wiki/{}",
                                params.language,
                                title.replace(' ', "_")
                            );

                            results.push(serde_json::json!({
                                "title": title,
                                "extract": extract,
                                "url": url
                            }));
                        }
                    }
                }
            }

            Ok(AgentToolResult {
                content: vec![Part::text(serde_json::to_string_pretty(
                    &serde_json::json!({
                        "results": results,
                        "query": params.query
                    }),
                )?)],
                is_error: false,
            })
        })
    }
}

#[derive(Deserialize)]
struct GetNewsParams {
    query: Option<String>,
    #[serde(default = "default_category")]
    category: String,
    country: Option<String>,
    #[serde(default = "default_news_language")]
    language: String,
    #[serde(default = "default_sort_by")]
    sort_by: String,
    #[serde(default = "default_limit_news")]
    limit: u32,
}

fn default_category() -> String {
    "general".to_string()
}

fn default_news_language() -> String {
    "en".to_string()
}

fn default_sort_by() -> String {
    "publishedAt".to_string()
}

fn default_limit_news() -> u32 {
    5
}

pub struct GetNewsTool;

impl AgentTool<MyContext> for GetNewsTool {
    fn name(&self) -> String {
        "get_news".to_string()
    }

    fn description(&self) -> String {
        "Get current news articles based on search criteria".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "query": {
                    "type": ["string", "null"],
                    "description": "Keywords or phrases to search for",
                    "maxLength": 500,
                    "default": null
                },
                "category": {
                    "type": "string",
                    "enum": ["business", "entertainment", "general", "health", "science", "sports", "technology"],
                    "description": "News category filter",
                    "default": "general"
                },
                "country": {
                    "type": ["string", "null"],
                    "description": "ISO 2-letter country code",
                    "default": null
                },
                "language": {
                    "type": "string",
                    "description": "ISO 2-letter language code",
                    "default": "en"
                },
                "sort_by": {
                    "type": "string",
                    "enum": ["relevancy", "popularity", "publishedAt"],
                    "description": "Sort order for results",
                    "default": "publishedAt"
                },
                "limit": {
                    "type": "number",
                    "description": "Number of articles to return",
                    "minimum": 1,
                    "maximum": 100,
                    "default": 5
                }
            },
            "required": ["query", "category", "country", "language", "sort_by", "limit"],
            "additionalProperties": false
        })
    }

    fn execute<'a>(
        &'a self,
        args: Value,
        context: &'a MyContext,
        _state: &'a llm_agent::RunState,
    ) -> BoxFuture<'a, Result<AgentToolResult, Box<dyn Error + Send + Sync>>> {
        Box::pin(async move {
            let params: GetNewsParams = serde_json::from_value(args)?;

            let env_key = std::env::var("NEWS_API_KEY").ok();
            let api_key = context
                .news_api_key
                .as_ref()
                .or(env_key.as_ref())
                .ok_or("API key required. Get one free at newsapi.org")?;

            let client = reqwest::Client::new();

            let (endpoint, query_params) = if let Some(query) = params.query {
                (
                    "everything",
                    vec![
                        ("q", query),
                        ("language", params.language),
                        ("sortBy", params.sort_by),
                        ("pageSize", params.limit.to_string()),
                    ],
                )
            } else {
                (
                    "top-headlines",
                    vec![
                        ("category", params.category),
                        (
                            "country",
                            params.country.unwrap_or_else(|| "us".to_string()),
                        ),
                        ("pageSize", params.limit.to_string()),
                    ],
                )
            };

            let url = format!("https://newsapi.org/v2/{}", endpoint);

            let response = client
                .get(&url)
                .header("X-Api-Key", api_key)
                .query(&query_params)
                .send()
                .await?;

            if !response.status().is_success() {
                return Ok(AgentToolResult {
                    content: vec![Part::text(format!(
                        "Request failed with status {}",
                        response.status().as_u16()
                    ))],
                    is_error: true,
                });
            }

            let data: Value = response.json().await?;

            let articles = data
                .get("articles")
                .and_then(|a| a.as_array())
                .map(|articles| {
                    articles
                        .iter()
                        .take(params.limit as usize)
                        .map(|article| {
                            serde_json::json!({
                                "title": article.get("title"),
                                "description": article.get("description"),
                                "url": article.get("url"),
                                "source": article.get("source").and_then(|s| s.get("name")),
                                "published_at": article.get("publishedAt"),
                                "author": article.get("author")
                            })
                        })
                        .collect::<Vec<_>>()
                })
                .unwrap_or_default();

            Ok(AgentToolResult {
                content: vec![Part::text(serde_json::to_string_pretty(
                    &serde_json::json!({
                        "articles": articles,
                        "total_results": data.get("totalResults")
                    }),
                )?)],
                is_error: false,
            })
        })
    }
}
