contain script to deploy the example servers.

```bash
docker buildx build -t hoangvvodev/llm-sdk:latest --push --build-context agent-go=../agent-go --build-context sdk-go=../sdk-go --build-context agent-rust=../agent-rust  --build-context sdk-rust=../sdk-rust --platform linux/amd64 .
```
