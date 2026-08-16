# syntax=docker/dockerfile:labs

# 0. Prepare images
ARG PYTHON_VERSION="3.13"
ARG GO_VERSION="1.26"


# 1. Build frameflow binary
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

WORKDIR /build

RUN apk add git

# Cache dependencies
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath -o frameflow .


# 2. Final image
FROM python:${PYTHON_VERSION}-alpine AS base

# Install ffmpeg, tini (for signal handling),
# and other common tools for the echo source.
# alsa-plugins-pulse for ALSA support (+0MB)
# font-droid for FFmpeg drawtext filter (+2MB)
RUN apk add --no-cache tini ffmpeg ffplay bash curl jq alsa-plugins-pulse font-droid

# Hardware Acceleration for Intel CPU (+50MB)
ARG TARGETARCH

RUN if [ "${TARGETARCH}" = "amd64" ]; then apk add --no-cache libva-intel-driver intel-media-driver; fi

# Hardware: AMD and NVidia VAAPI (not sure about this)
# RUN libva-glx mesa-va-gallium
# Hardware: AMD and NVidia VDPAU (not sure about this)
# RUN libva-vdpau-driver mesa-vdpau-gallium (+150MB total)

COPY --from=build /build/frameflow /usr/local/bin/

EXPOSE 27788 8554 1935 8555 8555/udp
ENTRYPOINT ["/sbin/tini", "--"]
VOLUME /config
WORKDIR /config

COPY configs/frameflow.yaml /config/frameflow.yaml

CMD ["frameflow", "-c", "/config/frameflow.yaml"]
