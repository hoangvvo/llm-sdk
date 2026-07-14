package testcommon

import (
	"regexp"
	"testing"

	llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"
	"github.com/hoangvvo/llm-sdk/sdk-go/utils/ptr"
)

func TestReasoningAssertionRequiresTextAndSignature(t *testing.T) {
	assertion := ReasoningPartAssertion{
		Text:      regexp.MustCompile("John"),
		Signature: true,
	}

	if assertion.matches(&llmsdk.ReasoningPart{Text: "wrong", Signature: ptr.To("opaque")}) {
		t.Fatal("a signature must not substitute for matching reasoning text")
	}
	if assertion.matches(&llmsdk.ReasoningPart{Text: "John", Signature: nil}) {
		t.Fatal("the assertion must fail when a required signature is absent")
	}
	if !assertion.matches(&llmsdk.ReasoningPart{Text: "John", Signature: ptr.To("opaque")}) {
		t.Fatal("matching text with a present signature should pass")
	}
}
