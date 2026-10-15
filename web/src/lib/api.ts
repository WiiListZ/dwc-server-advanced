export class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

type RequestOptions = Omit<RequestInit, "body"> & { body?: unknown }

export async function api<T = unknown>(path: string, options: RequestOptions = {}): Promise<T> {
  const { body, headers, ...rest } = options
  const init: RequestInit = {
    ...rest,
    credentials: "same-origin",
    headers: { Accept: "application/json", ...(headers as Record<string, string>) },
  }
  if (body !== undefined) {
    init.body = typeof body === "string" ? body : JSON.stringify(body)
    ;(init.headers as Record<string, string>)["Content-Type"] = "application/json"
  }
  const response = await fetch(path, init)
  const data = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new ApiError((data && data.error) || `HTTP ${response.status}`, response.status)
  }
  return data as T
}

export const post = <T = unknown>(path: string, body: unknown = {}) => api<T>(path, { method: "POST", body })

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

export function list<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

export type Role = "owner" | "moderator" | "player"

export type Rights = {
  view_users: boolean
  view_bans: boolean
  ban: boolean
  unban: boolean
  kick: boolean
  message: boolean
  manage_rooms_create: boolean
  manage_rooms_join: boolean
  manage_rooms_delete: boolean
  manage_rooms_end_game: boolean
  view_logs: boolean
  view_reports: boolean
  manage_ghosts: boolean
  manage_games: boolean
  manage_rights: boolean
  private_room_kick: boolean
  report_players: boolean
  open_host: boolean
  log_types: string[]
}

export type RightKey = Exclude<keyof Rights, "log_types">

export type LinkedProfile = { pid: number; fc?: string; name?: string }

export type Account = {
  id: number
  username: string
  role: Role
  created: string
  pid?: number
  fc?: string
  name?: string
  linked?: string
  linked_profiles?: LinkedProfile[]
  rights: Rights
}

export type Session = {
  authenticated: boolean
  setup_required: boolean
  account?: Account
  avatar_url?: string
  features?: { profiles?: boolean }
}

export type ProfileLink = { fc: string; username: string; avatar_url?: string }

export type ProfileStats = {
  matches: number
  last_match?: string
  games: string[]
  vr?: number
  br?: number
  time_trials: number
}

export type PublicProfile = {
  username: string
  role: Role
  created: string
  bio: string
  avatar_url?: string
  show_stats: boolean
  linked: boolean
  status: {
    online: boolean
    name?: string
    game?: string
    game_code?: string
    room?: { id: string; type: string; state?: string; players: number; host: boolean; rk?: string; game: string }
  }
  profiles?: { name: string; fc?: string }[]
  stats?: ProfileStats
  can_moderate?: boolean
  is_own?: boolean
}

export type MyProfile = { username: string; bio: string; avatar_url?: string; show_stats: boolean }

export type GameStats = { online: number; active: number; groups: number }
export type Stats = Record<string, GameStats>

export type OnlineUser = {
  pid?: number
  fc?: string
  name: string
  game: string
  game_code: string
  status?: string
  address?: string
  restricted?: boolean
  payload_features?: boolean
}

export type RoomPlayer = {
  pid?: number
  fc?: string
  name?: string
  state?: string
  count?: string
}

export type Room = {
  id: string
  game: string
  created?: string
  type: string
  suspend?: boolean
  host?: string
  host_pid?: number
  rk?: string
  state?: string
  players: Record<string, RoomPlayer>
  race?: { num: number; course: number; cc: number }
  race_end?: RaceEndStatus
}

export type RaceEndStatus = {
  requested: string
  mode: "auto" | "native_try" | "force_disconnect" | string
  payload: number
  disconnect: number
  finished: number
  fallback: boolean
  results?: Record<string, string>
}

export type BanRecord = {
  kind?: string
  pid?: number
  fc?: string
  name?: string
  status?: string
  reason?: string
  reason_hidden?: string
  moderator?: string
  tos?: boolean
  issued?: string
  expires?: string
  revoked?: string
  room_id?: string
  console_id?: number
}

export type LogEntry = {
  id: string
  type: string
  level: string
  created: string
  summary: string
  moderator?: string
  target_pid?: number
  target_name?: string
  target_fc?: string
  room_id?: string
  reason?: string
  note?: string
  expires?: string
  tos?: boolean
  extra?: string
}

export type ReportSummary = {
  pid: number
  fc?: string
  name?: string
  reports: number
  moderator_reports: number
  user_reports: number
  last_reported: string
  online: boolean
  room_id?: string
  room_type?: string
  room_game?: string
}

export type ReportDetail = {
  id: number
  room_id: string
  game?: string
  type?: string
  reporter_pid: number
  reporter_fc?: string
  reporter_name?: string
  reporter_type: string
  target_pid: number
  reason: string
  created: string
}

export type MatchLog = {
  id: number
  room_id: string
  game: string
  type: string
  player_count: number
  created: string
}

export type DisabledGame = {
  id: number
  gameCodePrefix: string
  gameName: string
  nasReturnCode: string
  errorCode: number
  message: string
  moderator?: string
  created: string
  updated: string
}

export type MKWRanking = {
  id: number
  regionid: number
  courseid: number
  score: number
  pid: number
  fc: string
  playerinfo: string
  has_ghost: boolean
  ghost_size: number
  upload_time?: string
}

export type ConsoleProfile = { pid: number; name?: string; fc?: string; has_ban?: boolean; primary?: boolean }

export type ConsoleSummary = {
  id: number
  placeholder?: string
  type?: string
  console_fc?: string
  assigned_at?: string
  linked_at?: string
  last_login?: string
  has_ban?: boolean
  profiles?: ConsoleProfile[]
}

export type OpenHostMode = { value: string; label: string; description: string }

export type OpenHostProfile = {
  pid: number
  fc?: string
  name?: string
  primary: boolean
  enabled: boolean
  mode: string
  list: number[]
  list_display: string[]
  online: boolean
  right_locked: boolean
  console_id?: number
  console_name?: string
  console_type?: string
  console_fc?: string
}

export type ErrorCodeEntry = { code: string; title?: string; description?: string; action?: string }
