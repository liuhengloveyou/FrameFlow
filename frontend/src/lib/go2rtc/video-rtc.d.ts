/** Ambient types for go2rtc VideoRTC custom element. */
declare module '@/lib/go2rtc/video-rtc' {
  export class VideoRTC extends HTMLElement {
    DISCONNECT_TIMEOUT: number
    RECONNECT_TIMEOUT: number
    CODECS: string[]
    mode: string
    media: string
    background: boolean
    visibilityThreshold: number
    visibilityCheck: boolean
    pcConfig: RTCConfiguration
    wsState: number
    pcState: number
    video: HTMLVideoElement | null
    ws: WebSocket | null
    wsURL: string
    pc: RTCPeerConnection | null
    src: string | URL
    play(): void
    send(value: object): void
    oninit(): void
    onconnect(): boolean
    ondisconnect(): void
    onopen(): void
    onclose(): void
  }
}
