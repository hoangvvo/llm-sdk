use futures::StreamExt;
use llm_sdk::{
    llm_sdk_test::{MockGenerateResult, MockLanguageModel, MockStreamResult},
    ContentDelta, LanguageModel, LanguageModelError, LanguageModelInput, LanguageModelResult,
    LanguageModelStream, Message, ModelResponse, Part, PartDelta, PartialModelResponse,
    TextPartDelta, UserMessage,
};

fn user_input(text: &str) -> LanguageModelInput {
    LanguageModelInput {
        messages: vec![Message::User(UserMessage {
            content: vec![Part::text(text)],
        })],
        ..LanguageModelInput::default()
    }
}

fn text_partial(text: &str) -> PartialModelResponse {
    PartialModelResponse {
        delta: Some(ContentDelta {
            index: 0,
            part: PartDelta::Text(TextPartDelta {
                text: text.to_string(),
            }),
        }),
        ..PartialModelResponse::default()
    }
}

#[tokio::test]
async fn mock_language_model_tracks_generate_inputs_and_returns_results() {
    let model = MockLanguageModel::new();

    let response1 = ModelResponse {
        content: vec![Part::text("Hello, world!")],
        ..ModelResponse::default()
    };
    let response3 = ModelResponse {
        content: vec![Part::text("Goodbye, world!")],
        ..ModelResponse::default()
    };

    model
        .enqueue_generate(response1.clone())
        .enqueue_generate(MockGenerateResult::error(LanguageModelError::InvalidInput(
            "generate error".to_string(),
        )))
        .enqueue_generate(response3.clone());

    let input1 = user_input("Hi");
    let res1 = model
        .generate(input1.clone())
        .await
        .expect("first generate should succeed");
    assert_eq!(res1, response1);
    let tracked = model.tracked_generate_inputs();
    assert_eq!(tracked.len(), 1);
    assert_eq!(tracked[0].messages, input1.messages.clone());

    let input2 = user_input("Error");
    let err = model
        .generate(input2.clone())
        .await
        .expect_err("second generate should error");
    match err {
        LanguageModelError::InvalidInput(msg) => {
            assert_eq!(msg, "generate error");
        }
        other => panic!("unexpected error variant: {:?}", other),
    }
    let tracked = model.tracked_generate_inputs();
    assert_eq!(tracked.len(), 2);
    assert_eq!(tracked[1].messages, input2.messages.clone());

    let input3 = user_input("Goodbye");
    let res3 = model
        .generate(input3.clone())
        .await
        .expect("third generate should succeed");
    assert_eq!(res3, response3);
    let tracked = model.tracked_generate_inputs();
    assert_eq!(tracked.len(), 3);
    assert_eq!(tracked[2].messages, input3.messages.clone());

    model.reset();
    assert!(model.tracked_generate_inputs().is_empty());

    model.enqueue_generate(ModelResponse {
        content: vec![Part::text("After reset")],
        ..ModelResponse::default()
    });

    model.restore();
    assert!(model.tracked_generate_inputs().is_empty());

    let err = model
        .generate(input1.clone())
        .await
        .expect_err("generate after restore should fail");
    match err {
        LanguageModelError::Invariant(provider, message) => {
            assert_eq!(provider, "mock");
            assert_eq!(message, "no mocked generate results available");
        }
        other => panic!("unexpected error variant: {:?}", other),
    }
}

#[tokio::test]
async fn mock_language_model_tracks_stream_inputs_and_yields_partials() {
    let model = MockLanguageModel::new();

    let partials1 = vec![
        text_partial("Hello"),
        text_partial(", "),
        text_partial("world!"),
    ];
    let partials3 = vec![
        text_partial("Goodbye"),
        text_partial(", "),
        text_partial("world!"),
    ];

    model
        .enqueue_stream(partials1.clone())
        .enqueue_stream(MockStreamResult::error(LanguageModelError::InvalidInput(
            "stream error".to_string(),
        )))
        .enqueue_stream(partials3.clone());

    let stream_input1 = user_input("Hi");
    let stream1 = model
        .stream(stream_input1.clone())
        .await
        .expect("first stream should succeed");
    let collected1 = collect_stream_partials(stream1)
        .await
        .expect("collecting partials should succeed");
    assert_eq!(collected1, partials1);
    let tracked = model.tracked_stream_inputs();
    assert_eq!(tracked.len(), 1);
    assert_eq!(tracked[0].messages, stream_input1.messages.clone());

    let stream_input2 = user_input("Error");
    let err = match model.stream(stream_input2.clone()).await {
        Ok(_) => panic!("expected stream error"),
        Err(err) => err,
    };
    match err {
        LanguageModelError::InvalidInput(msg) => assert_eq!(msg, "stream error"),
        other => panic!("unexpected error variant: {:?}", other),
    }
    let tracked = model.tracked_stream_inputs();
    assert_eq!(tracked.len(), 2);
    assert_eq!(tracked[1].messages, stream_input2.messages.clone());

    let stream_input3 = user_input("Goodbye");
    let stream3 = model
        .stream(stream_input3.clone())
        .await
        .expect("third stream should succeed");
    let collected3 = collect_stream_partials(stream3)
        .await
        .expect("collecting partials should succeed");
    assert_eq!(collected3, partials3);
    let tracked = model.tracked_stream_inputs();
    assert_eq!(tracked.len(), 3);
    assert_eq!(tracked[2].messages, stream_input3.messages.clone());

    model.reset();
    assert!(model.tracked_stream_inputs().is_empty());

    model.enqueue_stream(vec![text_partial("After reset")]);

    model.restore();
    assert!(model.tracked_stream_inputs().is_empty());

    let err = match model.stream(stream_input1.clone()).await {
        Ok(_) => panic!("expected stream failure"),
        Err(err) => err,
    };
    match err {
        LanguageModelError::Invariant(provider, message) => {
            assert_eq!(provider, "mock");
            assert_eq!(message, "no mocked stream results available");
        }
        other => panic!("unexpected error variant: {:?}", other),
    }
}

async fn collect_stream_partials(
    mut stream: LanguageModelStream,
) -> LanguageModelResult<Vec<PartialModelResponse>> {
    let mut partials = Vec::new();
    while let Some(item) = stream.next().await {
        partials.push(item?);
    }
    Ok(partials)
}
