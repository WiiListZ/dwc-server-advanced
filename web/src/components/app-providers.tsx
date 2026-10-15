import { ThemeProvider } from "next-themes"
import type { ReactNode } from "react"
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import { ConfirmProvider } from "@/components/confirm"
import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"

import "@/index.css"

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem disableTransitionOnChange storageKey="wfc-theme">
      <TooltipProvider delayDuration={200}>
        <ConfirmProvider>
          {children}
          <Toaster richColors closeButton position="bottom-right" />
        </ConfirmProvider>
      </TooltipProvider>
    </ThemeProvider>
  )
}

export function mount(node: ReactNode) {
  createRoot(document.getElementById("root")!).render(
    <StrictMode>
      <AppProviders>{node}</AppProviders>
    </StrictMode>,
  )
}
