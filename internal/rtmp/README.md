# RTMP / FLV

FrameFlow 的 RTMP 推拉流与 HTTP-FLV 能力说明。

| 能力 | 说明 |
|------|------|
| RTMP 拉流（客户端） | 从外部 RTMP 源拉流转发，配置在 `streams` |
| RTMP 服务端 | 本机监听（默认 `:1935`），支持 publish / play |
| HTTP-FLV | `GET/POST /api/stream.flv`（默认 API `:27788`） |
| IPC 推流接入 | 占位流 `ingest_*` + 可选 `push_key` 鉴权 |

监听与对外主机配置持久化在系统库 `sys_config.rtmp`，**不写 yaml**。UI：系统 → 协议配置 → RTMP；API：`GET/PUT /api/rtmp/settings`。

---

## 1. 作为客户端拉外部 RTMP

```yaml
streams:
  rtmp_stream: rtmp://192.168.1.123/live/camera1
```

---

## 2. 作为服务端（publish / play）

先在「系统 → 协议配置 → RTMP」启用监听并设置端口。

**文件推流（转封装）：**

```shell
ffmpeg -re -i input.mp4 -c copy -f flv rtmp://127.0.0.1:1935/<stream_key>
```

**拉流：**

```shell
ffplay rtmp://127.0.0.1:1935/<stream_key>
ffplay "http://127.0.0.1:27788/api/stream.flv?src=<stream_key>"
```

中途打开预览 / 拉流时，H264/H265 从视频轨 **AVRing 最近 IDR** 起读再跟 live（约保留 3s GOP 历史），不必干等下一关键帧。

**HTTP-FLV 推入（较少用）：**

```shell
ffmpeg -re -i input.mp4 -c copy -f flv \
  "http://127.0.0.1:27788/api/stream.flv?dst=<stream_key>"
```

`<stream_key>` 为 `streams` 中已存在的流名（含设备推流占位键）。

---

## 3. IPC 推流接入（设备 / ffmpeg publish）

### 3.1 流名规则

每条推流槽位对应 `streams.<stream_key>`（占位可为 `null`）。

设备侧 RTMP 的 **app** 可与 `stream_key` 相同，或常见为 `live` + publish 子流名。服务端映射为单键（与 AINVR / go2rtc 习惯一致）：

```text
ReplaceAll(app, "/", "_") + "_" + ReplaceAll(stream, "/", "_")
```

仅把 `/` 换成 `_`；`.`、Unicode 等保留。若 publish 子流名为空，则键即为 app。

### 3.2 登记槽位

1. 系统 → 协议配置 → RTMP：端口、对外主机。
2. 设备管理：协议 RTMP → 接入方式「推流」→ 保存后复制 `stream_key` / 密钥。
3. 或 API：`POST /api/devices/rtmp`，例如 `{"stream_key":"cam01","allow_anonymous":false}`。未指定 `push_secret` 且不允许匿名时，服务端生成密钥并在响应中**只返回一次**。

删除：设备管理删摄像头；或 `DELETE /api/devices/rtmp?stream_key=...`；或 `DELETE /api/streams?src=...`。

### 3.3 鉴权

密钥走 RTMP **connect** 的 **tcUrl** 查询参数（默认名 `push_key`）：

```shell
ffmpeg -re -i input.mp4 -c copy -f flv \
  -rtmp_tcurl "rtmp://127.0.0.1:1935/cam01?push_key=SECRET" \
  rtmp://127.0.0.1:1935/cam01
```

未登记密钥的槽位允许匿名推流。

---

## 4. 本地联调（macOS 桌面采集）

将示例中的 `ingest_<hex>` 换成设备管理里的 `stream_key`。HTTP API 默认 `:27788`，RTMP 默认 `:1935`。

### 4.1 推流（必须强制输出帧率）

macOS `avfoundation` 常把时间基报成 `1000k tbr/tbn`。若只写输入 `-framerate 30`、不写输出 `fps=30` / `-r 30`，libx264 会按「约百万 fps」压码，出现：

- `MB rate ... > level limit`
- QP 顶死、P 帧只有一两百字节、实际码率几十 kb/s
- 拉流无画或花屏

**推荐命令：**

```shell
ffmpeg -f avfoundation -framerate 30 -capture_cursor 1 -i "0:none" \
  -vf "fps=30,scale=1280:-2" \
  -c:v libx264 -preset veryfast -tune zerolatency \
  -pix_fmt yuv420p -profile:v baseline -level 4.0 \
  -g 60 -keyint_min 60 -sc_threshold 0 \
  -b:v 2500k -maxrate 2500k -bufsize 5000k \
  -an -f flv rtmp://127.0.0.1:1935/ingest_e7159148f5c2721e
```

正常时应看到约 `30 fps`、码率接近设定值，I 帧为数 KB～数十 KB（随画面变化）。

带推流密钥时：

```shell
ffmpeg -f avfoundation -framerate 30 -capture_cursor 1 -i "0:none" \
  -vf "fps=30,scale=1280:-2" \
  -c:v libx264 -preset veryfast -tune zerolatency \
  -pix_fmt yuv420p -profile:v baseline -level 4.0 \
  -g 60 -keyint_min 60 -sc_threshold 0 \
  -b:v 2500k -maxrate 2500k -bufsize 5000k \
  -an -f flv \
  -rtmp_tcurl "rtmp://127.0.0.1:1935/ingest_e7159148f5c2721e?push_key=SECRET" \
  rtmp://127.0.0.1:1935/ingest_e7159148f5c2721e
```

### 4.2 拉流（推流保持运行时另开终端）

```shell
# HTTP-FLV
ffplay -fflags nobuffer \
  "http://127.0.0.1:27788/api/stream.flv?src=ingest_e7159148f5c2721e"

# RTMP
ffplay -fflags nobuffer rtmp://127.0.0.1:1935/ingest_e7159148f5c2721e
```

浏览器预览走 WebRTC（监控页），观感应与 ffplay 一致。

### 4.3 不要这样推（易踩坑）

```shell
# 缺少输出 fps=30 → 时间基错乱，码流几乎不可播
ffmpeg -f avfoundation -framerate 30 -capture_cursor 1 -i "0:none" \
  -c:v libx264 -preset veryfast -tune zerolatency -pix_fmt yuv420p \
  -g 60 -b:v 2500k \
  -f flv rtmp://127.0.0.1:1935/ingest_e7159148f5c2721e
```

---

## 5. 兼容性摘录

| 来源 | 角色 | 结果 |
|------|------|------|
| OBS 31 | 推到本服务 | OK |
| OpenIPC | 推到本服务 | OK |
| FFmpeg | 推/拉 | OK（注意帧率，见上） |
| GoPro | 推流 | OK（如 1080p / 5Mbps） |
| Reolink 等 IPC | 本服务去拉其 RTMP | 视机型固件 |
