# Build Stage
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# We accept the target architecture arguments from the build system
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o imap-backup .

# Final Stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for SSL/TLS connections
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/imap-backup .

# Create output directory
RUN mkdir -p output

# Command to run
ENTRYPOINT ["./imap-backup"]
