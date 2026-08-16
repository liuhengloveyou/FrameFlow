# syntax=docker/dockerfile:labs

# 0. Prepare images
# only debian 13 (trixie) has latest ffmpeg
# https://packages.debian.org/trixie/ffmpeg
ARG DEBIAN_VERSION="trixie-slim"
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
FROM debian:${DEBIAN_VERSION}

# Prepare apt for buildkit cache
RUN rm -f /etc/apt/apt.conf.d/docker-clean \
  && echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' >/etc/apt/apt.conf.d/keep-cache

# Install ffmpeg, tini (for signal handling),
# and other common tools for the echo source.
# non-free for Intel QSV support (optional for hardware tests)
# mesa-va-drivers for AMD APU
# libasound2-plugins for ALSA support
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked --mount=type=cache,target=/var/lib/apt,sharing=locked \
    echo 'deb http://deb.debian.org/debian trixie non-free' > /etc/apt/sources.list.d/debian-non-free.list && \
    apt-get -y update && apt-get -y install ffmpeg tini \
        python3 curl jq \
        intel-media-va-driver-non-free \
        mesa-va-drivers \
        libasound2-plugins && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=build /build/frameflow /usr/local/bin/

EXPOSE 27788 8554 1935 8555 8555/udp
ENTRYPOINT ["/usr/bin/tini", "--"]
VOLUME /config
WORKDIR /config
# https://github.com/NVIDIA/nvidia-docker/wiki/Installation-(Native-GPU-Support)
ENV NVIDIA_VISIBLE_DEVICES all
ENV NVIDIA_DRIVER_CAPABILITIES compute,video,utility

COPY configs/frameflow.yaml /config/frameflow.yaml

CMD ["frameflow", "-c", "/config/frameflow.yaml"]
