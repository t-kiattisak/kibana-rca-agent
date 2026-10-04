# Multi-stage build for demo app
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download || true

COPY cmd/app/ ./cmd/app/
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/app ./cmd/app

# Minimal runtime image
FROM alpine:3.21

WORKDIR /app
RUN apk --no-cache add ca-certificates curl

COPY --from=builder /app/bin/app /app/app

EXPOSE 8000

ENTRYPOINT ["/app/app"]
