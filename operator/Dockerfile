# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /workspace

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the manager binary (no codegen required in image build)
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/manager cmd/manager/main.go

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /

COPY --from=builder /workspace/bin/manager /manager

# Run as non-root
USER 65532:65532

ENTRYPOINT ["/manager"]
