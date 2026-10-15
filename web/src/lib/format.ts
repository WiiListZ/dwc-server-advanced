import type { Account, BanRecord, Room, RoomPlayer } from "@/lib/api"

export function fmtDate(value?: string | null, fallback = "Permanent"): string {
  if (!value) return fallback
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

export function fmtOptionalDate(value?: string | null): string {
  return value ? fmtDate(value) : "-"
}

export function fmtLogTime(value?: string | null): string {
  if (!value) return ""
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

export function relativeTime(value?: string | null): string {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  const seconds = Math.round((date.getTime() - Date.now()) / 1000)
  const units: [Intl.RelativeTimeFormatUnit, number][] = [
    ["year", 31536000],
    ["month", 2592000],
    ["week", 604800],
    ["day", 86400],
    ["hour", 3600],
    ["minute", 60],
  ]
  const formatter = new Intl.RelativeTimeFormat("en", { numeric: "auto" })
  for (const [unit, size] of units) {
    if (Math.abs(seconds) >= size) return formatter.format(Math.round(seconds / size), unit)
  }
  return formatter.format(seconds, "second")
}

export function humanDuration(startValue: string | Date | undefined, endValue?: string): string {
  if (!endValue) return "permanently"
  const end = new Date(endValue)
  const start = startValue ? new Date(startValue) : new Date()
  if (Number.isNaN(end.getTime()) || Number.isNaN(start.getTime())) return "until " + fmtDate(endValue)
  const minutes = Math.max(1, Math.round((end.getTime() - start.getTime()) / 60000))
  const day = 1440
  const week = 10080
  const month = 43200
  let value = minutes
  let unit = "minute"
  if (minutes >= 60 && minutes % 60 === 0) {
    value = minutes / 60
    unit = "hour"
  }
  if (minutes >= day && minutes % day === 0) {
    value = minutes / day
    unit = "day"
  }
  if (minutes >= week && minutes % week === 0) {
    value = minutes / week
    unit = "week"
  }
  if (minutes >= 60 * day && minutes % month === 0) {
    value = minutes / month
    unit = "month"
  }
  return `${value} ${unit}${value === 1 ? "" : "s"}`
}

export const banStatus = (ban: BanRecord) => ban.status || "active"
export const isPrivateRoomKick = (ban: BanRecord) => ban.kind === "private_room_kick" || ban.kind === "room_kick"
export const isModeratorKick = (ban: BanRecord) => ban.kind === "moderator_kick"
export const isKickRecord = (ban: BanRecord) => isPrivateRoomKick(ban) || isModeratorKick(ban)

export function moderationKind(ban: BanRecord): string {
  if (isPrivateRoomKick(ban)) return "Private room kick"
  if (isModeratorKick(ban)) return "Moderator kick"
  return "Ban"
}

export function banDurationText(ban: BanRecord): string {
  if (isKickRecord(ban)) return moderationKind(ban)
  const end = banStatus(ban) === "revoked" && ban.revoked ? ban.revoked : ban.expires
  if (!end) return "Console and user banned permanently"
  return "Console and user banned for " + humanDuration(ban.issued, end)
}

// compactDuration renders a span as its two largest units, e.g. "6d 23h" or "42m".
export function compactDuration(ms: number): string {
  const minutes = Math.max(1, Math.round(ms / 60000))
  const days = Math.floor(minutes / 1440)
  const hours = Math.floor((minutes % 1440) / 60)
  const mins = minutes % 60
  if (days >= 365) return `${Math.floor(days / 365)}y ${Math.floor((days % 365) / 30)}mo`
  if (days >= 30) return `${Math.floor(days / 30)}mo ${days % 30}d`
  if (days > 0) return hours ? `${days}d ${hours}h` : `${days}d`
  if (hours > 0) return mins ? `${hours}h ${mins}m` : `${hours}h`
  return `${mins}m`
}

export function remainingBanTime(ban: BanRecord): string {
  if (isKickRecord(ban)) return "Logged"
  if (banStatus(ban) === "revoked") return "Revoked"
  if (!ban.expires) return "Permanent"
  const end = new Date(ban.expires)
  if (Number.isNaN(end.getTime())) return "Unknown"
  if (end.getTime() <= Date.now()) return "Expired"
  return compactDuration(end.getTime() - Date.now())
}

export type BanFilter = "all" | "active" | "expired" | "revoked" | "kick" | "mine"

export function filterBans(bans: BanRecord[], filter: BanFilter, query: string, account?: Account): BanRecord[] {
  const q = query.trim().toLowerCase()
  const ownPid = account?.pid ? String(account.pid) : ""
  const ownFc = account?.fc ? String(account.fc) : ""
  return bans.filter((ban) => {
    if (filter === "mine") {
      const matchesPid = ownPid && String(ban.pid || "") === ownPid
      const matchesFc = ownFc && String(ban.fc || "") === ownFc
      if (!matchesPid && !matchesFc) return false
    } else if (filter === "kick") {
      if (!isKickRecord(ban)) return false
    } else if (filter !== "all" && banStatus(ban) !== filter) {
      return false
    }
    if (!q) return true
    return [ban.pid, ban.fc, ban.name, ban.reason, ban.reason_hidden, ban.moderator, ban.room_id, moderationKind(ban)].some(
      (value) => String(value ?? "").toLowerCase().includes(q),
    )
  })
}

export const roomType = (room: Room) => room.type || "unknown"

export function roomStateLabel(state?: string): string {
  switch (state) {
    case "playing":
      return "Playing"
    case "waiting_for_lane_selection":
      return "Lane selection"
    case "waiting_for_course_selection":
      return "Course selection"
    case "matchmaking":
      return "Matchmaking"
    default:
      return state || "Room"
  }
}

export function playerStateLabel(state?: string): string {
  switch (state) {
    case "playing":
      return "Playing"
    case "spectating":
      return "Spectating"
    case "waiting_for_lane_selection":
      return "Lane selection"
    case "waiting_for_course_selection":
      return "Course selection"
    default:
      return "Waiting"
  }
}

export const playerName = (player: RoomPlayer) => player.name || player.fc || `PID ${player.pid ?? "-"}`
export const playerIdentifier = (player: RoomPlayer) => (player.pid ? String(player.pid) : player.fc || "")

export function accountProfileIDs(account?: Account): Set<string> {
  const ids = new Set<string>()
  if (!account) return ids
  if (account.pid) ids.add(String(account.pid))
  for (const profile of account.linked_profiles ?? []) {
    if (profile?.pid) ids.add(String(profile.pid))
  }
  return ids
}

export function playerMatchesAccount(player: RoomPlayer, account: Account | undefined, ids = accountProfileIDs(account)): boolean {
  if (!account) return false
  return (!!account.fc && String(player.fc || "") === String(account.fc)) || (!!player.pid && ids.has(String(player.pid)))
}

export function roomContainsAccount(room: Room, account?: Account): boolean {
  if (!account) return false
  const ids = accountProfileIDs(account)
  if (room.host_pid && ids.has(String(room.host_pid))) return true
  return Object.values(room.players || {}).some((player) => playerMatchesAccount(player, account, ids))
}

export function sortedPlayers(room: Room, account?: Account): [string, RoomPlayer][] {
  const ids = accountProfileIDs(account)
  return Object.entries(room.players || {}).sort(([a, pa], [b, pb]) => {
    const aSelf = playerMatchesAccount(pa, account, ids)
    const bSelf = playerMatchesAccount(pb, account, ids)
    if (aSelf !== bSelf) return aSelf ? -1 : 1
    return Number(a) - Number(b)
  })
}

export function sortRoomsSelfFirst(rooms: Room[], account?: Account): Room[] {
  if (!account) return rooms
  const mine: Room[] = []
  const others: Room[] = []
  for (const room of rooms) (roomContainsAccount(room, account) ? mine : others).push(room)
  return mine.concat(others)
}

export function roleLabel(role?: string): string {
  if (role === "player") return "User"
  if (role === "moderator") return "Moderator"
  if (role === "owner") return "Administrator"
  return role || "Account"
}

export function initials(name: string): string {
  const words = name.split(/[^\p{L}\p{N}]+/u).filter(Boolean)
  if (words.length > 1) return (words[0][0] + words[1][0]).toUpperCase()
  const letters = name.replace(/[^\p{L}\p{N}]/gu, "")
  return (letters.slice(0, 2) || "WF").toUpperCase()
}

export const truncate = (value: string | undefined, max: number) => {
  const text = String(value || "")
  return text.length > max ? text.slice(0, max - 1) + "…" : text
}
