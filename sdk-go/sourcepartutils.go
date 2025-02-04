package llmsdk

// [internal] For providers that do not support source parts,
// we translate them to compatible parts such as Text.
// Inner Source parts are flattened.
func GetCompatiblePartsWithoutSourceParts(parts []Part) []Part {
	var result []Part
	for _, part := range parts {
		if part.SourcePart != nil {
			result = append(result, GetCompatiblePartsWithoutSourceParts(part.SourcePart.Content)...)
		} else {
			result = append(result, part)
		}
	}
	return result
}
