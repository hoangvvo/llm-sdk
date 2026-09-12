use llm_sdk::{LanguageModelPricing, ModelTokensDetails, ModelUsage, ModelUsageCostOptions};
use serde::Deserialize;

#[derive(Deserialize)]
struct UsageCostSuite {
    test_cases: Vec<UsageCostCase>,
}

#[derive(Deserialize)]
struct UsageCostCase {
    name: String,
    usage: ModelUsage,
    pricing: LanguageModelPricing,
    options: UsageCostCaseOptions,
    expected_cost: f64,
}

#[derive(Deserialize)]
struct UsageCostCaseOptions {
    input_cache_tokens_are_additional: bool,
    output_reasoning_tokens_are_additional: bool,
}

#[test]
fn shared_usage_cost_cases() {
    let suite: UsageCostSuite = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../sdk-tests/usage-costs.json"
    )))
    .expect("shared usage cost cases must be valid JSON");

    for test_case in suite.test_cases {
        let actual = test_case.usage.calculate_cost(
            &test_case.pricing,
            &ModelUsageCostOptions {
                input_cache_tokens_are_additional: test_case
                    .options
                    .input_cache_tokens_are_additional,
                output_reasoning_tokens_are_additional: test_case
                    .options
                    .output_reasoning_tokens_are_additional,
            },
        );
        let tolerance = (test_case.expected_cost.abs() * 1e-12).max(1e-12);
        assert!(
            (actual - test_case.expected_cost).abs() <= tolerance,
            "{}: expected cost {}, received {actual}",
            test_case.name,
            test_case.expected_cost
        );
    }
}

#[test]
fn merges_cumulative_partial_usage_without_erasing_known_counts() {
    let mut usage = ModelUsage {
        input_tokens: 10,
        output_tokens: 0,
        input_tokens_details: Some(ModelTokensDetails {
            cached_tokens: Some(2),
            ..Default::default()
        }),
        ..Default::default()
    };
    usage.merge_max(&ModelUsage {
        input_tokens: 0,
        output_tokens: 5,
        output_tokens_details: Some(ModelTokensDetails {
            reasoning_tokens: Some(1),
            ..Default::default()
        }),
        ..Default::default()
    });
    assert_eq!(
        usage,
        ModelUsage {
            input_tokens: 10,
            output_tokens: 5,
            input_tokens_details: Some(ModelTokensDetails {
                cached_tokens: Some(2),
                ..Default::default()
            }),
            output_tokens_details: Some(ModelTokensDetails {
                reasoning_tokens: Some(1),
                ..Default::default()
            }),
            server_tool_use: None,
        }
    );
}
