use std::error::Error;

use crate::{LanguageModel, LanguageModelInput, Part, StreamAccumulator};
use futures::stream::StreamExt;
use regex::Regex;

#[derive(Debug, Clone)]
pub struct TextPartAssertion {
    pub text: Regex,
}

impl TextPartAssertion {
    pub fn assert(&self, content: &[Part]) -> Result<(), String> {
        let found_part = content.iter().find(|part| {
            if let Part::Text(text_part) = part {
                self.text.is_match(&text_part.text)
            } else {
                false
            }
        });

        if found_part.is_none() {
            return Err(format!(
                "Expected matching text part: {}\nReceived:\n{}",
                self.text,
                serde_json::to_string_pretty(content).unwrap()
            ));
        }

        Ok(())
    }
}

#[derive(Debug, Clone)]
pub enum ToolCallpartAssertionArgPropValue {
    Value(Regex),
    // Object(Vec<(String, ToolCallpartAssertionArgPropValue)>),
}

impl ToolCallpartAssertionArgPropValue {
    pub fn is_matched(&self, actual: &serde_json::Value) -> bool {
        match self {
            Self::Value(regex) => {
                let actual_str = actual.to_string();
                regex.is_match(&actual_str)
            } // Self::Object(expected_props) => {
              //     if let Some(actual_obj) = actual.as_object() {
              //         for (key, expected_value) in expected_props {
              //             if let Some(actual_value) = actual_obj.get(key) {
              //                 if !expected_value.is_matched(actual_value) {
              //                     return false;
              //                 }
              //             } else {
              //                 return false;
              //             }
              //         }
              //         true
              //     } else {
              //         false
              //     }
              // }
        }
    }
}

#[derive(Debug, Clone)]
pub struct ToolCallPartAssertion {
    pub tool_name: String,
    pub args: Vec<(String, ToolCallpartAssertionArgPropValue)>,
}

impl ToolCallPartAssertion {
    pub fn assert(&self, content: &[Part]) -> Result<(), String> {
        let found_part = content.iter().find(|part| {
            if let Part::ToolCall(tool_call_part) = part {
                tool_call_part.tool_name == self.tool_name
                    && self.args.iter().all(|(key, value)| {
                        tool_call_part
                            .args
                            .get(key)
                            .is_some_and(|arg| value.is_matched(arg))
                    })
            } else {
                false
            }
        });

        if found_part.is_none() {
            return Err(format!(
                "Expected matching tool call part:\nExpected tool {} with args {:?}\nReceived:\n{}",
                self.tool_name,
                self.args,
                serde_json::to_string_pretty(content).unwrap()
            ));
        }

        Ok(())
    }
}

#[derive(Debug, Clone)]
pub enum PartAssertion {
    Text(TextPartAssertion),
    ToolCall(ToolCallPartAssertion),
}

impl PartAssertion {
    pub fn assert(&self, content: &[Part]) -> Result<(), String> {
        match self {
            Self::Text(text_assertion) => text_assertion.assert(content),
            Self::ToolCall(tool_call_assertion) => tool_call_assertion.assert(content),
        }
    }
}

#[derive(Debug, Clone)]
pub enum TestMethod {
    Generate,
    Stream,
}

#[derive(Debug, Clone)]
pub struct OutputAssertion {
    pub content: Vec<PartAssertion>,
}

#[derive(Debug, Clone)]
pub struct TestCase {
    pub input: LanguageModelInput,
    pub method: TestMethod,
    pub output: OutputAssertion,
}

pub async fn run_test_case(
    model: &dyn LanguageModel,
    test_case: TestCase,
) -> Result<(), Box<dyn Error>> {
    match test_case.method {
        TestMethod::Generate => {
            let result = model.generate(test_case.input).await?;
            for part_assertion in test_case.output.content {
                part_assertion.assert(&result.content)?;
            }
        }
        TestMethod::Stream => {
            let mut stream = model.stream(test_case.input).await?;

            let mut accumulator = StreamAccumulator::new();

            while let Some(partial_response) = stream.next().await {
                let partial_response = partial_response?;
                accumulator.add_partial(&partial_response)?;
            }

            let result = accumulator.compute_response()?;
            for part_assertion in test_case.output.content {
                part_assertion.assert(&result.content)?;
            }
        }
    }
    Ok(())
}
