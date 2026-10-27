FROM golang:1.26-alpine AS builder

WORKDIR /build

# Install required packages
RUN apk add --no-cache \
    git \
    curl \
    bash \
    sudo

# Copy module + workspace files (o mcp vive no módulo cli/, em modo workspace)
COPY go.work go.mod go.sum ./
COPY cli/go.mod cli/go.sum ./cli/

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the MCP server
RUN go build -o /build/zord ./cli/cmd/zord

# Final stage
FROM golang:1.26-alpine

WORKDIR /app

# Install required packages
RUN apk add --no-cache \
    git \
    curl \
    bash \
    sudo

# Set up Atlas
ENV ATLAS_VERSION=v0.29.0
RUN curl -sSf https://atlasgo.sh | sh

# Copy the binary from builder
COPY --from=builder /build/zord /usr/local/bin/zord

# Create data directory for mounted volumes
RUN mkdir -p /app/data

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/zord", "mcp"] 