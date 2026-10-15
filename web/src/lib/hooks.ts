import { useCallback, useEffect, useRef, useState } from "react"

import { api, type Session } from "@/lib/api"

export function useSession() {
  const [session, setSession] = useState<Session | null>(null)
  const [error, setError] = useState<string>("")

  const reload = useCallback(async () => {
    try {
      const data = await api<Session>("/api/panel/session")
      setSession(data)
      setError("")
      return data
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setSession({ authenticated: false, setup_required: false })
      return null
    }
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  return { session, setSession, reload, error, loading: session === null }
}

export function useInterval(callback: () => void, delay: number | null) {
  const saved = useRef(callback)
  useEffect(() => {
    saved.current = callback
  }, [callback])
  useEffect(() => {
    if (delay === null) return
    const id = window.setInterval(() => saved.current(), delay)
    return () => window.clearInterval(id)
  }, [delay])
}

export function useNow(intervalMs = 1000) {
  const [now, setNow] = useState(() => Date.now())
  useInterval(() => setNow(Date.now()), intervalMs)
  return now
}

const SETTINGS_KEY = "wwfc_public_settings"
export type DisplaySettings = { autoRefresh: boolean; pageSize: number }
export const PAGE_SIZES = [5, 10, 25, 50]

export function loadDisplaySettings(): DisplaySettings {
  const fallback = { autoRefresh: true, pageSize: 10 }
  try {
    const raw = window.localStorage.getItem(SETTINGS_KEY)
    if (!raw) return fallback
    const parsed = JSON.parse(raw)
    return {
      autoRefresh: parsed.autoRefresh !== false,
      pageSize: PAGE_SIZES.includes(Number(parsed.pageSize)) ? Number(parsed.pageSize) : 10,
    }
  } catch {
    return fallback
  }
}

export function saveDisplaySettings(settings: DisplaySettings) {
  try {
    window.localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings))
  } catch {
    // Storage can be unavailable (private mode); preferences then last for this page only.
  }
}

export function serverName(): string {
  const meta = document.querySelector<HTMLMetaElement>('meta[name="server-name"]')
  const value = meta?.content?.trim() ?? ""
  return value && !value.startsWith("{{") ? value : "WFC Server"
}
