import type { ComponentProps } from "react"

import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

export type Tone = "neutral" | "success" | "warning" | "danger" | "info"

// Badges stay neutral by design; tone is kept so callers still describe intent.
export function StatusBadge({ tone: _tone, className, ...props }: ComponentProps<typeof Badge> & { tone?: Tone }) {
  return <Badge variant="outline" className={cn("text-muted-foreground", className)} {...props} />
}

export function remainingTone(label: string): Tone {
  switch (label) {
    case "Revoked":
    case "Expired":
      return "neutral"
    case "Logged":
      return "warning"
    case "Permanent":
      return "danger"
    default:
      return "danger"
  }
}

export function logLevelTone(level?: string): Tone {
  switch (level) {
    case "success":
      return "success"
    case "warn":
      return "warning"
    case "danger":
    case "error":
      return "danger"
    default:
      return "info"
  }
}
