# Build Stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies (if needed, though go mod usually handles it)
# Alpine sometimes needs libc-dev/gcc for cgo, but we are disabling cgo.
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 creates a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -o imap-backup .

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
