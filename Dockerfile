# syntax=docker/dockerfile:1.4

FROM golang:1.24-bookworm AS build
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . ./

# Build the binary with version information
ARG VERSION="dev"
RUN go build -v -ldflags "-X main.version=${VERSION}" -o firestellar ./cmd/firestellar

FROM ghcr.io/streamingfast/firehose-core:v1.10.1

# Copy the firestellar binary to the firehose-core image
COPY --from=build /app/firestellar /app/firestellar

ENTRYPOINT ["/app/firecore"]