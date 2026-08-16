import { createContext, useContext } from 'react'

export type ShellNavContextValue = {
  isCollapsed: boolean
  toggle: () => void
}

const ShellNavContext = createContext<ShellNavContextValue | null>(null)

export function ShellNavProvider({
  value,
  children,
}: {
  value: ShellNavContextValue
  children: React.ReactNode
}) {
  return <ShellNavContext.Provider value={value}>{children}</ShellNavContext.Provider>
}

export function useShellNav() {
  return useContext(ShellNavContext)
}
