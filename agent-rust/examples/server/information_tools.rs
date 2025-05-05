use async_trait::async_trait;
use llm_agent::{AgentTool, AgentToolResult};
use llm_sdk::{JSONSchema, Part};
use serde::Deserialize;
use serde_json::Value;
use std::error::Error;

use crate::context::MyContext;

// Information Tools
#[derive(Deserialize)]
struct SearchWikipediaParams {
    query: String,
    limit: Option<i32>,
}

pub struct SearchWikipediaTool;

#[async_trait]
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
                    "description": "Search query for Wikipedia"
                },
                "limit": {
                    "type": ["integer", "null"],
                    "description": "Maximum number of results to return (default: 5)"
                }
            },
            "required": ["query", "limit"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: Value,
        _context: &MyContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: SearchWikipediaParams = serde_json::from_value(args)?;
        let _limit = params.limit.unwrap_or(5);

        let search_url = format!(
            "https://en.wikipedia.org/api/rest_v1/page/summary/{}",
            urlencoding::encode(&params.query)
        );

        let client = reqwest::Client::new();

        match client.get(&search_url).send().await {
            Ok(response) => {
                if !response.status().is_success() {
                    return Ok(AgentToolResult {
                        content: vec![Part::text(format!(
                            "Failed to search Wikipedia for '{}'",
                            params.query
                        ))],
                        is_error: true,
                    });
                }

                match response.json::<Value>().await {
                    Ok(data) => {
                        let mut result = serde_json::Map::new();
                        result.insert("query".to_string(), Value::String(params.query.clone()));

                        if let Some(title) = data.get("title") {
                            result.insert("title".to_string(), title.clone());
                        }
                        if let Some(extract) = data.get("extract") {
                            result.insert("summary".to_string(), extract.clone());
                        }
                        if let Some(page_url) = data
                            .get("content_urls")
                            .and_then(|u| u.get("desktop"))
                            .and_then(|d| d.get("page"))
                        {
                            result.insert("url".to_string(), page_url.clone());
                        }

                        Ok(AgentToolResult {
                            content: vec![Part::text(serde_json::to_string_pretty(
                                &Value::Object(result),
                            )?)],
                            is_error: false,
                        })
                    }
                    Err(e) => Ok(AgentToolResult {
                        content: vec![Part::text(format!("Error: {e}"))],
                        is_error: true,
                    }),
                }
            }
            Err(e) => Ok(AgentToolResult {
                content: vec![Part::text(format!("Error: {e}"))],
                is_error: true,
            }),
        }
    }
}

#[derive(Deserialize)]
struct GetNewsParams {
    query: Option<String>,
    category: Option<String>,
    country: Option<String>,
    language: Option<String>,
    page_size: Option<i32>,
}

pub struct GetNewsTool;

#[async_trait]
impl AgentTool<MyContext> for GetNewsTool {
    fn name(&self) -> String {
        "get_news".to_string()
    }

    fn description(&self) -> String {
        "Get latest news articles based on query or category".to_string()
    }

    fn parameters(&self) -> JSONSchema {
        serde_json::json!({
            "type": "object",
            "properties": {
                "query": {
                    "type": ["string", "null"],
                    "description": "Search query for news articles"
                },
                "category": {
                    "type": ["string", "null"],
                    "description": "News category (business, entertainment, general, health, science, sports, technology)"
                },
                "country": {
                    "type": ["string", "null"],
                    "description": "Country code for news (default: us)"
                },
                "language": {
                    "type": ["string", "null"],
                    "description": "Language code for news (default: en)"
                },
                "page_size": {
                    "type": ["integer", "null"],
                    "description": "Number of articles to return (default: 5, max: 20)"
                }
            },
            "required": ["query", "category", "country", "language", "page_size"],
            "additionalProperties": false
        })
    }

    async fn execute(
        &self,
        args: Value,
        context: &MyContext,
        _state: &llm_agent::RunState,
    ) -> Result<AgentToolResult, Box<dyn Error + Send + Sync>> {
        let params: GetNewsParams = serde_json::from_value(args)?;

        let api_key = context
            .news_api_key
            .as_ref()
            .ok_or("News API key not provided")?;

        let mut url = String::from("https://newsapi.org/v2/top-headlines?");

        if let Some(query) = &params.query {
            url.push_str(&format!("q={}&", urlencoding::encode(query)));
        }
        if let Some(category) = &params.category {
            url.push_str(&format!("category={category}&"));
        }

        let country = params.country.as_deref().unwrap_or("us");
        let language = params.language.as_deref().unwrap_or("en");
        let page_size = params.page_size.unwrap_or(5).min(20);

        url.push_str(&format!(
            "country={country}&language={language}&pageSize={page_size}&apiKey={api_key}"
        ));

        let client = reqwest::Client::new();

        match client.get(&url).send().await {
            Ok(response) => {
                if !response.status().is_success() {
                    return Ok(AgentToolResult {
                        content: vec![Part::text("Failed to fetch news".to_string())],
                        is_error: true,
                    });
                }

                match response.json::<Value>().await {
                    Ok(data) => {
                        if let Some(articles) = data.get("articles").and_then(|a| a.as_array()) {
                            let simplified_articles: Vec<Value> = articles
                                .iter()
                                .map(|article| {
                                    serde_json::json!({
                                        "title": article.get("title"),
                                        "description": article.get("description"),
                                        "url": article.get("url"),
                                        "published_at": article.get("publishedAt"),
                                        "source": article.get("source").and_then(|s| s.get("name"))
                                    })
                                })
                                .collect();

                            let result = serde_json::json!({
                                "articles": simplified_articles,
                                "total_results": data.get("totalResults")
                            });

                            Ok(AgentToolResult {
                                content: vec![Part::text(serde_json::to_string_pretty(&result)?)],
                                is_error: false,
                            })
                        } else {
                            Ok(AgentToolResult {
                                content: vec![Part::text("No articles found".to_string())],
                                is_error: true,
                            })
                        }
                    }
                    Err(e) => Ok(AgentToolResult {
                        content: vec![Part::text(format!("Error: {e}"))],
                        is_error: true,
                    }),
                }
            }
            Err(e) => Ok(AgentToolResult {
                content: vec![Part::text(format!("Error: {e}"))],
                is_error: true,
            }),
        }
    }
}
