# dbn-duckduck-goose Dockerfile
# Copyright (c) 2025 Neomantra Corp

ARG GOOSE_BUILD_BASE="golang"
ARG GOOSE_BUILD_TAG="1.24-bullseye"

# ARG GOOSE_RUNTIME_BASE="debian"
# ARG GOOSE_RUNTIME_TAG="bullseye-slim"

ARG GOOSE_RUNTIME_BASE="golang"
ARG GOOSE_RUNTIME_TAG="1.24-bullseye"

##################################################################################################
# Builder
##################################################################################################

FROM ${GOOSE_BUILD_BASE}:${GOOSE_BUILD_TAG} AS build

ARG GOOSE_BUILD_BASE="golang"
ARG GOOSE_BUILD_TAG="1.24-bullseye"

# Extract TARGETARCH from BuildKit
ARG TARGETARCH

RUN DEBIAN_FRONTEND=noninteractive apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    git

ARG TASKFILE_VERSION="v3.42.1"
RUN curl -fSL "https://github.com/go-task/task/releases/download/${TASKFILE_VERSION}/task_linux_${TARGETARCH}.deb" -o /tmp/task_linux.deb \
    && dpkg -i /tmp/task_linux.deb \
    && rm /tmp/task_linux.deb

ARG SWAGGO_VERSION="v1.16.4"
RUN go install "github.com/swaggo/swag/cmd/swag@${SWAGGO_VERSION}"

ADD . /src
WORKDIR /src

# Regular build for smoke-test
RUN mkdir -p bin && task build
RUN uname -a 
# Labels
LABEL GOOSE_BUILD_BASE="${GOOSE_BUILD_BASE}"
LABEL GOOSE_BUILD_TAG="${GOOSE_BUILD_TAG}"

LABEL GOOSE_TARGET_ARCH="${TARGETARCH}"
LABEL SWAGGO_VERSION="${SWAGGO_VERSION}"


##################################################################################################
# Runtime environment
###########################q#######################################################################

FROM ${GOOSE_RUNTIME_BASE}:${GOOSE_RUNTIME_TAG} AS runtime

ARG GOOSE_BUILD_BASE="golang"
ARG GOOSE_BUILD_TAG="1.24-bullseye"

# ARG GOOSE_RUNTIME_BASE="debian"
# ARG GOOSE_RUNTIME_TAG="bullseye-slim"

ARG GOOSE_RUNTIME_BASE="golang"
ARG GOOSE_RUNTIME_TAG="1.24-bullseye"

# Extract TARGETARCH from BuildKit
ARG TARGETARCH

# Install dependencies and ops tools
RUN DEBIAN_FRONTEND=noninteractive apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        ca-certificates \
        coreutils \
        curl \
        zstd \
    && rm -rf /var/lib/apt/lists/*

# Copy binaries
COPY --from=build /src/bin/* /usr/local/bin/

# Labels
LABEL GOOSE_BUILD_BASE="${GOOSE_BUILD_BASE}"
LABEL GOOSE_BUILD_TAG="${GOOSE_BUILD_TAG}"
LABEL GOOSE_RUNTIME_BASE="${GOOSE_RUNTIME_BASE}"
LABEL GOOSE_RUNTIME_TAG="${GOOSE_RUNTIME_TAG}"
LABEL GOOSE_TARGET_ARCH="${TARGETARCH}"

# Set our service to be the default command
ENTRYPOINT ["/usr/local/bin/dbn-duckduck-goose"]
