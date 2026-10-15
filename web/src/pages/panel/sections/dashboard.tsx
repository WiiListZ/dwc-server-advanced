import { Activity, ArrowRight, DoorOpen, Flag, Gavel, Users, type LucideIcon } from "lucide-react"
import { useCallback, useEffect, useMemo, useState } from "react"
import { Area, AreaChart } from "recharts"

import { MatchesPerHourChart, OnlineHistoryChart, PlayersByGameChart, type OnlineSample } from "@/components/charts"
import { StatusBadge, logLevelTone } from "@/components/status-badge"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ChartContainer } from "@/components/ui/chart"
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemMedia, ItemTitle } from "@/components/ui/item"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { api, list, type LogEntry, type MatchLog, type ReportSummary } from "@/lib/api"
import { banStatus, fmtLogTime, initials, isKickRecord, relativeTime } from "@/lib/format"
import { ROOM_RIGHTS } from "@/lib/rights"
import { useLiveArea, usePanel } from "@/pages/panel/panel-context"
import { RoomTile } from "@/pages/panel/room-views"

function Sparkline({ samples, dataKey, color }: { samples: OnlineSample[]; dataKey: keyof OnlineSample; color: string }) {
  if (samples.length < 2) return <div className="h-10" />
  return (
    <ChartContainer config={{ [dataKey]: { label: String(dataKey), color } }} className="aspect-auto h-10 w-full">
      <AreaChart data={samples} margin={{ top: 2, bottom: 0, left: 0, right: 0 }}>
        <Area dataKey={dataKey} type="monotone" stroke={color} strokeWidth={1.5} fill={color} fillOpacity={0.15} isAnimationActive={false} />
      </AreaChart>
    </ChartContainer>
  )
}

function Kpi({
  label,
  value,
  icon: Icon,
  loading,
  hint,
  spark,
  onClick,
}: {
  label: string
  value: number | string
  icon: LucideIcon
  loading?: boolean
  hint?: string
  spark?: React.ReactNode
  onClick?: () => void
}) {
  return (
    <Card
      className={`gap-2 overflow-hidden pb-0 ${onClick ? "cursor-pointer transition-colors hover:bg-muted/30" : ""}`}
      onClick={onClick}
      role={onClick ? "button" : undefined}
      tabIndex={onClick ? 0 : undefined}
    >
      <CardHeader>
        <CardDescription className="flex items-center gap-1.5">
          <Icon className="size-3.5" /> {label}
        </CardDescription>
        <CardTitle className="text-3xl font-semibold tabular-nums">{loading ? <Skeleton className="h-9 w-14" /> : value}</CardTitle>
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      </CardHeader>
      <div className="mt-auto">{spark ?? <div className="h-10" />}</div>
    </Card>
  )
}

function hourlyBuckets(matches: MatchLog[]) {
  const now = new Date()
  now.setMinutes(0, 0, 0)
  const buckets = Array.from({ length: 24 }, (_, i) => {
    const start = new Date(now.getTime() - (23 - i) * 3600_000)
    return { start: start.getTime(), hour: start.toLocaleTimeString([], { hour: "2-digit" }), matches: 0 }
  })
  for (const match of matches) {
    const t = new Date(match.created).getTime()
    const bucket = buckets.find((b) => t >= b.start && t < b.start + 3600_000)
    if (bucket) bucket.matches += 1
  }
  return buckets.map(({ hour, matches }) => ({ hour, matches }))
}

function greeting() {
  const h = new Date().getHours()
  return h < 5 ? "Good night" : h < 12 ? "Good morning" : h < 18 ? "Good afternoon" : "Good evening"
}

export function DashboardSection() {
  const { account, dashboard, dashboardLoading, history, can, canAny, navigate, openPlayer } = usePanel()
  const [feed, setFeed] = useState<LogEntry[] | null>(null)
  const [matches, setMatches] = useState<MatchLog[] | null>(null)
  const [reports, setReports] = useState<ReportSummary[] | null>(null)
  const global = dashboard.stats.global ?? { online: 0, active: 0, groups: 0 }
  const activeBans = dashboard.bans.filter((b) => banStatus(b) === "active" && !isKickRecord(b)).length

  const loadLogs = useCallback(() => {
    if (!can("view_logs")) return
    const from = new Date(Date.now() - 24 * 3600_000).toISOString()
    api<LogEntry[]>(`/api/panel/logs?from=${encodeURIComponent(from)}&to=${encodeURIComponent(new Date(Date.now() + 60_000).toISOString())}`)
      .then((rows) => setFeed(list(rows).sort((a, b) => new Date(b.created).getTime() - new Date(a.created).getTime()).slice(0, 25)))
      .catch(() => setFeed([]))
    api<MatchLog[]>("/api/panel/matches?limit=500")
      .then((rows) => setMatches(list(rows)))
      .catch(() => setMatches([]))
  }, [can])
  const loadReports = useCallback(() => {
    if (!can("view_reports")) return
    api<ReportSummary[]>("/api/panel/reports")
      .then((rows) => setReports(list(rows).sort((a, b) => b.reports - a.reports)))
      .catch(() => setReports([]))
  }, [can])
  useEffect(loadLogs, [loadLogs])
  useEffect(loadReports, [loadReports])
  useLiveArea("logs", loadLogs)
  useLiveArea("reports", loadReports)

  const buckets = useMemo(() => hourlyBuckets(matches ?? []), [matches])

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="text-sm text-muted-foreground">
            {greeting()}, {account.username}
          </p>
          <h1 className="text-3xl font-semibold tracking-tight">
            {global.online} player{global.online === 1 ? "" : "s"} online right now
          </h1>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
        <Kpi label="Online" value={global.online} icon={Users} loading={dashboardLoading} spark={<Sparkline samples={history} dataKey="online" color="var(--chart-1)" />} onClick={can("view_users") ? () => navigate("live") : undefined} />
        <Kpi label="In a match" value={global.active} icon={Activity} loading={dashboardLoading} spark={<Sparkline samples={history} dataKey="active" color="var(--chart-2)" />} />
        <Kpi label="Rooms" value={global.groups} icon={DoorOpen} loading={dashboardLoading} spark={<Sparkline samples={history} dataKey="rooms" color="var(--chart-3)" />} onClick={canAny(ROOM_RIGHTS) ? () => navigate("live") : undefined} />
        <Kpi label="Active bans" value={can("view_bans") ? activeBans : "—"} icon={Gavel} loading={dashboardLoading} hint={can("view_bans") ? `${dashboard.bans.length} records` : "Needs view bans"} onClick={can("view_bans") ? () => navigate("bans") : undefined} />
        <Kpi label="Reported players" value={can("view_reports") ? (reports ? reports.length : "…") : "—"} icon={Flag} hint={can("view_reports") ? "Waiting for review" : "Needs view reports"} onClick={can("view_reports") ? () => navigate("reports") : undefined} />
      </div>

      <div className="grid gap-4 xl:grid-cols-3">
        <Card className="xl:col-span-2">
          <CardHeader>
            <CardTitle>Network activity</CardTitle>
            <CardDescription>Sampled every 15 seconds while the panel is open.</CardDescription>
          </CardHeader>
          <CardContent>
            {history.length < 2 ? (
              <div className="flex h-[220px] flex-col items-center justify-center gap-2 rounded-lg border border-dashed text-sm text-muted-foreground">
                <Spinner /> Collecting samples…
              </div>
            ) : (
              <OnlineHistoryChart samples={history} />
            )}
          </CardContent>
        </Card>
        <Card className="gap-3">
          <CardHeader>
            <CardTitle>Live feed</CardTitle>
            <CardDescription>Latest events from the last 24 hours.</CardDescription>
            {can("view_logs") && (
              <CardAction>
                <Button variant="ghost" size="sm" onClick={() => navigate("activity")}>
                  Open <ArrowRight data-icon="inline-end" />
                </Button>
              </CardAction>
            )}
          </CardHeader>
          <CardContent className="px-2">
            {!can("view_logs") ? (
              <p className="px-4 text-sm text-muted-foreground">Needs the view logs right.</p>
            ) : feed === null ? (
              <Skeleton className="mx-4 h-60" />
            ) : feed.length ? (
              <ScrollArea className="h-[260px]">
                <ol className="grid gap-0.5 px-2">
                  {feed.map((log) => {
                    const target = log.target_pid || log.target_fc ? { pid: log.target_pid, fc: log.target_fc, name: log.target_name } : null
                    return (
                      <li key={log.id}>
                        <button
                          type="button"
                          disabled={!target}
                          onClick={() => target && openPlayer(target)}
                          className="flex w-full items-start gap-2.5 rounded-md px-2 py-1.5 text-left hover:bg-muted/50 disabled:hover:bg-transparent"
                        >
                          <StatusBadge tone={logLevelTone(log.level)} className="mt-0.5 h-5 w-16 justify-center px-1 text-[10px] uppercase">
                            {log.type}
                          </StatusBadge>
                          <span className="grid min-w-0 flex-1">
                            <span className="truncate text-sm">{log.target_name ? `${log.target_name}: ${log.summary}` : log.summary}</span>
                            <span className="font-mono text-[11px] text-muted-foreground">{fmtLogTime(log.created).slice(11)}</span>
                          </span>
                        </button>
                      </li>
                    )
                  })}
                </ol>
              </ScrollArea>
            ) : (
              <Empty className="h-[260px]">
                <EmptyHeader>
                  <EmptyTitle>All quiet</EmptyTitle>
                  <EmptyDescription>No events in the last 24 hours.</EmptyDescription>
                </EmptyHeader>
              </Empty>
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 xl:grid-cols-3">
        {can("view_reports") && (
          <Card>
            <CardHeader>
              <CardTitle>Needs attention</CardTitle>
              <CardDescription>Most reported players.</CardDescription>
              <CardAction>
                <Button variant="ghost" size="sm" onClick={() => navigate("reports")}>
                  Inbox <ArrowRight data-icon="inline-end" />
                </Button>
              </CardAction>
            </CardHeader>
            <CardContent>
              {!reports ? (
                <Skeleton className="h-40" />
              ) : reports.length ? (
                <ItemGroup className="gap-1.5">
                  {reports.slice(0, 5).map((r) => (
                    <Item key={r.pid} variant="outline" size="sm" asChild>
                      <button type="button" className="text-left" onClick={() => openPlayer({ pid: r.pid, fc: r.fc, name: r.name })}>
                        <ItemMedia>
                          <Avatar className="size-8">
                            <AvatarFallback className="text-[11px]">{initials(r.name || "?")}</AvatarFallback>
                          </Avatar>
                        </ItemMedia>
                        <ItemContent className="min-w-0">
                          <ItemTitle className="truncate">{r.name || r.fc || `PID ${r.pid}`}</ItemTitle>
                          <ItemDescription>last {relativeTime(r.last_reported)}</ItemDescription>
                        </ItemContent>
                        <ItemActions>
                          {r.online && <StatusBadge tone="success">online</StatusBadge>}
                          <Badge variant="secondary">{r.reports}</Badge>
                        </ItemActions>
                      </button>
                    </Item>
                  ))}
                </ItemGroup>
              ) : (
                <Empty className="py-8">
                  <EmptyHeader>
                    <EmptyTitle>Inbox zero</EmptyTitle>
                    <EmptyDescription>No open player reports.</EmptyDescription>
                  </EmptyHeader>
                </Empty>
              )}
            </CardContent>
          </Card>
        )}
        <Card>
          <CardHeader>
            <CardTitle>Players by game</CardTitle>
            <CardDescription>Current connections per game.</CardDescription>
          </CardHeader>
          <CardContent>
            <PlayersByGameChart stats={dashboard.stats} />
          </CardContent>
        </Card>
        {can("view_logs") && (
          <Card>
            <CardHeader>
              <CardTitle>Matches · 24h</CardTitle>
              <CardDescription>From the match log, per hour.</CardDescription>
            </CardHeader>
            <CardContent>{matches === null ? <Skeleton className="h-[220px]" /> : <MatchesPerHourChart buckets={buckets} />}</CardContent>
          </Card>
        )}
      </div>

      {canAny(ROOM_RIGHTS) && dashboard.rooms.length > 0 && (
        <section className="grid gap-3">
          <div className="flex items-center justify-between">
            <h2 className="font-semibold">Open rooms</h2>
            <Button variant="ghost" size="sm" onClick={() => navigate("live")}>
              All rooms <ArrowRight data-icon="inline-end" />
            </Button>
          </div>
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            {dashboard.rooms.slice(0, 8).map((room) => (
              <RoomTile key={room.id} room={room} />
            ))}
          </div>
        </section>
      )}
    </div>
  )
}
