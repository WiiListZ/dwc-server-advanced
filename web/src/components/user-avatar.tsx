import { useEffect, useState } from "react"

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { api, list, type ProfileLink } from "@/lib/api"
import { initials } from "@/lib/format"
import { cn } from "@/lib/utils"

export function UserAvatar({ name, src, className, fallbackClassName }: { name: string; src?: string; className?: string; fallbackClassName?: string }) {
  return (
    <Avatar className={className}>
      {src && <AvatarImage src={src} alt="" />}
      <AvatarFallback className={cn("text-[11px]", fallbackClassName)}>{initials(name || "?")}</AvatarFallback>
    </Avatar>
  )
}

export const profileHref = (username: string) => `/u/${encodeURIComponent(username)}`

/** Friend code → site profile of the online players that have one. */
export function useProfileLinks(enabled: boolean, refreshKey: unknown) {
  const [links, setLinks] = useState<Map<string, ProfileLink>>(new Map())
  useEffect(() => {
    if (!enabled) return
    let cancelled = false
    api<ProfileLink[]>("/api/public/profile-links")
      .then((rows) => {
        if (!cancelled) setLinks(new Map(list(rows).map((row) => [row.fc, row])))
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [enabled, refreshKey])
  return enabled ? links : new Map<string, ProfileLink>()
}
