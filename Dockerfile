ARG FIRECORE_VERSION=v1.14.1

FROM golang:1.26-bookworm AS build
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
# SDF only publishes amd64 packages; arm64 images ship without stellar-core
# and require mounting a binary at /usr/bin/stellar-core or overriding
# --stellar-core-bin. The RPC fetcher works on arm64 without stellar-core.
#
# Security: stellar-core 26.1.0-3210.427aa3978 fixes a critical network
# vulnerability (SDF advisory, May 2026). The build pulls from SDF's
# `stable` apt channel which now ships the patched version; rebuilds
# after that publish date pick it up automatically. STELLAR_CORE_MIN_VERSION
# is asserted post-install to fail the build loudly if the apt index is
# pinned/cached to a pre-fix package somehow.
ARG TARGETARCH
ARG STELLAR_CORE_MIN_VERSION=26.1.0
RUN set -eux; \
    if [ "${TARGETARCH}" = "amd64" ]; then \
        apt-get update; \
        apt-get install -y --no-install-recommends ca-certificates curl gnupg dpkg; \
        install -m 0755 -d /etc/apt/keyrings; \
        curl -sSL https://apt.stellar.org/SDF.asc | gpg --dearmor -o /etc/apt/keyrings/SDF.gpg; \
        chmod a+r /etc/apt/keyrings/SDF.gpg; \
        . /etc/os-release; \
        echo "deb [signed-by=/etc/apt/keyrings/SDF.gpg] https://apt.stellar.org ${VERSION_CODENAME} stable" \
            > /etc/apt/sources.list.d/SDF.list; \
        apt-get update; \
        apt-get install -y --no-install-recommends stellar-core; \
        rm -rf /var/lib/apt/lists/*; \
        stellar-core version; \
        INSTALLED=$(dpkg-query -W -f='${Version}' stellar-core); \
        if ! dpkg --compare-versions "${INSTALLED}" ge "${STELLAR_CORE_MIN_VERSION}"; then \
            echo "stellar-core ${INSTALLED} is older than required ${STELLAR_CORE_MIN_VERSION}; refusing to build (see SDF May 2026 advisory)." >&2; \
            exit 1; \
        fi; \
    else \
        echo "Skipping stellar-core install on ${TARGETARCH} (SDF amd64-only). Mount /usr/bin/stellar-core or use --stellar-core-bin."; \
    fi

COPY --from=build "/app/${BINARY_NAME}" "/app/${BINARY_NAME}"

ENTRYPOINT ["/app/firecore"]
