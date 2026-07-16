package mcp

import "testing"

func TestEnsureBearerPrefix(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"raw token":          "token",
		"prefixed token":     "Bearer token",
		"lowercase prefix":   "bearer token",
		"surrounding spaces": "  BEARER   token  ",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if actual := ensureBearerPrefix(input); actual != "Bearer token" {
				t.Fatalf("ensureBearerPrefix() = %q, want %q", actual, "Bearer token")
			}
		})
	}
}
