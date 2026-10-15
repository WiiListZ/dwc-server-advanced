import type { Account, RightKey, Rights } from "@/lib/api"

export type RightItem = { id: string; label: string; keys: RightKey[]; logsToggle?: boolean }
export type RightGroup = { title: string; items: RightItem[] }

export const RIGHT_GROUPS: RightGroup[] = [
  {
    title: "Ban and kick",
    items: [
      { id: "public_ban_kick", label: "Kick and ban on public rooms.", keys: ["ban", "kick"] },
      { id: "unban", label: "Unban users.", keys: ["unban"] },
      { id: "private_kick", label: "Kick on my private rooms.", keys: ["private_room_kick"] },
    ],
  },
  {
    title: "Communication",
    items: [{ id: "message", label: "Send login messages to users.", keys: ["message"] }],
  },
  {
    title: "Matchmaking rooms",
    items: [
      { id: "manage_rooms_create", label: "Create matchmaking rooms from the moderation panel.", keys: ["manage_rooms_create"] },
      { id: "manage_rooms_join", label: "Force profiles to join matchmaking rooms from the moderation panel.", keys: ["manage_rooms_join"] },
      { id: "manage_rooms_delete", label: "Delete matchmaking rooms from the moderation panel.", keys: ["manage_rooms_delete"] },
      { id: "manage_rooms_end_game", label: "End started Mario Kart Wii races and return rooms to track selection.", keys: ["manage_rooms_end_game"] },
    ],
  },
  {
    title: "Visibility",
    items: [
      { id: "view_users", label: "View online users in the moderation panel.", keys: ["view_users"] },
      { id: "view_bans", label: "View the ban list in the moderation panel.", keys: ["view_bans"] },
      { id: "view_reports", label: "View player reports in the moderation panel.", keys: ["view_reports"] },
    ],
  },
  {
    title: "Logs access",
    items: [{ id: "view_logs", label: "View moderator logs.", keys: ["view_logs"], logsToggle: true }],
  },
  {
    title: "Mario Kart Wii",
    items: [{ id: "manage_ghosts", label: "Manage ghosts and rankings.", keys: ["manage_ghosts"] }],
  },
  {
    title: "Games",
    items: [{ id: "manage_games", label: "Disable and re-enable games on the service.", keys: ["manage_games"] }],
  },
  {
    title: "Miscellaneous",
    items: [
      {
        id: "open_host",
        label: "Open Host: auto-friend strangers so they can join hosted matches without manual approval.",
        keys: ["open_host"],
      },
    ],
  },
]

export const LOG_TYPES = [
  { key: "ban", label: "Bans", short: "BAN" },
  { key: "unban", label: "Unbans", short: "UNBAN" },
  { key: "kick", label: "Kicks", short: "KICK" },
  { key: "message", label: "Login messages", short: "MSG" },
  { key: "match", label: "Match logs", short: "MATCH" },
  { key: "security", label: "Security alerts", short: "SEC" },
  { key: "network", label: "Network events", short: "NET" },
  { key: "ghost", label: "Ghost actions", short: "GHOST" },
  { key: "session", label: "Session events", short: "SESSION" },
  { key: "panel", label: "Panel actions", short: "PANEL" },
  { key: "error", label: "Errors", short: "ERROR" },
] as const

export const ROOM_RIGHTS: RightKey[] = ["manage_rooms_create", "manage_rooms_join", "manage_rooms_delete", "manage_rooms_end_game"]

export function can(account: Account | undefined, right: RightKey): boolean {
  if (!account) return false
  if (account.role === "owner") return true
  return !!account.rights?.[right]
}

export function canAny(account: Account | undefined, rights: RightKey[]): boolean {
  return rights.some((right) => can(account, right))
}

export const EMPTY_RIGHTS: Rights = {
  view_users: false,
  view_bans: false,
  ban: false,
  unban: false,
  kick: false,
  message: false,
  manage_rooms_create: false,
  manage_rooms_join: false,
  manage_rooms_delete: false,
  manage_rooms_end_game: false,
  view_logs: false,
  view_reports: false,
  manage_ghosts: false,
  manage_games: false,
  manage_rights: false,
  private_room_kick: false,
  report_players: false,
  open_host: false,
  log_types: [],
}

export function rightsSummary(account: Account): string[] {
  const rights = account.rights ?? EMPTY_RIGHTS
  const lines: string[] = []
  for (const group of RIGHT_GROUPS) {
    for (const item of group.items) {
      if (item.keys.some((key) => rights[key])) lines.push(item.label)
    }
  }
  if (rights.view_logs && rights.log_types?.length) {
    const labels = LOG_TYPES.filter((t) => rights.log_types.includes(t.key)).map((t) => t.label)
    if (labels.length) lines.push("Visible log types: " + labels.join(", "))
  }
  return lines
}
