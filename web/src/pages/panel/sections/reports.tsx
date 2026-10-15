import { Ban, CheckCheck, Flag, Inbox, LogIn, ScrollText, UserMinus, UserRound } from "lucide-react"
import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"

import { useConfirm } from "@/components/confirm"
import { SearchInput } from "@/components/filters"
import { StatusBadge } from "@/components/status-badge"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { api, errorMessage, list, type ReportDetail, type ReportSummary } from "@/lib/api"
import { fmtDate, initials, relativeTime } from "@/lib/format"
import { cn } from "@/lib/utils"
import { useLiveArea, usePanel } from "@/pages/panel/panel-context"

const label = (r: ReportSummary) => r.name || r.fc || `PID ${r.pid}`

export function ReportsSection() {
  const { can, openJoinDialog, openPlayer, openAction, showActivityFor } = usePanel()
  const { confirm } = useConfirm()
  const [reports, setReports] = useState<ReportSummary[] | null>(null)
  const [selected, setSelected] = useState<number | null>(null)
  const [details, setDetails] = useState<ReportDetail[] | null>(null)
  const [query, setQuery] = useState("")

  const load = useCallback(async () => {
    try {
      const rows = list(await api<ReportSummary[]>("/api/panel/reports")).sort(
        (a, b) => b.reports - a.reports || new Date(b.last_reported).getTime() - new Date(a.last_reported).getTime(),
      )
      setReports(rows)
      setSelected((current) => (current && rows.some((r) => r.pid === current) ? current : (rows[0]?.pid ?? null)))
    } catch (error) {
      toast.error(errorMessage(error))
      setReports([])
    }
  }, [])
  useEffect(() => {
    void load()
  }, [load])
  useLiveArea("reports", () => void load())

  useEffect(() => {
    setDetails(null)
    if (!selected) return
    api<ReportDetail[]>(`/api/panel/reports/details?pid=${selected}`)
      .then((rows) => setDetails(list(rows).sort((a, b) => new Date(b.created).getTime() - new Date(a.created).getTime())))
      .catch((error) => {
        toast.error(errorMessage(error))
        setDetails([])
      })
  }, [selected])

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    return (reports ?? []).filter((r) => !q || [r.name, r.fc, r.pid].some((v) => String(v ?? "").toLowerCase().includes(q)))
  }, [reports, query])
  const current = reports?.find((r) => r.pid === selected) ?? null

  async function dismiss(report: ReportSummary) {
    const ok = await confirm({ title: `Dismiss all reports for ${label(report)}?`, description: "The reports are deleted. Logs keep a record.", confirmLabel: "Dismiss" })
    if (!ok) return
    try {
      const response = await api<{ removed?: number }>(`/api/panel/reports?pid=${encodeURIComponent(report.pid)}`, { method: "DELETE" })
      toast.success(`Dismissed ${response.removed || 0} report${response.removed === 1 ? "" : "s"}`)
      void load()
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  if (reports && !reports.length) {
    return (
      <Empty className="border py-24">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <Inbox />
          </EmptyMedia>
          <EmptyTitle>Inbox zero</EmptyTitle>
          <EmptyDescription>No player reports waiting for review.</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  const ref = current ? { pid: current.pid, fc: current.fc, name: current.name } : undefined

  return (
    <div className="grid gap-4 lg:grid-cols-[340px_1fr]">
      <Card className="gap-0 py-0">
        <CardHeader className="border-b p-3">
          <SearchInput value={query} onChange={setQuery} placeholder="Search reported players" className="sm:w-full" />
        </CardHeader>
        <ScrollArea className="lg:h-[calc(100svh-14rem)]">
          <div className="grid gap-0.5 p-1.5">
            {!reports
              ? Array.from({ length: 5 }).map((_, i) => <Skeleton key={i} className="h-14" />)
              : visible.map((r) => (
                  <button
                    key={r.pid}
                    type="button"
                    onClick={() => setSelected(r.pid)}
                    className={cn("flex items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-muted/60", selected === r.pid && "bg-muted")}
                  >
                    <div className="relative">
                      <Avatar className="size-9">
                        <AvatarFallback className="text-xs">{initials(label(r))}</AvatarFallback>
                      </Avatar>
                      {r.online && <span className="absolute right-0 bottom-0 size-2.5 rounded-full border-2 border-card bg-success" />}
                    </div>
                    <div className="grid min-w-0 flex-1">
                      <span className="truncate text-sm font-medium">{label(r)}</span>
                      <span className="truncate text-xs text-muted-foreground">
                        {r.moderator_reports ? `${r.moderator_reports} from moderators · ` : ""}
                        {relativeTime(r.last_reported)}
                      </span>
                    </div>
                    <Badge variant="secondary" className="tabular-nums">
                      {r.reports}
                    </Badge>
                  </button>
                ))}
          </div>
        </ScrollArea>
      </Card>

      <Card className="min-h-[420px]">
        {current ? (
          <>
            <CardHeader className="gap-4">
              <div className="flex flex-wrap items-center gap-4">
                <Avatar className="size-12">
                  <AvatarFallback>{initials(label(current))}</AvatarFallback>
                </Avatar>
                <div className="grid min-w-0 flex-1 gap-1">
                  <h2 className="truncate text-xl font-semibold">{label(current)}</h2>
                  <div className="flex flex-wrap items-center gap-1.5">
                    <StatusBadge tone={current.online ? "success" : "neutral"}>{current.online ? "Online" : "Offline"}</StatusBadge>
                    {current.room_id && <Badge variant="outline" className="font-mono">{current.room_id}</Badge>}
                    <span className="font-mono text-xs text-muted-foreground">{[current.fc && `FC ${current.fc}`, `PID ${current.pid}`].filter(Boolean).join(" · ")}</span>
                  </div>
                </div>
                <div className="text-right">
                  <p className="text-3xl font-semibold tabular-nums">{current.reports}</p>
                  <p className="text-xs text-muted-foreground">reports</p>
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button size="sm" variant="outline" onClick={() => openPlayer(ref!)}>
                  <UserRound data-icon="inline-start" /> Profile
                </Button>
                {can("ban") && (
                  <Button size="sm" variant="destructive" onClick={() => openAction("ban", ref)}>
                    <Ban data-icon="inline-start" /> Ban
                  </Button>
                )}
                {can("kick") && current.online && (
                  <Button size="sm" variant="outline" onClick={() => openAction("kick", ref)}>
                    <UserMinus data-icon="inline-start" /> Kick
                  </Button>
                )}
                {current.online && current.room_id && can("manage_rooms_join") && (
                  <Button size="sm" variant="outline" onClick={() => openJoinDialog(current.room_id!)}>
                    <LogIn data-icon="inline-start" /> Join their room
                  </Button>
                )}
                {can("view_logs") && (
                  <Button size="sm" variant="ghost" onClick={() => showActivityFor(String(current.pid))}>
                    <ScrollText data-icon="inline-start" /> Activity
                  </Button>
                )}
                <Button size="sm" variant="secondary" className="ml-auto" onClick={() => void dismiss(current)}>
                  <CheckCheck data-icon="inline-start" /> Dismiss all
                </Button>
              </div>
            </CardHeader>
            <Separator />
            <CardContent className="grid gap-3">
              {!details ? (
                <Skeleton className="h-32" />
              ) : (
                details.map((d) => (
                  <div key={d.id} className="grid gap-2 rounded-lg border p-4">
                    <div className="flex flex-wrap items-center gap-2 text-sm">
                      <Flag className="size-4 text-warning" />
                      <span className="font-medium">{d.reporter_name || d.reporter_fc || `PID ${d.reporter_pid}`}</span>
                      {d.reporter_type === "moderator" && <StatusBadge tone="warning">moderator</StatusBadge>}
                      <span className="ml-auto text-xs text-muted-foreground" title={fmtDate(d.created)}>
                        {relativeTime(d.created)}
                      </span>
                    </div>
                    <p className="text-sm leading-relaxed">{d.reason}</p>
                    <p className="font-mono text-xs text-muted-foreground">{[d.room_id && `room ${d.room_id}`, d.type, d.game].filter(Boolean).join(" · ")}</p>
                  </div>
                ))
              )}
            </CardContent>
          </>
        ) : (
          <Empty>
            <EmptyHeader>
              <EmptyTitle>Select a player</EmptyTitle>
              <EmptyDescription>Pick a reported player from the list.</EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}
      </Card>
    </div>
  )
}
