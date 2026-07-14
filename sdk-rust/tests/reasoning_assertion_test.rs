#![allow(dead_code)]

use llm_sdk::{Part, ReasoningPart};

#[path = "common/assert.rs"]
mod assertions;

use assertions::{compile_pattern, ReasoningPartAssertion};

#[test]
fn reasoning_assertion_requires_matching_text_even_with_a_signature() {
    let assertion = ReasoningPartAssertion {
        text: compile_pattern("John"),
        signature: true,
    };
    let content = [Part::Reasoning(
        ReasoningPart::new("wrong").with_signature("opaque"),
    )];

    assert!(assertion.assert(&content).is_err());
}

#[test]
fn reasoning_assertion_can_require_signature_presence() {
    let assertion = ReasoningPartAssertion {
        text: compile_pattern("John"),
        signature: true,
    };

    assert!(assertion
        .assert(&[Part::Reasoning(ReasoningPart::new("John"))])
        .is_err());
    assert!(assertion
        .assert(&[Part::Reasoning(
            ReasoningPart::new("John").with_signature("opaque"),
        )])
        .is_ok());
}
