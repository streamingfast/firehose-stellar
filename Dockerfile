# syntax=docker/dockerfile:1.2

FROM ghcr.io/streamingfast/firehose-core:v1.6.9 as core

FROM ubuntu:24.04

RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    apt-get -y install -y \
    ca-certificates vim htop iotop sysstat \
    dstat strace lsof curl jq tzdata && \
    rm -rf /var/cache/apt /var/lib/apt/lists/*

RUN rm /etc/localtime && ln -snf /usr/share/zoneinfo/America/Montreal /etc/localtime && dpkg-reconfigure -f noninteractive tzdata

ADD /firestellar /app/firestellar

ENV PATH "$PATH:/app"

COPY --from=core /app/firecore /app/firecore

ENTRYPOINT ["/app/firestellar"]
