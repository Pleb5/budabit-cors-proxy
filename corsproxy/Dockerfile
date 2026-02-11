FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY corsproxy ./corsproxy
RUN go mod download
RUN mkdir -p /app/bin
RUN go build -o /app/bin/corsproxy ./corsproxy

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/bin/corsproxy /app/corsproxy
EXPOSE 8080
ENTRYPOINT ["/app/corsproxy"]
