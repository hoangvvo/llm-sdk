use llm_sdk::{LanguageModelPricing, ModelTokensDetails, ModelUsage, ModelUsageCostOptions};

fn assert_cost_eq(actual: f64, expected: f64) {
    assert!(
        (actual - expected).abs() < f64::EPSILON,
        "expected cost {expected}, got {actual}"
    );
}

#[test]
fn calculates_included_and_additional_cache_tokens_without_double_counting() {
    let pricing = LanguageModelPricing {
        input_cost_per_text_token: Some(2.0),
        input_cost_per_cached_token: Some(0.5),
        input_cost_per_cache_write_token: Some(2.5),
        input_cost_per_cached_text_token: None,
        output_cost_per_text_token: Some(3.0),
        input_cost_per_audio_token: None,
        input_cost_per_cached_audio_token: None,
        output_cost_per_audio_token: None,
        input_cost_per_image_token: None,
        input_cost_per_cached_image_token: None,
        output_cost_per_image_token: None,
    };
    let included = ModelUsage {
        input_tokens: 100,
        output_tokens: 10,
        input_tokens_details: Some(ModelTokensDetails {
            cached_tokens: Some(40),
            cache_write_tokens: Some(20),
            ..Default::default()
        }),
        output_tokens_details: None,
    };
    let additional = ModelUsage {
        input_tokens: 40,
        ..included.clone()
    };

    assert_cost_eq(
        included.calculate_cost(
            &pricing,
            &ModelUsageCostOptions {
                input_cache_tokens_are_additional: false,
                output_reasoning_tokens_are_additional: false,
            },
        ),
        180.0,
    );
    assert_cost_eq(
        additional.calculate_cost(
            &pricing,
            &ModelUsageCostOptions {
                input_cache_tokens_are_additional: true,
                output_reasoning_tokens_are_additional: false,
            },
        ),
        180.0,
    );
}

#[test]
fn uses_modality_cache_breakdown_instead_of_aggregate_duplicate() {
    let usage = ModelUsage {
        input_tokens: 100,
        output_tokens: 0,
        input_tokens_details: Some(ModelTokensDetails {
            text_tokens: Some(100),
            cached_text_tokens: Some(80),
            cached_tokens: Some(80),
            ..Default::default()
        }),
        output_tokens_details: None,
    };
    let pricing = LanguageModelPricing {
        input_cost_per_text_token: Some(2.0),
        input_cost_per_cached_token: Some(0.1),
        input_cost_per_cache_write_token: None,
        input_cost_per_cached_text_token: Some(0.5),
        output_cost_per_text_token: None,
        input_cost_per_audio_token: None,
        input_cost_per_cached_audio_token: None,
        output_cost_per_audio_token: None,
        input_cost_per_image_token: None,
        input_cost_per_cached_image_token: None,
        output_cost_per_image_token: None,
    };
    assert_cost_eq(
        usage.calculate_cost(
            &pricing,
            &ModelUsageCostOptions {
                input_cache_tokens_are_additional: false,
                output_reasoning_tokens_are_additional: false,
            },
        ),
        80.0,
    );
}

#[test]
fn handles_additional_reasoning_and_missing_cache_rate() {
    let reasoning_usage = ModelUsage {
        input_tokens: 0,
        output_tokens: 10,
        input_tokens_details: None,
        output_tokens_details: Some(ModelTokensDetails {
            reasoning_tokens: Some(5),
            ..Default::default()
        }),
    };
    let output_pricing = LanguageModelPricing {
        input_cost_per_text_token: None,
        input_cost_per_cached_token: None,
        input_cost_per_cache_write_token: None,
        input_cost_per_cached_text_token: None,
        output_cost_per_text_token: Some(3.0),
        input_cost_per_audio_token: None,
        input_cost_per_cached_audio_token: None,
        output_cost_per_audio_token: None,
        input_cost_per_image_token: None,
        input_cost_per_cached_image_token: None,
        output_cost_per_image_token: None,
    };
    assert_cost_eq(
        reasoning_usage.calculate_cost(
            &output_pricing,
            &ModelUsageCostOptions {
                input_cache_tokens_are_additional: false,
                output_reasoning_tokens_are_additional: false,
            },
        ),
        30.0,
    );
    assert_cost_eq(
        reasoning_usage.calculate_cost(
            &output_pricing,
            &ModelUsageCostOptions {
                input_cache_tokens_are_additional: false,
                output_reasoning_tokens_are_additional: true,
            },
        ),
        45.0,
    );

    let write_usage = ModelUsage {
        input_tokens: 40,
        output_tokens: 0,
        input_tokens_details: Some(ModelTokensDetails {
            cache_write_tokens: Some(20),
            ..Default::default()
        }),
        output_tokens_details: None,
    };
    let input_pricing = LanguageModelPricing {
        input_cost_per_text_token: Some(2.0),
        input_cost_per_cached_token: None,
        input_cost_per_cache_write_token: None,
        input_cost_per_cached_text_token: None,
        output_cost_per_text_token: None,
        input_cost_per_audio_token: None,
        input_cost_per_cached_audio_token: None,
        output_cost_per_audio_token: None,
        input_cost_per_image_token: None,
        input_cost_per_cached_image_token: None,
        output_cost_per_image_token: None,
    };
    assert_cost_eq(
        write_usage.calculate_cost(
            &input_pricing,
            &ModelUsageCostOptions {
                input_cache_tokens_are_additional: true,
                output_reasoning_tokens_are_additional: false,
            },
        ),
        80.0,
    );
}

#[test]
fn adjusts_reported_modality_tokens_from_the_aggregate_rate() {
    let usage = ModelUsage {
        input_tokens: 100,
        output_tokens: 0,
        input_tokens_details: Some(ModelTokensDetails {
            audio_tokens: Some(20),
            ..Default::default()
        }),
        output_tokens_details: None,
    };
    let pricing = LanguageModelPricing {
        input_cost_per_text_token: Some(2.0),
        input_cost_per_cached_token: None,
        input_cost_per_cache_write_token: None,
        input_cost_per_cached_text_token: None,
        output_cost_per_text_token: None,
        input_cost_per_audio_token: Some(3.0),
        input_cost_per_cached_audio_token: None,
        output_cost_per_audio_token: None,
        input_cost_per_image_token: None,
        input_cost_per_cached_image_token: None,
        output_cost_per_image_token: None,
    };
    assert_cost_eq(
        usage.calculate_cost(
            &pricing,
            &ModelUsageCostOptions {
                input_cache_tokens_are_additional: false,
                output_reasoning_tokens_are_additional: false,
            },
        ),
        220.0,
    );
}

#[test]
fn ignores_zero_only_modality_details() {
    let usage = ModelUsage {
        input_tokens: 100,
        output_tokens: 10,
        input_tokens_details: Some(ModelTokensDetails {
            audio_tokens: Some(0),
            ..Default::default()
        }),
        output_tokens_details: Some(ModelTokensDetails {
            audio_tokens: Some(0),
            ..Default::default()
        }),
    };
    let pricing = LanguageModelPricing {
        input_cost_per_text_token: Some(2.0),
        input_cost_per_cached_token: None,
        input_cost_per_cache_write_token: None,
        input_cost_per_cached_text_token: None,
        output_cost_per_text_token: Some(4.0),
        input_cost_per_audio_token: Some(3.0),
        input_cost_per_cached_audio_token: None,
        output_cost_per_audio_token: Some(5.0),
        input_cost_per_image_token: None,
        input_cost_per_cached_image_token: None,
        output_cost_per_image_token: None,
    };
    assert_cost_eq(
        usage.calculate_cost(
            &pricing,
            &ModelUsageCostOptions {
                input_cache_tokens_are_additional: false,
                output_reasoning_tokens_are_additional: false,
            },
        ),
        240.0,
    );
}
