import { Ban, Check, Copy, DoorOpen, Flag, Gamepad2, LogIn, MessageSquare, ScrollText, ShieldOff, Undo2, UserMinus } from "lucide-react"
import { useEffect, useMemo, useState } from "react"

import { StatusBadge, logLevelTone, remainingTone, type Tone } from "@/components/status-badge"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemTitle } from "@/components/ui/item"
import { Separator } from "@/components/ui/separator"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { api, list, type BanRecord, type LogEntry, type ReportDetail, type ReportSummary, type Room } from "@/lib/api"
import { banStatus, fmtDate, fmtLogTime, initials, isKickRecord, moderationKind, playerStateLabel, relativeTime, remainingBanTime, roomStateLabel, roomType } from "@/lib/format"
import { cn } from "@/lib/utils"
import { useUnban } from "@/pages/panel/sections/bans"
import { playerLabel, usePanel, type PlayerRef } from "@/pages/panel/panel-context"

function CopyValue({ label, value }: { label: string; value?: string | number }) {
  const [copied, setCopied] = useState(false)
  if (!value) return null
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1.5 rounded-md border bg-muted/40 px-2 py-1 font-mono text-xs hover:bg-muted"
          onClick={() => {
            void navigator.clipboard?.writeText(String(value))
            setCopied(true)
            window.setTimeout(() => setCopied(false), 1200)
          }}
        >
          <span className="text-muted-foreground">{label}</span> {value}
          {copied ? <Check className="size-3 text-success" /> : <Copy className="size-3 opacity-50" />}
        </button>
      </TooltipTrigger>
      <TooltipContent>Copy {label}</TooltipContent>
    </Tooltip>
  )
}

const samePlayer = (ref: PlayerRef, pid?: number, fc?: string) =>
  (!!ref.pid && !!pid && String(ref.pid) === String(pid)) || (!!ref.fc && !!fc && ref.fc === fc)

function useResolvedPlayer(ref: PlayerRef | null) {
  const { dashboard } = usePanel()
  return useMemo(() => {
    if (!ref) return null
    const online = dashboard.users.find((u) => samePlayer(ref, u.pid, u.fc))
    let room: Room | undefined
    let state: string | undefined
    for (const r of dashboard.rooms) {
      const hit = Object.values(r.players || {}).find((p) => samePlayer(ref, p.pid, p.fc))
      if (hit) {
        room = r
        state = hit.state
        break
      }
    }
    const roomPlayer = room && Object.values(room.players).find((p) => samePlayer(ref, p.pid, p.fc))
    const bans = dashboard.bans.filter((b) => samePlayer(ref, b.pid, b.fc))
    const pid = ref.pid || online?.pid || roomPlayer?.pid || bans.find((b) => b.pid)?.pid
    const fc = ref.fc || online?.fc || roomPlayer?.fc || bans.find((b) => b.fc)?.fc
    const name = ref.name || online?.name || roomPlayer?.name || bans.find((b) => b.name)?.name
    const activeBan = bans.find((b) => banStatus(b) === "active" && !isKickRecord(b))
    return { pid, fc, name, online, room, state, bans, activeBan }
  }, [ref, dashboard])
}

function BanList({ bans }: { bans: BanRecord[] }) {
  if (!bans.length) return <p className="py-6 text-center text-sm text-muted-foreground">No bans or kicks on record.</p>
  return (
    <ItemGroup className="gap-2">
      {bans.map((ban, i) => {
        const remaining = remainingBanTime(ban)
        return (
          <Item key={`${ban.issued}-${i}`} variant="outline" size="sm" className="flex-col items-stretch gap-1">
            <div className="flex items-center gap-2">
              <StatusBadge tone={isKickRecord(ban) ? "warning" : "danger"}>{moderationKind(ban)}</StatusBadge>
              <span className="text-xs text-muted-foreground">{fmtDate(ban.issued, "-")}</span>
              <StatusBadge tone={remaining === "Revoked" || remaining === "Expired" ? "neutral" : remainingTone(remaining)} className="ml-auto">
                {remaining}
              </StatusBadge>
            </div>
            <p className="text-sm">{ban.reason || "-"}</p>
            {ban.reason_hidden && <p className="text-xs text-muted-foreground">Note: {ban.reason_hidden}</p>}
            {ban.moderator && <p className="text-xs text-muted-foreground">by {ban.moderator}</p>}
          </Item>
        )
      })}
    </ItemGroup>
  )
}

const dotClass: Record<Tone, string> = {
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-destructive",
  info: "bg-chart-1",
  neutral: "bg-muted-foreground",
}

export function PlayerSheet() {
  const { player, closePlayer, can, openAction, openJoinDialog, openCreateRoom, openRoom, showActivityFor } = usePanel()
  const resolved = useResolvedPlayer(player)
  const unban = useUnban()
  const [reports, setReports] = useState<{ summary?: ReportSummary; details: ReportDetail[] } | null>(null)
  const [logs, setLogs] = useState<LogEntry[] | null>(null)
  const pid = resolved?.pid
  const fc = resolved?.fc

  useEffect(() => {
    setReports(null)
    setLogs(null)
    if (!player) return
    if (can("view_reports") && pid) {
      Promise.all([api<ReportSummary[]>("/api/panel/reports"), api<ReportDetail[]>(`/api/panel/reports/details?pid=${pid}`)])
        .then(([summaries, details]) => setReports({ summary: list(summaries).find((s) => s.pid === pid), details: list(details) }))
        .catch(() => setReports({ details: [] }))
    }
    if (can("view_logs")) {
      api<LogEntry[]>("/api/panel/logs")
        .then((rows) =>
          setLogs(
            list(rows)
              .filter((l) => (pid && l.target_pid === pid) || (fc && l.target_fc === fc))
              .sort((a, b) => new Date(b.created).getTime() - new Date(a.created).getTime())
              .slice(0, 40),
          ),
        )
        .catch(() => setLogs([]))
    }
  }, [player, pid, fc, can])

  const identifier = pid ? String(pid) : fc || ""
  const ref: PlayerRef = { pid, fc, name: resolved?.name }

  return (
    <Dialog open={!!player} onOpenChange={(open) => !open && closePlayer()}>
      <DialogContent className="flex max-h-[88vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
        {resolved && (
          <>
            <DialogHeader className="gap-3 border-b p-6 pr-12">
              <div className="flex items-center gap-4">
                <div className="relative">
                  <Avatar className="size-14 text-lg">
                    <AvatarFallback>{initials(playerLabel(ref))}</AvatarFallback>
                  </Avatar>
                  <span className={`absolute right-0 bottom-0 size-3.5 rounded-full border-2 border-background ${resolved.online ? "bg-success" : "bg-muted-foreground/40"}`} />
                </div>
                <div className="grid min-w-0 gap-1">
                  <DialogTitle className="truncate text-xl">{playerLabel(ref)}</DialogTitle>
                  <DialogDescription className="flex flex-wrap items-center gap-1.5">
                    <StatusBadge tone={resolved.online ? "success" : "neutral"}>{resolved.online ? "Online" : "Offline"}</StatusBadge>
                    {resolved.online?.restricted && <StatusBadge tone="danger">Restricted</StatusBadge>}
                    {resolved.activeBan && <StatusBadge tone="danger">Banned · {remainingBanTime(resolved.activeBan)}</StatusBadge>}
                    {resolved.online && <Badge variant="secondary">{resolved.online.game || resolved.online.game_code}</Badge>}
                  </DialogDescription>
                </div>
              </div>
              <div className="flex flex-wrap gap-1.5">
                <CopyValue label="FC" value={fc} />
                <CopyValue label="PID" value={pid} />
              </div>
              <div className="flex flex-wrap gap-2">
                {can("ban") && !resolved.activeBan && (
                  <Button size="sm" variant="destructive" onClick={() => openAction("ban", ref)}>
                    <Ban data-icon="inline-start" /> Ban
                  </Button>
                )}
                {can("unban") && resolved.activeBan && (
                  <Button size="sm" variant="outline" onClick={() => void unban(identifier)}>
                    <Undo2 data-icon="inline-start" /> Unban
                  </Button>
                )}
                {can("kick") && resolved.online && (
                  <Button size="sm" variant="outline" onClick={() => openAction("kick", ref)}>
                    <UserMinus data-icon="inline-start" /> Kick
                  </Button>
                )}
                {can("message") && (
                  <Button size="sm" variant="outline" onClick={() => openAction("message", ref)}>
                    <MessageSquare data-icon="inline-start" /> Message
                  </Button>
                )}
                {resolved.online && can("manage_rooms_create") && !resolved.room && (
                  <Button size="sm" variant="outline" onClick={() => openCreateRoom(identifier)}>
                    <DoorOpen data-icon="inline-start" /> Host a room
                  </Button>
                )}
              </div>
            </DialogHeader>
            <Tabs defaultValue="overview" className="min-h-0 flex-1 gap-0">
              <TabsList variant="line" className="w-full justify-start gap-2 border-b px-6">
                <TabsTrigger value="overview">Overview</TabsTrigger>
                <TabsTrigger value="bans">
                  Bans {resolved.bans.length > 0 && <Badge variant="secondary">{resolved.bans.length}</Badge>}
                </TabsTrigger>
                {can("view_reports") && (
                  <TabsTrigger value="reports">
                    Reports {reports?.details.length ? <Badge variant="secondary">{reports.details.length}</Badge> : null}
                  </TabsTrigger>
                )}
                {can("view_logs") && <TabsTrigger value="activity">Activity</TabsTrigger>}
              </TabsList>
              <div className="scrollbar-thin min-h-0 flex-1 overflow-y-auto overscroll-contain">
                <div className="p-6">
                  <TabsContent value="overview" className="grid gap-4">
                    <section className="grid gap-2">
                      <h3 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">Current room</h3>
                      {resolved.room ? (
                        <Item variant="outline" asChild>
                          <button type="button" className="text-left" onClick={() => openRoom(resolved.room!.id)}>
                            <ItemContent>
                              <ItemTitle className="font-mono">{resolved.room.id}</ItemTitle>
                              <ItemDescription>
                                {resolved.room.game} · {roomType(resolved.room)} · {roomStateLabel(resolved.room.state)} · {playerStateLabel(resolved.state)}
                              </ItemDescription>
                            </ItemContent>
                            <ItemActions>
                              {roomType(resolved.room) !== "private" && can("manage_rooms_join") && (
                                <Button
                                  size="sm"
                                  variant="secondary"
                                  onClick={(e) => {
                                    e.stopPropagation()
                                    openJoinDialog(resolved.room!.id)
                                  }}
                                >
                                  <LogIn data-icon="inline-start" /> Join
                                </Button>
                              )}
                            </ItemActions>
                          </button>
                        </Item>
                      ) : (
                        <p className="text-sm text-muted-foreground">{resolved.online ? "Online, not in a room." : "Not in a room."}</p>
                      )}
                    </section>
                    <Separator />
                    <section className="grid grid-cols-3 gap-3 text-center">
                      <div className="rounded-lg border p-3">
                        <p className="text-2xl font-semibold tabular-nums">{resolved.bans.filter((b) => !isKickRecord(b)).length}</p>
                        <p className="text-xs text-muted-foreground">Bans</p>
                      </div>
                      <div className="rounded-lg border p-3">
                        <p className="text-2xl font-semibold tabular-nums">{resolved.bans.filter(isKickRecord).length}</p>
                        <p className="text-xs text-muted-foreground">Kicks</p>
                      </div>
                      <div className="rounded-lg border p-3">
                        <p className="text-2xl font-semibold tabular-nums">{can("view_reports") ? (reports ? reports.details.length : "…") : "—"}</p>
                        <p className="text-xs text-muted-foreground">Reports</p>
                      </div>
                    </section>
                    {resolved.online && (
                      <section className="grid gap-1 text-sm">
                        <p className="flex items-center gap-2">
                          <Gamepad2 className="size-4 text-muted-foreground" /> {resolved.online.game || resolved.online.game_code}
                          <span className="font-mono text-xs text-muted-foreground">{resolved.online.game_code}</span>
                        </p>
                        {resolved.online.game === "mariokartwii" && (
                          <p className="flex items-center gap-2 text-muted-foreground">
                            <ShieldOff className="size-4" /> {resolved.online.payload_features ? "Payload features active" : "No payload"}
                          </p>
                        )}
                      </section>
                    )}
                  </TabsContent>
                  <TabsContent value="bans">
                    <BanList bans={resolved.bans} />
                  </TabsContent>
                  <TabsContent value="reports" className="grid gap-2">
                    {!reports ? (
                      <Skeleton className="h-24" />
                    ) : reports.details.length ? (
                      reports.details.map((r) => (
                        <Item key={r.id} variant="outline" size="sm" className="flex-col items-stretch gap-1">
                          <div className="flex items-center gap-2 text-xs">
                            <Flag className="size-3.5 text-warning" />
                            <span className="font-medium">{r.reporter_name || r.reporter_fc || `PID ${r.reporter_pid}`}</span>
                            {r.reporter_type === "moderator" && <StatusBadge tone="warning">moderator</StatusBadge>}
                            <span className="ml-auto text-muted-foreground">{relativeTime(r.created)}</span>
                          </div>
                          <p className="text-sm">{r.reason}</p>
                          <p className="font-mono text-xs text-muted-foreground">{[r.room_id, r.type, r.game].filter(Boolean).join(" · ")}</p>
                        </Item>
                      ))
                    ) : (
                      <p className="py-6 text-center text-sm text-muted-foreground">No reports about this player.</p>
                    )}
                  </TabsContent>
                  <TabsContent value="activity" className="grid gap-3">
                    {!logs ? (
                      <Skeleton className="h-24" />
                    ) : logs.length ? (
                      <ol className="relative grid gap-3 border-l pl-4">
                        {logs.map((log) => (
                          <li key={log.id} className="relative">
                            <span className={cn("absolute top-1.5 -left-[21px] size-2.5 rounded-full border-2 border-background", dotClass[logLevelTone(log.level)])} />
                            <p className="text-sm">{log.summary}</p>
                            <p className="font-mono text-xs text-muted-foreground">
                              {fmtLogTime(log.created)}
                              {log.moderator ? ` · ${log.moderator}` : ""}
                              {log.reason ? ` · ${log.reason}` : ""}
                            </p>
                          </li>
                        ))}
                      </ol>
                    ) : (
                      <Empty>
                        <EmptyHeader>
                          <EmptyTitle>No recent activity</EmptyTitle>
                          <EmptyDescription>Nothing logged for this player in the last 14 days.</EmptyDescription>
                        </EmptyHeader>
                      </Empty>
                    )}
                    <Button variant="outline" size="sm" className="w-fit" onClick={() => showActivityFor(identifier)}>
                      <ScrollText data-icon="inline-start" /> Open in activity console
                    </Button>
                  </TabsContent>
                </div>
              </div>
            </Tabs>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
