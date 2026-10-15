export type WebhookSettings = { enabled: boolean; url: string; author: string; eventTypes: string[] }

export type ServerSettings = {
  serverName: string
  gsAddress: string
  nasAddress: string
  nasPort: string
  nasAddressHttps: string
  nasPortHttps: string
  payloadServerAddress: string
  enableHttps: boolean
  certPath: string
  keyPath: string
  enableHttpsExploitWii: boolean
  certDerPathWii: string
  keyPathWii: string
  enableHttpsExploitDS: boolean
  certDerPathDS: string
  wiiCertDerPathDS: string
  keyPathDS: string
  frontendAddress: string
  frontendBackendAddress: string
  backendAddress: string
  backendFrontendAddress: string
  databaseAddress: string
  databaseName: string
  username: string
  password?: string
  hasPassword: boolean
  allowDefaultDolphinKeys: boolean
  allowNoSSLCheatClients: boolean
  allowConnectWithoutDeviceID: boolean
  allowMultipleDeviceIDs: string
  logLevel: number
  logOutput: string
  keepRevokedBans: boolean
  banHistoryDays: number
  profilesEnabled: boolean
  updatesEnabled: boolean
  updatesRepository: string
  updatesCheckIntervalMinutes: number
  logEventsToDatabase: boolean
  webhooks: WebhookSettings[]
  hasApiSecret: boolean
  regenerateApiSecret?: boolean
}

export const LOG_LEVELS = [
  { value: 0, label: "0 · Nothing" },
  { value: 1, label: "1 · General" },
  { value: 2, label: "2 · + Errors" },
  { value: 3, label: "3 · + Warnings" },
  { value: 4, label: "4 · + Info (most detail)" },
]

export const LOG_OUTPUTS = [
  { value: "StdOutAndFile", label: "Console and file" },
  { value: "StdOut", label: "Console only" },
  { value: "None", label: "Nothing" },
]

export const DEVICE_ID_MODES = [
  { value: "never", label: "Never" },
  { value: "SameIPAddress", label: "Only from the same IP address" },
  { value: "always", label: "Always" },
]

export const WEBHOOK_EVENTS = [
  "all",
  "backend_started",
  "backend_stopped",
  "profile_created",
  "logged_in",
  "logged_out",
  "profile_kicked",
  "profile_banned",
  "profile_unbanned",
  "gpcm_returned_error",
  "group_created",
  "group_deleted",
  "natneg_failed",
  "reported_bad_packet",
  "room_player_reported",
  "disabled_game_added",
  "disabled_game_removed",
  "server_updated",
]
