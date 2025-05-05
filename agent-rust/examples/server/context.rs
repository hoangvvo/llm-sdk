use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct MyContext {
    pub name: Option<String>,
    pub location: Option<String>,
    pub language: Option<String>,
    pub geo_api_key: Option<String>,
    pub tomorrow_api_key: Option<String>,
    pub news_api_key: Option<String>,
}
