# Streams（流管理）

本模块维护命名流表（`streams`）：登记拉流源、推流占位槽、预加载与对外发布。设备管理、预览 WebRTC / HTTP-FLV、RTMP/RTSP 服务端均以这里的 **流名（src）** 为键。

默认 API 端口：`27788`（见 `api.listen`）。

---

## 能力一览

| 能力 | 说明 |
|------|------|
| 拉流源 | yaml / 设备保存后写入 `streams.<name>`，如 `rtsp://…`、`rtmp://…`、`gb28181://…` |
| 推流占位 | `null` 或空源；RTMP publish / RTSP ANNOUNCE 写入该键 |
| 预加载 preload | 进程启动后自动拉起指定流（慢启动摄像头） |
| 发布 publish | 把已有流再推到 YouTube / Telegram 等 RTMP(S) |
| HTTP API | `/api/streams`、`/api/preload`、`/api/devices/rtmp`、`/api/devices/rtsp` |

H264/H265 视频轨走 **AVRing**：中途订阅从最近 IDR 起读再跟 live。

---

## 配置示例

```yaml
streams:
  live: null   # 常见 RTMP app=live 占位；真实设备多用 ingest_* 槽位
  cam_rtsp: rtsp://admin:pass@192.168.1.100:554/stream1#transport=tcp
  cam_gb: gb28181://34020000001320000001/34020000001320000001?transport=udp

preload:
  cam_rtsp: "video&audio"

publish:
  cam_rtsp:
    - rtmp://a.rtmp.youtube.com/live2/<stream_key>
```

设备推流一般在 UI「设备管理」创建（自动 `Ensure*IngestSlot`），不必手写 yaml。

---

## HTTP API（常用）

```text
GET  /api/streams
GET  /api/streams?src=<name>
PUT  /api/streams?src=<name>          # body: 源 URL 或 JSON
DELETE /api/streams?src=<name>

POST /api/devices/rtmp                # 创建 RTMP 推流槽
POST /api/devices/rtsp                # 创建 RTSP 推流槽
DELETE /api/devices/rtmp?stream_key=
DELETE /api/devices/rtsp?stream_key=
```

向摄像头回放音频（双向对讲类，需设备支持）：

```text
POST /api/streams?dst=<camera_stream>&src=ffmpeg:http://example.com/song.mp3#audio=pcma#input=file
```

空 `src` 可停止当前回放。

---

## 联调：ffmpeg / ffplay

以下假设：FrameFlow 已启动；协议配置中 **RTMP :1935**、**RTSP :8554** 已开启；`<stream_key>` 为已登记流名（设备推流保存后复制，或 `POST /api/devices/rtmp|rtsp` 返回）。

### 1. RTMP 推流 → 本机

macOS 摄像头（需强制帧率，避免 timebase 异常）：

```shell
ffmpeg -f avfoundation -framerate 30 -i "0:none" \
  -vf "fps=30,scale=1280:-2" -c:v libx264 -preset ultrafast -tune zerolatency \
  -pix_fmt yuv420p -g 30 -f flv \
  rtmp://127.0.0.1:1935/<stream_key>
```

文件循环推流：

```shell
ffmpeg -re -stream_loop -1 -i input.mp4 -c copy -f flv \
  rtmp://127.0.0.1:1935/<stream_key>
```

带推流密钥（tcUrl 查询参数，默认名 `push_key`）：

```shell
ffmpeg -re -i input.mp4 -c copy -f flv \
  "rtmp://127.0.0.1:1935/<stream_key>?push_key=<push_secret>"
```

### 2. RTSP ANNOUNCE 推流 → 本机

```shell
ffmpeg -re -i input.mp4 -c copy -rtsp_transport tcp -f rtsp \
  rtsp://127.0.0.1:8554/<stream_key>
```

带密钥：

```shell
ffmpeg -re -i input.mp4 -c copy -rtsp_transport tcp -f rtsp \
  "rtsp://127.0.0.1:8554/<stream_key>?push_key=<push_secret>"
```

### 3. 拉流播放（验收）

```shell
# RTMP play
ffplay -fflags nobuffer rtmp://127.0.0.1:1935/<stream_key>

# HTTP-FLV
ffplay -fflags nobuffer "http://127.0.0.1:27788/api/stream.flv?src=<stream_key>"

# RTSP play（本机 RTSP 服务出口）
ffplay -rtsp_transport tcp rtsp://127.0.0.1:8554/<stream_key>
```

浏览器预览：预览页 WebRTC，`src` 填 `<stream_key>`（不要填 `rtmp-ingest://` / `rtsp-ingest://`）。

### 4. 平台拉远端（设备「拉流」模式）

仅登记源即可，无需本机 publish。示例源写进 `streams` 或设备表单：

```text
rtsp://admin:pass@192.168.1.100:554/stream1#transport=tcp
rtmp://192.168.1.92/bcs/channel0_main.bcs?channel=0&stream=0&user=admin&password=xxx
```

用 ffplay 测摄像头本身（不经过 FrameFlow）：

```shell
ffplay -rtsp_transport tcp "rtsp://admin:pass@192.168.1.100:554/stream1"
```

### 5. HTTP-FLV 推入（较少用）

```shell
ffmpeg -re -i input.mp4 -c copy -f flv \
  "http://127.0.0.1:27788/api/stream.flv?dst=<stream_key>"
```

---

## 发布到外网（publish）

把已有流再推到 YouTube / Telegram 等（需 H264 + AAC）。不必为本机 RTMP 监听服务。

```yaml
publish:
  cam1:
    - rtmp://xxx.rtmp.youtube.com/live2/xxxx-xxxx-xxxx-xxxx-xxxx
```

或：

```text
POST /api/streams?src=cam1&dst=rtmps://...
```

---

## 预加载（preload）

```yaml
preload:
  cam1:                 # 默认 video&audio
  cam2: "video"
  cam3: "video=h264&audio=opus"
```

启动约 1s 后按表拉起，适合开机慢的 IPC。

---

## 常见源格式备忘

```yaml
streams:
  # RTSP
  dahua:  rtsp://admin:pass@192.168.1.90/cam/realmonitor?channel=1&subtype=0#transport=tcp
  tplink: rtsp://admin:pass@192.168.1.91/stream1#transport=udp

  # RTMP 拉远端
  reolink: rtmp://192.168.1.92/bcs/channel0_main.bcs?channel=0&stream=0&user=admin&password=pass

  # HTTP-FLV
  reolink_flv: http://192.168.1.92/flv?port=1935&app=bcs&stream=channel0_main.bcs&user=admin&password=pass

  # ONVIF / 国标 / ffmpeg / exec
  onvif1: onvif://admin:pass@192.168.1.90?subtype=MediaProfile00000
  gb1:    gb28181://34020000001320000001/34020000001320000001?transport=udp
  pipe:   exec:ffmpeg -re -i bbb.mp4 -c copy -f flv -
  virt:   ffmpeg:virtual?video#video=h264
```

更细的 RTMP / RTSP 接入说明见 [`internal/rtmp/README.md`](../rtmp/README.md)、[`internal/rtsp/README.md`](../rtsp/README.md)。
