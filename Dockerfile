ARG FIRECORE_VERSION=v1.14.1

FROM golang:1.25-bookworm AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG VERSION="edge"
ARG BINARY_NAME=firestellar

RUN go build -v -ldflags "-X main.version=${VERSION}" -o "${BINARY_NAME}" "./cmd/${BINARY_NAME}"

FROM ghcr.io/streamingfast/firehose-core:${FIRECORE_VERSION}

ARG BINARY_NAME=firestellar

# Install stellar-core from SDF apt repo so the captive-core fetcher works
# standalone (default --stellar-core-bin is /usr/bin/stellar-core).
# Note: SDF only publishes amd64 packages; arm64 builds must mount a
# stellar-core binary at /usr/bin/stellar-core or override --stellar-core-bin.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates curl gnupg; \
    install -m 0755 -d /etc/apt/keyrings; \
    curl -sSL https://apt.stellar.org/SDF.asc | gpg --dearmor -o /etc/apt/keyrings/SDF.gpg; \
    chmod a+r /etc/apt/keyrings/SDF.gpg; \
    . /etc/os-release; \
    echo "deb [signed-by=/etc/apt/keyrings/SDF.gpg] https://apt.stellar.org ${VERSION_CODENAME} stable" \
        > /etc/apt/sources.list.d/SDF.list; \
    apt-get update; \
    apt-get install -y --no-install-recommends stellar-core; \
    rm -rf /var/lib/apt/lists/*; \
    stellar-core version

COPY --from=build "/app/${BINARY_NAME}" "/app/${BINARY_NAME}"

ENTRYPOINT ["/app/firecore"]
