import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import { toast } from "sonner"

import type { OnlineSample } from "@/components/charts"
import { ApiError, api, errorMessage, list, type Account, type BanRecord, type OnlineUser, type RightKey, type Room, type Session, type Stats } from "@/lib/api"
import { ROOM_RIGHTS, can as canFor, canAny as canAnyFor } from "@/lib/rights"

export type SectionId = "dashboard" | "live" | "reports" | "bans" | "activity" | "ghosts" | "games" | "team" | "server"

export const SECTIONS: SectionId[] = ["dashboard", "live", "reports", "bans", "activity", "ghosts", "games", "team", "server"]

// PlayerRef identifies a player by whatever the caller knows; the player sheet
// resolves the rest from live data.
export type PlayerRef = { pid?: number; fc?: string; name?: string }
export type ActionKind = "ban" | "kick" | "message"
export type ActionRequest = { kind: ActionKind; player?: PlayerRef }

type Dashboard = { stats: Stats; users: OnlineUser[]; rooms: Room[]; bans: BanRecord[] }

type PanelContextValue = {
  session: Session
  account: Account
  can: (right: RightKey) => boolean
  canAny: (rights: RightKey[]) => boolean
  isOwner: boolean
  section: SectionId
  navigate: (section: SectionId) => void
  dashboard: Dashboard
  dashboardLoading: boolean
  dashboardUpdated: Date | null
  refreshDashboard: () => Promise<void>
  history: OnlineSample[]
  player: PlayerRef | null
  openPlayer: (player: PlayerRef) => void
  closePlayer: () => void
  roomId: string | null
  openRoom: (roomId: string) => void
  closeRoom: () => void
  action: ActionRequest | null
  openAction: (kind: ActionKind, player?: PlayerRef) => void
  closeAction: () => void
  createRoomHost: string | null
  openCreateRoom: (host?: string) => void
  closeCreateRoom: () => void
  joinRoomId: string | null
  openJoinDialog: (roomId: string) => void
  closeJoinDialog: () => void
  activityQuery: string
  showActivityFor: (query: string) => void
  setActivityQuery: (query: string) => void
  subscribe: (area: string, handler: () => void) => () => void
}

const PanelContext = createContext<PanelContextValue | null>(null)

export function usePanel() {
  const ctx = useContext(PanelContext)
  if (!ctx) throw new Error("usePanel must be used inside PanelProvider")
  return ctx
}

export function canModerate(account: Account) {
  return account.role === "owner" || canAnyFor(account, ["ban", "kick", "unban", "message"])
}

export function sectionAllowed(section: SectionId, account: Account): boolean {
  const can = (right: RightKey) => canFor(account, right)
  switch (section) {
    case "dashboard":
      return true
    case "live":
      return can("view_users") || canAnyFor(account, ROOM_RIGHTS)
    case "reports":
      return can("view_reports")
    case "bans":
      return can("view_bans")
    case "activity":
      return can("view_logs")
    case "ghosts":
      return can("manage_ghosts")
    case "games":
      return can("manage_games")
    case "team":
    case "server":
      return account.role === "owner"
  }
}

export const sectionPath = (section: SectionId) => `/panel/${section}`

// Each section has its own route (/panel/<section>). The server answers 404
// for sections the account cannot open, so the path is trusted here.
function sectionFromLocation(account: Account): SectionId {
  const fromPath = window.location.pathname.replace(/\/+$/, "").split("/")[2] as SectionId
  const fromHash = window.location.hash.replace(/^#/, "") as SectionId
  for (const candidate of [fromHash, fromPath]) {
    if (SECTIONS.includes(candidate) && sectionAllowed(candidate, account)) return candidate
  }
  return "dashboard"
}

const MAX_SAMPLES = 120

export function PanelProvider({ session, children }: { session: Session; children: ReactNode }) {
  const account = session.account!
  const [section, setSection] = useState<SectionId>(() => sectionFromLocation(account))
  const [dashboard, setDashboard] = useState<Dashboard>({ stats: {}, users: [], rooms: [], bans: [] })
  const [dashboardLoading, setDashboardLoading] = useState(true)
  const [dashboardUpdated, setDashboardUpdated] = useState<Date | null>(null)
  const [history, setHistory] = useState<OnlineSample[]>([])
  const [player, setPlayer] = useState<PlayerRef | null>(null)
  const [roomId, setRoomId] = useState<string | null>(null)
  const [action, setAction] = useState<ActionRequest | null>(null)
  const [createRoomHost, setCreateRoomHost] = useState<string | null>(null)
  const [joinRoomId, setJoinRoomId] = useState<string | null>(null)
  const [activityQuery, setActivityQuery] = useState("")
  const listeners = useRef(new Map<string, Set<() => void>>())
  const timers = useRef(new Map<string, number>())

  const can = useCallback((right: RightKey) => canFor(account, right), [account])
  const canAny = useCallback((rights: RightKey[]) => canAnyFor(account, rights), [account])

  const navigate = useCallback(
    (next: SectionId) => {
      const target = sectionAllowed(next, account) ? next : "dashboard"
      setSection(target)
      if (window.location.pathname !== sectionPath(target)) window.history.pushState(null, "", sectionPath(target))
      window.scrollTo({ top: 0 })
    },
    [account],
  )

  useEffect(() => {
    // Old links used /panel#section; move them to the real route.
    if (window.location.pathname !== sectionPath(section) || window.location.hash) {
      window.history.replaceState(null, "", sectionPath(section))
    }
    const onPop = () => setSection(sectionFromLocation(account))
    window.addEventListener("popstate", onPop)
    return () => window.removeEventListener("popstate", onPop)
    // Runs once per account; section changes are pushed by navigate.
  }, [account])

  const refreshDashboard = useCallback(async () => {
    try {
      const [stats, users, rooms, bans] = await Promise.all([
        api<Stats>("/api/stats"),
        can("view_users") ? api<OnlineUser[]>("/api/panel/users") : Promise.resolve([]),
        canAny(ROOM_RIGHTS) ? api<Room[]>("/api/panel/rooms") : Promise.resolve([]),
        can("view_bans") ? api<BanRecord[]>("/api/panel/bans") : Promise.resolve([]),
      ])
      const next = { stats: stats || {}, users: list(users), rooms: list(rooms), bans: list(bans) }
      setDashboard(next)
      setDashboardUpdated(new Date())
      const global = next.stats.global ?? { online: 0, active: 0, groups: 0 }
      setHistory((prev) => [...prev, { time: Date.now(), online: global.online, active: global.active, rooms: global.groups }].slice(-MAX_SAMPLES))
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        window.location.replace("/login")
        return
      }
      toast.error(errorMessage(error))
    } finally {
      setDashboardLoading(false)
    }
  }, [can, canAny])

  const subscribe = useCallback((area: string, handler: () => void) => {
    const set = listeners.current.get(area) ?? new Set()
    set.add(handler)
    listeners.current.set(area, set)
    return () => {
      set.delete(handler)
    }
  }, [])

  // Server-sent events say which areas changed; refetches are debounced per area.
  useEffect(() => {
    if (!window.EventSource) return
    const source = new EventSource("/api/panel/events")
    source.addEventListener("panel-update", (event) => {
      let areas: string[] = []
      try {
        areas = list(JSON.parse((event as MessageEvent).data || "{}").areas)
      } catch {
        return
      }
      for (const area of areas) {
        window.clearTimeout(timers.current.get(area))
        timers.current.set(
          area,
          window.setTimeout(() => {
            if (area === "dashboard") void refreshDashboard()
            listeners.current.get(area)?.forEach((fn) => fn())
          }, 400),
        )
      }
    })
    const pending = timers.current
    return () => {
      source.close()
      pending.forEach((id) => window.clearTimeout(id))
    }
  }, [refreshDashboard])

  useEffect(() => {
    void refreshDashboard()
    const id = window.setInterval(() => void refreshDashboard(), 15000)
    return () => window.clearInterval(id)
  }, [refreshDashboard])

  const showActivityFor = useCallback(
    (query: string) => {
      setActivityQuery(query)
      setPlayer(null)
      navigate("activity")
    },
    [navigate],
  )

  const value = useMemo<PanelContextValue>(
    () => ({
      session,
      account,
      can,
      canAny,
      isOwner: account.role === "owner",
      section,
      navigate,
      dashboard,
      dashboardLoading,
      dashboardUpdated,
      refreshDashboard,
      history,
      player,
      openPlayer: setPlayer,
      closePlayer: () => setPlayer(null),
      roomId,
      openRoom: setRoomId,
      closeRoom: () => setRoomId(null),
      action,
      openAction: (kind, target) => setAction({ kind, player: target }),
      closeAction: () => setAction(null),
      createRoomHost,
      openCreateRoom: (host) => setCreateRoomHost(host ?? ""),
      closeCreateRoom: () => setCreateRoomHost(null),
      joinRoomId,
      openJoinDialog: setJoinRoomId,
      closeJoinDialog: () => setJoinRoomId(null),
      activityQuery,
      showActivityFor,
      setActivityQuery,
      subscribe,
    }),
    [session, account, can, canAny, section, navigate, dashboard, dashboardLoading, dashboardUpdated, refreshDashboard, history, player, roomId, action, createRoomHost, joinRoomId, activityQuery, showActivityFor, subscribe],
  )

  return <PanelContext.Provider value={value}>{children}</PanelContext.Provider>
}

export function useLiveArea(area: string, handler: () => void) {
  const { subscribe } = usePanel()
  const saved = useRef(handler)
  useEffect(() => {
    saved.current = handler
  }, [handler])
  useEffect(() => subscribe(area, () => saved.current()), [area, subscribe])
}

export const playerKey = (p: PlayerRef) => (p.pid ? String(p.pid) : p.fc || "")
export const playerLabel = (p: PlayerRef) => p.name || p.fc || (p.pid ? `PID ${p.pid}` : "Player")
