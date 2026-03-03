# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /workspace

# Copy go mod files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the unified binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /workspace/secure-inference ./cmd/secure-inference

# Runtime stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/secure-inference .

USER 65532:65532

ENTRYPOINT ["/secure-inference"]
