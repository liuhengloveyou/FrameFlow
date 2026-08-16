/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_DEV_API_TARGET?: string
  readonly VITE_USE_PASSPORT?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
