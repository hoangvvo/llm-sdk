FROM rust:1.88 AS rust-builder
WORKDIR /usr/src/app
COPY --from=agent-rust . .
RUN cargo install --path . --example server

# FROM golang:1.24 AS go-builder
# WORKDIR /usr/src/app
# COPY --from=agent-go ./go.mod ./go.sum ./
# RUN go mod download
# COPY --from=agent-go ./*.go ./
# RUN CGO_ENABLED=0 GOOS=linux go build -o ./server ./examples/server

FROM debian:bullseye-slim
COPY --from=rust-builder /usr/local/cargo/bin/server /usr/local/bin/rustserver
# COPY --from=go-builder /usr/src/app/server /usr/local/bin/goserver