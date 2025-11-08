FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o corsproxy

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/corsproxy /app/corsproxy
EXPOSE 8080
ENTRYPOINT ["/app/corsproxy"]
