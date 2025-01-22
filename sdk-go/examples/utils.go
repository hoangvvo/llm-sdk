package examples

import (
	"encoding/json"
)

// ToJSONString formats any value as pretty-formatted JSON string
func ToJSONString(v any) string {
	jsonData, _ := json.MarshalIndent(v, "", "  ")
	return string(jsonData)
}
