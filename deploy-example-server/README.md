contain script to deploy the example servers.

```bash
docker build --build-context agent-go=../agent-go --build-context sdk-go=../sdk-go --build-context agent-rust=../agent-rust  --build-context sdk-rust=../sdk-rust .
```
