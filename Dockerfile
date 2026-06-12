# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.23
ARG DEBIAN_VERSION=bookworm
ARG NSJAIL_VERSION=3.4

# ---- Build nsjail from source ----
FROM debian:${DEBIAN_VERSION}-slim AS nsjail-builder
ARG NSJAIL_VERSION
RUN apt-get update && apt-get install -y --no-install-recommends \
        autoconf bison ca-certificates flex g++ gcc git libnl-route-3-dev \
        libprotobuf-dev libtool make pkg-config protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*
RUN git clone --depth 1 --branch ${NSJAIL_VERSION} https://github.com/google/nsjail.git /src/nsjail \
    && make -C /src/nsjail \
    && install -m 0755 /src/nsjail/nsjail /usr/local/bin/nsjail

# ---- Builder / dev image (Go + linters + nsjail) ----
FROM golang:${GO_VERSION}-${DEBIAN_VERSION} AS builder
RUN apt-get update && apt-get install -y --no-install-recommends \
        libnl-route-3-200 libprotobuf32 python3 \
        default-jdk nodejs iverilog uidmap \
    && rm -rf /var/lib/apt/lists/* \
    && echo "root:100000:1000000000" > /etc/subuid \
    && echo "root:100000:1000000000" > /etc/subgid
COPY --from=nsjail-builder /usr/local/bin/nsjail /usr/local/bin/nsjail
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/goboxd ./cmd/goboxd

# ---- Runtime image ----
FROM debian:${DEBIAN_VERSION}-slim AS runtime
ARG KOTLIN_VERSION=2.1.10
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates libnl-route-3-200 libprotobuf32 \
        python3 gcc libc6-dev php sbcl \
        g++ default-jdk nodejs iverilog uidmap \
        curl unzip \
    # Install the Kotlin command-line compiler (depends on the JDK above)
    && curl -fsSL -o /tmp/kotlin.zip \
        "https://github.com/JetBrains/kotlin/releases/download/v${KOTLIN_VERSION}/kotlin-compiler-${KOTLIN_VERSION}.zip" \
    && unzip -q /tmp/kotlin.zip -d /opt \
    && ln -sf /opt/kotlinc/bin/kotlinc /usr/local/bin/kotlinc \
    && ln -sf /opt/kotlinc/bin/kotlin  /usr/local/bin/kotlin \
    && rm -f /tmp/kotlin.zip \
    && apt-get purge -y --auto-remove unzip \
    && rm -rf /var/lib/apt/lists/* \
    && echo "root:100000:1000000000" > /etc/subuid \
    && echo "root:100000:1000000000" > /etc/subgid
COPY --from=nsjail-builder /usr/local/bin/nsjail /usr/local/bin/nsjail
COPY --from=builder        /out/goboxd          /usr/local/bin/goboxd
COPY configs/ /configs/
ENV LANGUAGE_CONFIG=/configs/languages/languages.yaml
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/goboxd"]
