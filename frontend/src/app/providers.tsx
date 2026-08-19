import { ThemeProvider } from '@appica/ui-react/providers/theme-provider'
import { QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { queryClient } from '@/lib/query-client'
import { AuthBootstrap } from './auth-bootstrap'

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider defaultTheme="system" enableSystem storageKey="openlist-scraper-theme">
      <QueryClientProvider client={queryClient}>
        <AuthBootstrap>{children}</AuthBootstrap>
      </QueryClientProvider>
    </ThemeProvider>
  )
}
