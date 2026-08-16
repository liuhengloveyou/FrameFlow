# App

The application module is responsible for reading configuration files, running other modules and setting up [logs](#log).

The configuration can be edited through the application's WebUI with code highlighting, syntax and specification checking.

- By default, FrameFlow searches for `frameflow.yaml` in the current working directory, then `configs/frameflow.yaml` (see `internal/app/config.go`).
- FrameFlow supports multiple config files:
  - `FrameFlow -c config1.yaml -c config2.yaml -c config3.yaml`
- FrameFlow supports inline config in multiple formats from the command line:
  - **YAML**: `FrameFlow -c '{log: {format: text}}'`
  - **JSON**: `FrameFlow -c '{"log":{"format":"text"}}'`
  - **key=value**: `FrameFlow -c log.format=text`
- Each subsequent config will overwrite the previous one (but only for defined params)

```
FrameFlow -config "{log: {format: text}}" -config /config/frameflow.yaml -config "{rtsp: {listen: ''}}" -config /usr/local/frameflow/frameflow.yaml
```

or a simpler version

```
FrameFlow -c log.format=text -c /config/frameflow.yaml -c rtsp.listen='' -c /usr/local/frameflow/frameflow.yaml
```

## Environment variables

There is support for loading external variables into the config. First, they will be loaded from [credential files](https://systemd.io/CREDENTIALS). If `CREDENTIALS_DIRECTORY` is not set, then the key will be loaded from an environment variable. If no environment variable is set, then the string will be left as-is.

```yaml
streams:
  camera1: rtsp://rtsp:${CAMERA_PASSWORD}@192.168.1.123/av_stream/ch0

rtsp:
  username: ${RTSP_USER:admin}   # "admin" if "RTSP_USER" not set
  password: ${RTSP_PASS:secret}  # "secret" if "RTSP_PASS" not set
```

## JSON Schema

Editors like [GoLand](https://www.jetbrains.com/go/) and [VS Code](https://code.visualstudio.com/) support autocomplete and syntax validation.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/AlexxIT/go2rtc/master/www/schema.json
```

or from a running go2rtc:

```yaml
# yaml-language-server: $schema=http://localhost:8888/schema.json
```

## Defaults

- Default values may change in updates
- FFmpeg module has many presets, they are not listed here because they may also change in updates

```yaml
api:
  listen: ":8888"  # default public port for WebUI and HTTP API

ffmpeg:
  bin: "ffmpeg"    # default binary path for FFmpeg

log:
  level: "info"    # default log level
  output: "stdout"
  time: "UNIXMS"

rtsp:
  listen: ":8554"  # default public port for RTSP server
  default_query: "video&audio"

srtp:
  listen: ":8443"  # default public port for SRTP server (used for HomeKit)

webrtc:
  listen: ":8555"  # default public port for WebRTC server (TCP and UDP)
  ice_servers:
    - urls: [ "stun:stun.cloudflare.com:3478", "stun:stun.l.google.com:19302" ]
```

## Log

You can set different log levels for different modules.

```yaml
log:
  format: ""        # empty (default, autodetect color support), color, json, text 
  level: "info"     # disabled, trace, debug, info (default), warn, error
  output: "stdout"  # empty (only to memory), stderr, stdout (default), file:path (e.g. file:logs/frameflow.log)
  time: "UNIXMS"    # empty (disable timestamp), UNIXMS (default), UNIXMICRO, UNIXNANO
  
  api: trace   # module name: log level
```

Modules: `api`, `streams`, `rtsp`, `webrtc`, `mp4`, `hls`, `mjpeg`, `hass`, `homekit`, `onvif`, `rtmp`, `webtorrent`, `wyoming`, `echo`, `exec`, `expr`, `ffmpeg`, `wyze`, `xiaomi`.
