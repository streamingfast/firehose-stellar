ARG FIRECORE_VERSION=v1.14.1
# Fed from go.mod by the workflow (streamingfast/actions/go-version) so this tag
# cannot drift below the `go` directive and fail late, after the pull.
ARG GO_VERSION=1.26

# Pinned to the builder's architecture so the compiler always runs natively and
# cross compiles instead of being emulated. GOOS/GOARCH default to the platform
# Buildx is producing, which is what the runtime image needs, and are overridden
# to reach a platform Docker has no notion of (darwin).
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG VERSION="edge"
ARG BINARY_NAME=firestellar
ARG TARGETOS
ARG TARGETARCH
ARG GOOS
ARG GOARCH

# CGO off keeps one binary shape across every target: the darwin and arm64
# artifacts have no cross toolchain available here, and the released archives
# are expected to run on hosts other than the bookworm builder.
# `-s -w` matches what goreleaser applied to every published archive before this
# build produced them; without it the release assets carry DWARF and grow ~40%.
RUN CGO_ENABLED=0 GOOS="${GOOS:-$TARGETOS}" GOARCH="${GOARCH:-$TARGETARCH}" \
    go build -v -ldflags "-s -w -X main.version=${VERSION}" -o "${BINARY_NAME}" "./cmd/${BINARY_NAME}"

# Extracted by the release workflow with `--target binary --output type=local`,
# which writes the binary alone to the destination directory. Kept ahead of the
# runtime stage so building it never reaches the stellar-core install below.
FROM scratch AS binary

ARG BINARY_NAME=firestellar

COPY --from=build "/app/${BINARY_NAME}" "/${BINARY_NAME}"

FROM ghcr.io/streamingfast/firehose-core:${FIRECORE_VERSION}

ARG BINARY_NAME=firestellar

# Install stellar-core from SDF apt repo so the captive-core fetcher works
# standalone (default --stellar-core-bin is /usr/bin/stellar-core).
# SDF only publishes amd64 packages; arm64 images ship without stellar-core
# and require mounting a binary at /usr/bin/stellar-core or overriding
# --stellar-core-bin. The RPC fetcher works on arm64 without stellar-core.
#
# Protocol 28 requires stellar-core 28.0.x: an older captive-core halts at the
# P28 upgrade ledger (testnet vote 2026-08-27, mainnet vote 2026-09-16) instead
# of following the network. 28.0.1 is SDF's fix for the August 2026 critical
# security advisory and is the floor every node on the network is asked to run.
# The build pulls from SDF's `stable` apt channel, which ships that release;
# rebuilds pick it up automatically.
# STELLAR_CORE_MIN_VERSION is asserted post-install to fail the build loudly
# if the apt index is pinned/cached to a package below the floor somehow.
# The bound is intentionally codename-agnostic (no `.noble`/`.jammy` suffix)
# so the dpkg comparison holds whatever Ubuntu base firehose-core ships.
ARG TARGETARCH
ARG STELLAR_CORE_MIN_VERSION=28.0.1-3508.947aad841
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
            echo "stellar-core ${INSTALLED} is older than required ${STELLAR_CORE_MIN_VERSION}; refusing to build (the floor covers both Protocol 28 support and SDF's August 2026 security fix)." >&2; \
            exit 1; \
        fi; \
    else \
        echo "Skipping stellar-core install on ${TARGETARCH} (SDF amd64-only). Mount /usr/bin/stellar-core or use --stellar-core-bin."; \
    fi

COPY --from=build "/app/${BINARY_NAME}" "/app/${BINARY_NAME}"

ENTRYPOINT ["/app/firecore"]
