package partutil

import llmsdk "github.com/hoangvvo/llm-sdk/sdk-go"

// [internal] For providers that do not support source parts,
// we translate them to compatible parts such as Text.
// Inner Source parts are flattened.
func GetCompatiblePartsWithoutSourceParts(parts []llmsdk.Part) []llmsdk.Part {
	var result []llmsdk.Part
	for _, part := range parts {
		if part.SourcePart != nil {
			result = append(result, GetCompatiblePartsWithoutSourceParts(part.SourcePart.Content)...)
		} else {
			result = append(result, part)
		}
	}
	return result
}
