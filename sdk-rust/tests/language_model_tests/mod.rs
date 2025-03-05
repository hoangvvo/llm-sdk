use llm_sdk::{
    AssistantMessage, LanguageModel, LanguageModelInput, Message, Part, ResponseFormatJson,
    ResponseFormatOption, TextPart, Tool, ToolCallPart, ToolMessage, ToolResultPart, UserMessage,
};

fn tools() -> Vec<Tool> {
    vec![
        Tool {
            name: "get_weather".to_string(),
            description: "Get the weather".to_string(),
            parameters: serde_json::json!({
                "type": "object",
                "properties": {
                    "location": { "type": "string" },
                    "unit": { "type": "string", "enum": ["c", "f"] },
                },
                "required": ["location"],
            })
            .into(),
        },
        Tool {
            name: "get_stock_price".to_string(),
            description: "Get the stock price".to_string(),
            parameters: serde_json::json!({
                "type": "object",
                "properties": {
                    "symbol": { "type": "string" },
                },
                "required": ["symbol"],
            })
            .into(),
        },
    ]
}

fn complex_tool() -> Tool {
    Tool {
        name: "register_user".to_string(),
        description: "Register a user".to_string(),
        parameters: serde_json::json!({
            "type": "object",
            "properties": {
                "id": {
                    "type": "string",
                    "description": "Unique identifier in UUID format",
                },
                "name": {
                    "type": "string",
                    "minLength": 2,
                    "maxLength": 50,
                    "description": "The name of the user, between 2 and 50 characters",
                },
                "email": {
                    "type": "string",
                    "format": "email",
                    "description": "A valid email address",
                },
                "birthDate": {
                    "type": "string",
                    "format": "date",
                    "description": "Date of birth in YYYY-MM-DD format",
                },
                "age": {
                    "type": "integer",
                    "minimum": 0,
                    "maximum": 120,
                    "description": "Age of the user, must be between 0 and 120",
                },
                "isActive": {
                    "type": "boolean",
                    "default": true,
                    "description": "Indicates if the account is active",
                },
                "role": {
                    "type": "string",
                    "enum": ["user", "admin", "moderator"],
                    "description": "Role of the user in the system",
                },
                "accountBalance": {
                    "type": "number",
                    "minimum": 0,
                    "description": "User's account balance, must be greater than 0",
                },
                "phoneNumber": {
                    "type": "string",
                    "pattern": "^[+][0-9]{10,15}$",
                    "description": "Phone number in international format, e.g., +1234567890",
                },
                "tags": {
                    "type": "array",
                    "items": {
                        "type": "string",
                        "minLength": 1,
                        "maxLength": 20,
                    },
                    "uniqueItems": true,
                    "maxItems": 10,
                    "description": "An array of unique tags, each up to 20 characters long",
                },
                "lastLogin": {
                    "type": "string",
                    "format": "date-time",
                    "description": "The last login date and time",
                },
            },
            "required": ["id", "name", "email", "age", "isActive"],
            "additionalProperties": false,
        })
        .into(),
    }
}

pub async fn test_generate_text(language_model: Box<dyn LanguageModel>) {
    let response = language_model
        .generate(LanguageModelInput {
            messages: vec![Message::User(UserMessage {
                content: vec![Part::Text(TextPart {
                    text: "Hello".to_string(),
                    id: None,
                })],
            })],
            ..LanguageModelInput::default()
        })
        .await;

    let response = response
        .inspect_err(|err| {
            eprintln!("Response error: {:?}", err);
        })
        .unwrap();
    let part = response.content.first().unwrap();

    assert!(matches!(part, Part::Text(TextPart { text, .. }) if !text.is_empty()));
}

pub async fn test_generate_with_system_prompt(language_model: Box<dyn LanguageModel>) {
    let response = language_model
        .generate(LanguageModelInput {
            messages: vec![Message::User(UserMessage {
                content: vec![Part::Text(TextPart {
                    text: "Hello".to_string(),
                    id: None,
                })],
            })],
            system_prompt: Some("You must always start your message with '🤖'".to_string()),
            ..LanguageModelInput::default()
        })
        .await;

    let response = response
        .inspect_err(|err| {
            eprintln!("Response error: {:?}", err);
        })
        .unwrap();
    let part = response.content.first().unwrap();

    assert!(matches!(part, Part::Text(TextPart { text, .. }) if text.starts_with("🤖")));
}

pub async fn test_generate_tool_call(language_model: Box<dyn LanguageModel>) {
    let response = language_model
        .generate(LanguageModelInput {
            messages: vec![Message::User(UserMessage {
                content: vec![Part::Text(TextPart {
                    text: "What's the weather like in Boston today?".to_string(),
                    id: None,
                })],
            })],
            tools: tools().into(),
            ..LanguageModelInput::default()
        })
        .await;

    let response = response.unwrap();
    let tool_call_part = response
        .content
        .iter()
        .find(|part| matches!(part, Part::ToolCall(_)))
        .unwrap();

    assert!(matches!(tool_call_part, Part::ToolCall(
        ToolCallPart {
            tool_name,
            args,
            ..
        }
    ) if tool_name == "get_weather" && args.as_ref().unwrap()["location"].as_str().unwrap().contains("Boston")));
}

pub async fn test_generate_text_from_tool_result(language_model: Box<dyn LanguageModel>) {
    let response = language_model
        .generate(LanguageModelInput {
            messages: vec![
                Message::User(UserMessage {
                    content: vec![Part::Text(TextPart {
                        text: "What's the weather like in Boston today?".to_string(),
                        id: None,
                    })],
                }),
                Message::Assistant(AssistantMessage {
                    content: vec![Part::ToolCall(ToolCallPart {
                        tool_call_id: "0mbnj08nt".to_string(),
                        tool_name: "get_weather".to_string(),
                        args: serde_json::json!({ "location": "Boston" }).into(),
                        id: None,
                    })],
                }),
                Message::Tool(ToolMessage {
                    content: vec![ToolResultPart {
                        tool_call_id: "0mbnj08nt".to_string(),
                        tool_name: "get_weather".to_string(),
                        result: serde_json::json!({
                            "temperature": 70,
                            "unit": "f",
                            "description": "Sunny",
                        }),
                        is_error: None,
                    }],
                }),
            ],
            tools: vec![Tool {
                name: "get_weather".to_string(),
                description: "Get the weather".to_string(),
                parameters: serde_json::json!({
                    "type": "object",
                    "properties": {
                        "location": { "type": "string" },
                        "unit": { "type": "string", "enum": ["c", "f"] },
                    },
                    "required": ["location"],
                })
                .into(),
            }]
            .into(),
            ..LanguageModelInput::default()
        })
        .await;

    let response = response
        .inspect_err(|err| {
            eprintln!("Response error: {:?}", err);
        })
        .unwrap();
    let part = response.content.first().unwrap();

    assert!(matches!(part, Part::Text(TextPart { text, .. }) if !text.is_empty()));
}

pub async fn test_generate_tool_call_for_complex_schema(language_model: Box<dyn LanguageModel>) {
    let response = language_model
        .generate(LanguageModelInput {
            messages: vec![Message::User(UserMessage {
                content: vec![Part::Text(TextPart {
                    id: None,
                    text: "Hi, create a user with the id \"a1b2c3\", name \"John Doe\", email \"john.doe@example.com\", birthDate \"1990-05-15\", age 34, isActive true, role \"user\", accountBalance 500.75, phoneNumber \"+1234567890123\", tags [\"developer\", \"gamer\"], and lastLogin \"2024-11-09T10:30:00Z\".".to_string(),
                })],
            })],
            tools: vec![complex_tool()].into(),
            ..LanguageModelInput::default()
        })
        .await;

    let response = response
        .inspect_err(|err| {
            eprintln!("Response error: {:?}", err);
        })
        .unwrap();

    let tool_call_part = response
        .content
        .iter()
        .find(|part| matches!(part, Part::ToolCall(_)))
        .unwrap();

    assert!(
        matches!(tool_call_part, Part::ToolCall(ToolCallPart { tool_name, .. }) if tool_name == "register_user")
    );
}

pub async fn test_generate_response_format_json(language_model: Box<dyn LanguageModel>) {
    let response = language_model
        .generate(LanguageModelInput {
            messages: vec![Message::User(UserMessage {
                content: vec![Part::Text(TextPart {
                    text: "Hi, create a user with the id \"a1b2c3\", name \"John Doe\", email \"john.doe@example.com\", birthDate \"1990-05-15\", age 34, isActive true, role \"user\", accountBalance 500.75, phoneNumber \"+1234567890123\", tags [\"developer\", \"gamer\"], and lastLogin \"2024-11-09T10:30:00Z\".".to_string(),
                    id: None,
                })],
            })],
            response_format: Some(
                ResponseFormatOption::Json(ResponseFormatJson {
                    name: "user".to_string(),
                    description: None,
                    schema: serde_json::json!({
                        "type": "object",
                        "properties": {
                            "id": { "type": "string" },
                            "name": { "type": "string" },
                            "email": { "type": "string" },
                            "birthDate": { "type": "string" },
                            "age": { "type": "integer" },
                            "isActive": { "type": "boolean" },
                            "role": { "type": "string" },
                            "accountBalance": { "type": "number" },
                            "phoneNumber": { "type": "string" },
                            "tags": { "type": "array", "items": { "type": "string" } },
                            "lastLogin": { "type": "string" },
                        },
                        "required": ["id", "name", "email", "birthDate", "age", "isActive", "role", "accountBalance", "phoneNumber", "tags", "lastLogin"],
                        "additionalProperties": false,
                    })
                    .into(),
                })
            ),
            ..LanguageModelInput::default()
        })
        .await;

    let response = response
        .inspect_err(|err| {
            eprintln!("Response error: {:?}", err);
        })
        .unwrap();
    let part = response.content.first().unwrap();
    let text_part = match part {
        Part::Text(text_part) => text_part,
        _ => panic!("Expected text part"),
    };

    let json = serde_json::from_str::<serde_json::Value>(&text_part.text).unwrap();
    assert_eq!(json["id"], "a1b2c3");
    assert_eq!(json["name"], "John Doe");
    assert_eq!(json["email"], "john.doe@example.com");

    println!("JSON: {}", serde_json::to_string_pretty(&json).unwrap());
}
