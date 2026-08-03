use llm_sdk::{LanguageModelPricing, ModelUsage, ModelUsageCostOptions};
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
