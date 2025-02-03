package llmsdk

// [internal] For providers that do not support document parts,
// we translate them to compatible parts such as Text.
// Inner Document parts are flattened.
func GetCompatiblePartsWithoutDocumentParts(parts []Part) []Part {
	var result []Part
	for _, part := range parts {
		if part.DocumentPart != nil {
			result = append(result, GetCompatiblePartsWithoutDocumentParts(part.DocumentPart.Content)...)
		} else {
			result = append(result, part)
		}
	}
	return result
}
