FROM rust:1.90-bookworm AS rust-builder
WORKDIR /usr/src/app
COPY --from=sdk-rust . ./sdk-rust
COPY --from=agent-rust . ./agent-rust
RUN cargo install --path ./agent-rust --example server

FROM golang:1.25-bookworm AS go-builder
WORKDIR /usr/src/app
COPY --from=agent-go ./go.mod ./go.sum ./
RUN go mod download
COPY --from=agent-go . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o ./server ./examples/server

FROM debian:bookworm-slim
COPY --from=rust-builder /usr/local/cargo/bin/server /usr/local/bin/rustserver
COPY --from=go-builder /usr/src/app/server /usr/local/bin/goserver