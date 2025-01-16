#[macro_export]
macro_rules! generate_common_tests {
    (
        model_name: $model_name:ident,
    ) => {
        fn get_weather_tool() -> Tool {
            Tool {
                name: "get_weather".to_string(),
                description: "Get the weather".to_string(),
                parameters: json!({
                    "type": "object",
                    "properties": {
                        "location": { "type": "string" },
                        "unit": { "type": ["string", "null"], "enum": ["c", "f"] },
                    },
                    "required": ["location", "unit"],
                    "additionalProperties": false,
                }),
            }
        }

        fn get_stock_price_tool() -> Tool {
            Tool {
                name: "get_stock_price".to_string(),
                description: "Get the stock price".to_string(),
                parameters: json!({
                    "type": "object",
                    "properties": {
                        "symbol": { "type": "string" },
                    },
                    "required": ["symbol"],
                    "additionalProperties": false,
                }),
            }
        }

        #[test]
        async fn test_generate_text() -> Result<(), Box<dyn Error>> {
            let test_case = TestCase {
                input: LanguageModelInput {
                    messages: vec![Message::User(UserMessage {
                        content: vec![Part::Text(TextPart {
                            text: r#"Respond by saying "Hello""#.to_string(),
                            id: None,
                        })],
                    })],
                    ..Default::default()
                },
                method: TestMethod::Generate,
                output: OutputAssertion {
                    content: vec![PartAssertion::Text(TextPartAssertion {
                        text: Regex::new(r"Hello").unwrap(),
                    })],
                },
            };
            run_test_case(&*$model_name, test_case).await
        }

        #[test]
        async fn test_stream_text() -> Result<(), Box<dyn Error>> {
            let test_case = TestCase {
                input: LanguageModelInput {
                    messages: vec![Message::User(UserMessage {
                        content: vec![Part::Text(TextPart {
                            text: r#"Respond by saying "Hello""#.to_string(),
                            id: None,
                        })],
                    })],
                    ..Default::default()
                },
                method: TestMethod::Stream,
                output: OutputAssertion {
                    content: vec![PartAssertion::Text(TextPartAssertion {
                        text: Regex::new(r"Hello").unwrap(),
                    })],
                },
            };
            run_test_case(&*$model_name, test_case).await
        }

        #[test]
        async fn test_generate_with_system_prompt() -> Result<(), Box<dyn Error>> {
            let test_case = TestCase {
                input: LanguageModelInput {
                    system_prompt: Some(r#"You must always start your message with "🤖""#.to_string()),
                    messages: vec![Message::User(UserMessage {
                        content: vec![Part::Text(TextPart {
                            text: "Hello".to_string(),
                            id: None,
                        })],
                    })],
                    ..Default::default()
                },
                method: TestMethod::Generate,
                output: OutputAssertion {
                    content: vec![PartAssertion::Text(TextPartAssertion {
                        text: Regex::new(r"^🤖").unwrap(),
                    })],
                },
            };
            run_test_case(&*$model_name, test_case).await
        }

        #[test]
        async fn test_generate_tool_call() -> Result<(), Box<dyn Error>> {
            let test_case = TestCase {
                input: LanguageModelInput {
                    messages: vec![Message::User(UserMessage {
                        content: vec![Part::Text(TextPart {
                            text: "What's the weather like in Boston today?".to_string(),
                            id: None,
                        })],
                    })],
                    tools: Some(vec![get_weather_tool()]),
                    ..Default::default()
                },
                method: TestMethod::Generate,
                output: OutputAssertion {
                    content: vec![PartAssertion::ToolCall(ToolCallPartAssertion {
                        tool_name: "get_weather".to_string(),
                        args: vec![(
                            "location".to_string(),
                            ToolCallpartAssertionArgPropValue::Value(Regex::new(r"Boston").unwrap()),
                        )],
                    })],
                },
            };
            run_test_case(&*$model_name, test_case).await
        }

        #[test]
        async fn test_stream_tool_call() -> Result<(), Box<dyn Error>> {
            let test_case = TestCase {
                input: LanguageModelInput {
                    messages: vec![Message::User(UserMessage {
                        content: vec![Part::Text(TextPart {
                            text: "What's the weather like in Boston today?".to_string(),
                            id: None,
                        })],
                    })],
                    tools: Some(vec![get_weather_tool()]),
                    ..Default::default()
                },
                method: TestMethod::Stream,
                output: OutputAssertion {
                    content: vec![PartAssertion::ToolCall(ToolCallPartAssertion {
                        tool_name: "get_weather".to_string(),
                        args: vec![(
                            "location".to_string(),
                            ToolCallpartAssertionArgPropValue::Value(Regex::new(r"Boston").unwrap()),
                        )],
                    })],
                },
            };
            run_test_case(&*$model_name, test_case).await
        }

        #[test]
        async fn test_generate_text_from_tool_result() -> Result<(), Box<dyn Error>> {
            let test_case = TestCase {
                input: LanguageModelInput {
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
                                args: json!({
                                    "location": "Boston",
                                }),
                                id: None,
                            })],
                        }),
                        Message::Tool(ToolMessage {
                            content: vec![Part::ToolResult(ToolResultPart {
                                tool_call_id: "0mbnj08nt".to_string(),
                                tool_name: "get_weather".to_string(),
                                content: vec![Part::Text(TextPart {
                                    text: json!({
                                        "temperature": 70,
                                        "unit": "f",
                                        "description": "Sunny",
                                    })
                                    .to_string(),
                                    id: None,
                                })],
                                is_error: None,
                            })],
                        }),
                    ],
                    tools: Some(vec![get_weather_tool()]),
                    ..Default::default()
                },
                method: TestMethod::Generate,
                output: OutputAssertion {
                    content: vec![PartAssertion::Text(TextPartAssertion {
                        text: Regex::new(r"(?i)70.*sunny|sunny.*70").unwrap(),
                    })],
                },
            };
            run_test_case(&*$model_name, test_case).await
        }

        #[test]
        async fn test_stream_text_from_tool_result() -> Result<(), Box<dyn Error>> {
            let test_case = TestCase {
                input: LanguageModelInput {
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
                                args: json!({
                                    "location": "Boston",
                                }),
                                id: None,
                            })],
                        }),
                        Message::Tool(ToolMessage {
                            content: vec![Part::ToolResult(ToolResultPart {
                                tool_call_id: "0mbnj08nt".to_string(),
                                tool_name: "get_weather".to_string(),
                                content: vec![Part::Text(TextPart {
                                    text: json!({
                                        "temperature": 70,
                                        "unit": "f",
                                        "description": "Sunny",
                                    })
                                    .to_string(),
                                    id: None,
                                })],
                                is_error: None,
                            })],
                        }),
                    ],
                    tools: Some(vec![get_weather_tool()]),
                    ..Default::default()
                },
                method: TestMethod::Stream,
                output: OutputAssertion {
                    content: vec![PartAssertion::Text(TextPartAssertion {
                        text: Regex::new(r"(?i)70.*sunny|sunny.*70").unwrap(),
                    })],
                },
            };
            run_test_case(&*$model_name, test_case).await
        }

        #[test]
        async fn test_generate_parallel_tool_calls() -> Result<(), Box<dyn Error>> {
            let test_case = TestCase {
                input: LanguageModelInput {
                    messages: vec![Message::User(UserMessage {
                        content: vec![Part::Text(TextPart {
                            text: "Get me the weather in Boston and the stock price of AAPL."
                                .to_string(),
                            id: None,
                        })],
                    })],
                    tools: Some(vec![get_weather_tool(), get_stock_price_tool()]),
                    ..Default::default()
                },
                method: TestMethod::Generate,
                output: OutputAssertion {
                    content: vec![
                        PartAssertion::ToolCall(ToolCallPartAssertion {
                            tool_name: "get_weather".to_string(),
                            args: vec![(
                                "location".to_string(),
                                ToolCallpartAssertionArgPropValue::Value(
                                    Regex::new(r"Boston").unwrap(),
                                ),
                            )],
                        }),
                        PartAssertion::ToolCall(ToolCallPartAssertion {
                            tool_name: "get_stock_price".to_string(),
                            args: vec![(
                                "symbol".to_string(),
                                ToolCallpartAssertionArgPropValue::Value(Regex::new(r"AAPL").unwrap()),
                            )],
                        }),
                    ],
                },
            };
            run_test_case(&*$model_name, test_case).await
        }

        #[test]
        async fn test_stream_parallel_tool_calls() -> Result<(), Box<dyn Error>> {
            let test_case = TestCase {
                input: LanguageModelInput {
                    messages: vec![Message::User(UserMessage {
                        content: vec![Part::Text(TextPart {
                            text: "Get me the weather in Boston and the stock price of AAPL. You must do both of them in one go."
                                .to_string(),
                            id: None,
                        })],
                    })],
                    tools: Some(vec![get_weather_tool(), get_stock_price_tool()]),
                    ..Default::default()
                },
                method: TestMethod::Stream,
                output: OutputAssertion {
                    content: vec![
                        PartAssertion::ToolCall(ToolCallPartAssertion {
                            tool_name: "get_weather".to_string(),
                            args: vec![(
                                "location".to_string(),
                                ToolCallpartAssertionArgPropValue::Value(
                                    Regex::new(r"Boston").unwrap(),
                                ),
                            )],
                        }),
                        PartAssertion::ToolCall(ToolCallPartAssertion {
                            tool_name: "get_stock_price".to_string(),
                            args: vec![(
                                "symbol".to_string(),
                                ToolCallpartAssertionArgPropValue::Value(Regex::new(r"AAPL").unwrap()),
                            )],
                        }),
                    ],
                },
            };
            run_test_case(&*$model_name, test_case).await
        }

        #[test]
        async fn test_stream_parallel_tool_calls_same_name() -> Result<(), Box<dyn Error>> {
            let test_case = TestCase {
                input: LanguageModelInput {
                    messages: vec![Message::User(UserMessage {
                        content: vec![Part::Text(TextPart {
                            text: "Get me the weather in Boston and the weather in New York."
                                .to_string(),
                            id: None,
                        })],
                    })],
                    tools: Some(vec![get_weather_tool()]),
                    ..Default::default()
                },
                method: TestMethod::Stream,
                output: OutputAssertion {
                    content: vec![
                        PartAssertion::ToolCall(ToolCallPartAssertion {
                            tool_name: "get_weather".to_string(),
                            args: vec![(
                                "location".to_string(),
                                ToolCallpartAssertionArgPropValue::Value(
                                    Regex::new(r"Boston").unwrap(),
                                ),
                            )],
                        }),
                        PartAssertion::ToolCall(ToolCallPartAssertion {
                            tool_name: "get_weather".to_string(),
                            args: vec![(
                                "location".to_string(),
                                ToolCallpartAssertionArgPropValue::Value(
                                    Regex::new(r"New York").unwrap(),
                                ),
                            )],
                        }),
                    ],
                },
            };
            run_test_case(&*$model_name, test_case).await
        }

        #[test]
        async fn test_structured_response_format() -> Result<(), Box<dyn Error>> {
            let test_case = TestCase {
                input: LanguageModelInput {
                    messages: vec![Message::User(UserMessage {
                        content: vec![Part::Text(TextPart {
                            text: r#"Create a user with the id "a1b2c3", name "John Doe", email "john.doe@example.com", birthDate "1990-05-15", age 34, isActive true, role "user", accountBalance 500.75, phoneNumber "+1234567890123", tags ["developer", "gamer"], and lastLogin "2024-11-09T10:30:00Z"."#
                                .to_string(),
                            id: None,
                        })],
                    })],
                    response_format: Some(ResponseFormatOption::Json(ResponseFormatJson {
                        name: "user".to_string(),
                        description: None,
                        schema: Some(json!({
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
                            "required": [
                                "id",
                                "name",
                                "email",
                                "birthDate",
                                "age",
                                "isActive",
                                "role",
                                "accountBalance",
                                "phoneNumber",
                                "tags",
                                "lastLogin",
                            ],
                            "additionalProperties": false,
                        })),
                    })),
                    ..Default::default()
                },
                method: TestMethod::Generate,
                output: OutputAssertion {
                    content: vec![
                        PartAssertion::Text(TextPartAssertion {
                            text: Regex::new(r#""id"\s*:\s*"a1b2c3""#).unwrap(),
                        }),
                        PartAssertion::Text(TextPartAssertion {
                            text: Regex::new(r#""name"\s*:\s*"John Doe""#).unwrap(),
                        }),
                        PartAssertion::Text(TextPartAssertion {
                            text: Regex::new(r#""email"\s*:\s*"john\.doe@example\.com""#).unwrap(),
                        }),
                    ],
                },
            };
            run_test_case(&*$model_name, test_case).await
        }
    };
}
