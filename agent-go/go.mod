module github.com/hoangvvo/llm-sdk/agent-go

go 1.23.0

toolchain go1.24.5

replace github.com/hoangvvo/llm-sdk/sdk-go => ../sdk-go

require (
	github.com/hoangvvo/llm-sdk/sdk-go v0.0.0-00010101000000-000000000000
	github.com/joho/godotenv v1.5.1
	github.com/sergi/go-diff v1.3.1
)

require (
	github.com/google/go-cmp v0.7.0
	github.com/sanity-io/litter v1.5.8
	go.opentelemetry.io/otel v1.38.0
	go.opentelemetry.io/otel/trace v1.38.0
	golang.org/x/sync v0.16.0
)

require (
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
)
