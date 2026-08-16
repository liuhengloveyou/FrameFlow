import { type ReactNode } from 'react'
import { QueryClientProvider, type QueryClient } from '@tanstack/react-query'
import { Theme } from '@astryxdesign/core/theme'
import { LinkProvider } from '@astryxdesign/core/Link'
import { LayerProvider } from '@astryxdesign/core/Layer'
import { neutralTheme } from '@astryxdesign/theme-neutral'
import { RouterLink } from '@/components/router-link'

export function AppProviders({
  children,
  queryClient,
}: {
  children: ReactNode
  queryClient: QueryClient
}) {
  return (
    <Theme theme={neutralTheme} mode="light">
      <LinkProvider component={RouterLink}>
        <LayerProvider>
          <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
        </LayerProvider>
      </LinkProvider>
    </Theme>
  )
}
