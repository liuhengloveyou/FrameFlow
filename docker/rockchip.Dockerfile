# syntax=docker/dockerfile:labs

# 0. Prepare images
ARG PYTHON_VERSION="3.13-slim-bookworm"
ARG GO_VERSION="1.26-bookworm"


# 1. Build frameflow binary
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

WORKDIR /build

# Cache dependencies
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath -o frameflow .


# 2. Final image
FROM python:${PYTHON_VERSION}

# Prepare apt for buildkit cache
RUN rm -f /etc/apt/apt.conf.d/docker-clean \
  && echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' >/etc/apt/apt.conf.d/keep-cache

# Install ffmpeg, tini (for signal handling),
# and other common tools for the echo source.
# libasound2-plugins for ALSA support
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get -y update && apt-get -y install tini \
        curl jq \
        libasound2-plugins && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=build /build/frameflow /usr/local/bin/
ADD --chmod=755 https://github.com/MarcA711/Rockchip-FFmpeg-Builds/releases/download/6.1-8-no_extra_dump/ffmpeg /usr/local/bin

EXPOSE 27788 8554 1935 8555 8555/udp
ENTRYPOINT ["/usr/bin/tini", "--"]
VOLUME /config
WORKDIR /config

COPY configs/frameflow.yaml /config/frameflow.yaml

CMD ["frameflow", "-c", "/config/frameflow.yaml"]
