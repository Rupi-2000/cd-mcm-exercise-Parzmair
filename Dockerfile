# Build stage
FROM golang:1.26.3-alpine AS builder

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /api-server ./cmd/api

# Runtime stage
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /api-server /api-server

EXPOSE 8080

USER 65532:65532

ENTRYPOINT ["/api-server"]
