import {
  CircleAlert,
  DoorOpen,
  Flag,
  Gavel,
  Ghost,
  Inbox,
  LayoutDashboard,
  LogIn,
  MessageSquare,
  Network,
  Radio,
  ShieldAlert,
  Undo2,
  UserMinus,
  UserRound,
  type LucideIcon,
} from "lucide-react"
import { useCallback, useEffect, useMemo, useState } from "react"
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts"
import { toast } from "sonner"

import { SearchInput } from "@/components/filters"
import { StatusBadge, logLevelTone, type Tone } from "@/components/status-badge"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Spinner } from "@/components/ui/spinner"
import { Switch } from "@/components/ui/switch"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { api, errorMessage, list, type LogEntry } from "@/lib/api"
import { fmtDate, fmtLogTime, relativeTime } from "@/lib/format"
import { LOG_TYPES } from "@/lib/rights"
import { cn } from "@/lib/utils"
import { useLiveArea, usePanel } from "@/pages/panel/panel-context"

const STEP = 60

const TYPE_ICON: Record<string, LucideIcon> = {
  ban: Gavel,
  unban: Undo2,
  kick: UserMinus,
  message: MessageSquare,
  match: Flag,
  security: ShieldAlert,
  network: Network,
  ghost: Ghost,
  session: LogIn,
  panel: LayoutDashboard,
  error: CircleAlert,
}

const toneText: Record<Tone, string> = {
  success: "text-success",
  warning: "text-warning",
  danger: "text-destructive",
  info: "text-chart-1",
  neutral: "text-muted-foreground",
}

function dayKey(date: Date) {
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function dayLabel(key: string) {
  if (key === dayKey(new Date())) return "Today"
  if (key === dayKey(new Date(Date.now() - 86400_000))) return "Yesterday"
  const [y, m, d] = key.split("-").map(Number)
  return new Date(y, m - 1, d).toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric" })
}

function shortDay(key: string) {
  const [y, m, d] = key.split("-").map(Number)
  return new Date(y, m - 1, d).toLocaleDateString(undefined, { month: "short", day: "numeric" })
}

const DAY_OPTIONS = (() => {
  const options = [{ value: "all", label: "Last 14 days" }]
  for (let i = 0; i < 14; i++) {
    const d = new Date()
    d.setHours(0, 0, 0, 0)
    d.setDate(d.getDate() - i)
    options.push({ value: dayKey(d), label: dayLabel(dayKey(d)) })
  }
  return options
})()

function rangeFor(day: string) {
  if (day === "all") return ""
  const [y, m, d] = day.split("-").map(Number)
  const start = new Date(y, m - 1, d)
  return `?from=${encodeURIComponent(start.toISOString())}&to=${encodeURIComponent(new Date(start.getTime() + 86400_000).toISOString())}`
}

// parseExtra turns the server's "key=value | key=value" string into pairs.
function parseExtra(extra?: string): [string, string][] {
  if (!extra) return []
  return extra
    .split(" | ")
    .map((part) => {
      const i = part.indexOf("=")
      return (i > 0 ? [part.slice(0, i).replace(/_/g, " "), part.slice(i + 1)] : ["info", part]) as [string, string]
    })
    .filter(([, v]) => v !== "")
}

const targetOf = (log: LogEntry) => (log.target_pid || log.target_fc ? { pid: log.target_pid, fc: log.target_fc, name: log.target_name } : null)
const targetLabel = (log: LogEntry) => log.target_name || (log.target_pid ? `PID ${log.target_pid}` : log.target_fc || "")

function Histogram({ logs, day }: { logs: LogEntry[]; day: string }) {
  const data = useMemo(() => {
    if (day === "all") {
      return DAY_OPTIONS.slice(1)
        .reverse()
        .map((d) => ({ label: shortDay(d.value), key: d.value, events: logs.filter((l) => dayKey(new Date(l.created)) === d.value).length }))
    }
    return Array.from({ length: 24 }, (_, h) => ({
      label: String(h).padStart(2, "0"),
      key: String(h),
      events: logs.filter((l) => new Date(l.created).getHours() === h).length,
    }))
  }, [logs, day])
  return (
    <ChartContainer config={{ events: { label: "Events", color: "var(--chart-1)" } }} className="aspect-auto h-28 w-full">
      <BarChart data={data} margin={{ top: 4, left: 0, right: 0, bottom: 0 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="label" tickLine={false} axisLine={false} interval="preserveStartEnd" minTickGap={12} tick={{ fontSize: 10 }} />
        <YAxis hide allowDecimals={false} />
        <ChartTooltip cursor={false} content={<ChartTooltipContent hideIndicator />} />
        <Bar dataKey="events" fill="var(--color-events)" radius={3} isAnimationActive={false} />
      </BarChart>
    </ChartContainer>
  )
}

function EventRow({ log, onOpen }: { log: LogEntry; onOpen: () => void }) {
  const { openPlayer } = usePanel()
  const tone = logLevelTone(log.level)
  const Icon = TYPE_ICON[log.type] ?? Inbox
  const target = targetOf(log)
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => e.key === "Enter" && onOpen()}
      className="group flex cursor-pointer items-start gap-3 rounded-lg px-3 py-2.5 transition-colors hover:bg-muted/50 focus-visible:bg-muted/50 focus-visible:outline-none"
    >
      <Icon className={cn("mt-0.5 size-4 shrink-0", toneText[tone])} />
      <div className="grid min-w-0 flex-1 gap-1">
        <p className="text-sm leading-snug">
          {target && (
            <button
              type="button"
              className="mr-1.5 font-semibold hover:underline"
              onClick={(e) => {
                e.stopPropagation()
                openPlayer(target)
              }}
            >
              {targetLabel(log)}
            </button>
          )}
          <span className={cn(!target && "font-medium")}>{log.summary}</span>
        </p>
        {(log.reason || log.moderator || log.room_id) && (
          <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            {log.reason && <span className="max-w-full truncate">“{log.reason}”</span>}
            {log.moderator && (
              <Badge variant="outline" className="h-5 gap-1 px-1.5 font-normal">
                <UserRound className="size-3" /> {log.moderator}
              </Badge>
            )}
            {log.room_id && (
              <Badge variant="outline" className="h-5 gap-1 px-1.5 font-mono font-normal">
                <DoorOpen className="size-3" /> {log.room_id}
              </Badge>
            )}
          </div>
        )}
      </div>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="shrink-0 pt-0.5 text-xs text-muted-foreground tabular-nums">{fmtLogTime(log.created).slice(11, 16)}</span>
        </TooltipTrigger>
        <TooltipContent>
          {fmtDate(log.created)} · {relativeTime(log.created)}
        </TooltipContent>
      </Tooltip>
    </div>
  )
}

function EventSheet({ log, onClose }: { log: LogEntry | null; onClose: () => void }) {
  const { openPlayer, openRoom, dashboard } = usePanel()
  if (!log) return null
  const tone = logLevelTone(log.level)
  const Icon = TYPE_ICON[log.type] ?? Inbox
  const target = targetOf(log)
  const typeLabel = LOG_TYPES.find((t) => t.key === log.type)?.label ?? log.type
  const roomOpen = log.room_id && dashboard.rooms.some((r) => r.id === log.room_id)
  const fields: [string, React.ReactNode][] = [
    ["Time", `${fmtDate(log.created)} (${relativeTime(log.created)})`],
    ["Category", typeLabel],
    ["Player", target ? `${targetLabel(log)}${log.target_fc ? ` · FC ${log.target_fc}` : ""}${log.target_pid ? ` · PID ${log.target_pid}` : ""}` : null],
    ["Moderator", log.moderator],
    ["Room", log.room_id],
    ["Reason", log.reason],
    ["Note", log.note],
    ["Expires", log.expires ? fmtDate(log.expires) : null],
    ["Terms of service", log.tos ? "Yes" : null],
    ...parseExtra(log.extra).map(([k, v]) => [k.charAt(0).toUpperCase() + k.slice(1), v] as [string, string]),
  ]
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader className="gap-3">
          <Icon className={cn("size-6", toneText[tone])} />
          <DialogTitle className="text-lg leading-snug">{log.summary}</DialogTitle>
          <DialogDescription className="flex flex-wrap gap-1.5">
            <StatusBadge tone={tone}>{typeLabel}</StatusBadge>
            <Badge variant="outline">{log.level || "info"}</Badge>
          </DialogDescription>
        </DialogHeader>
        <Separator />
        <dl className="grid max-h-[50vh] gap-3 overflow-y-auto text-sm">
          {fields
            .filter(([, v]) => v)
            .map(([k, v]) => (
              <div key={k} className="grid grid-cols-[7.5rem_1fr] gap-3">
                <dt className="text-muted-foreground">{k}</dt>
                <dd className="font-medium break-words">{v}</dd>
              </div>
            ))}
        </dl>
        <DialogFooter className="flex-wrap sm:flex-row">
          {target && (
            <Button
              className="flex-1"
              onClick={() => {
                onClose()
                openPlayer(target)
              }}
            >
              <UserRound data-icon="inline-start" /> Open player
            </Button>
          )}
          {roomOpen && (
            <Button
              variant="outline"
              className="flex-1"
              onClick={() => {
                onClose()
                openRoom(log.room_id!)
              }}
            >
              <DoorOpen data-icon="inline-start" /> Open room
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function ActivitySection() {
  const { activityQuery, setActivityQuery } = usePanel()
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [day, setDay] = useState("all")
  const [type, setType] = useState("all")
  const [live, setLive] = useState(true)
  const [limit, setLimit] = useState(STEP)
  const [selected, setSelected] = useState<LogEntry | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setLogs(list(await api<LogEntry[]>(`/api/panel/logs${rangeFor(day)}`)))
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setLoading(false)
    }
  }, [day])
  useEffect(() => {
    void load()
  }, [load])
  useLiveArea("logs", () => live && void load())
  useEffect(() => setLimit(STEP), [type, activityQuery, day])

  const searched = useMemo(() => {
    const q = activityQuery.trim().toLowerCase()
    return logs
      .filter((log) => !q || [log.summary, log.moderator, log.target_name, log.target_fc, log.target_pid, log.room_id, log.reason, log.note, log.extra].some((v) => String(v ?? "").toLowerCase().includes(q)))
      .sort((a, b) => new Date(b.created).getTime() - new Date(a.created).getTime())
  }, [logs, activityQuery])

  const counts = useMemo(() => {
    const map = new Map<string, number>()
    for (const log of searched) map.set(log.type, (map.get(log.type) ?? 0) + 1)
    return map
  }, [searched])

  const rows = useMemo(() => (type === "all" ? searched : searched.filter((l) => l.type === type)), [searched, type])
  const groups = useMemo(() => {
    const out: { day: string; entries: LogEntry[] }[] = []
    for (const log of rows.slice(0, limit)) {
      const key = dayKey(new Date(log.created))
      const last = out[out.length - 1]
      if (last?.day === key) last.entries.push(log)
      else out.push({ day: key, entries: [log] })
    }
    return out
  }, [rows, limit])

  const categories = [{ key: "all", label: "All events", icon: Inbox, count: searched.length }, ...LOG_TYPES.map((t) => ({ key: t.key, label: t.label, icon: TYPE_ICON[t.key] ?? Inbox, count: counts.get(t.key) ?? 0 }))]

  return (
    <div className="grid gap-4 lg:grid-cols-[230px_1fr]">
      <aside className="grid h-fit gap-4 lg:sticky lg:top-20">
        <div className="lg:hidden">
          <Select value={type} onValueChange={setType}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {categories.map((c) => (
                <SelectItem key={c.key} value={c.key}>
                  {c.label} ({c.count})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <nav className="hidden gap-0.5 lg:grid">
          {categories.map((c) => (
            <button
              key={c.key}
              type="button"
              onClick={() => setType(c.key)}
              className={cn(
                "flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground",
                type === c.key && "bg-muted font-medium text-foreground",
                c.count === 0 && type !== c.key && "opacity-50",
              )}
            >
              <c.icon className="size-4" />
              <span className="flex-1 text-left">{c.label}</span>
              <span className="text-xs tabular-nums">{c.count}</span>
            </button>
          ))}
        </nav>
        <Separator className="hidden lg:block" />
        <div className="flex items-center justify-between gap-2 px-1">
          <Label htmlFor="activity-live" className="flex items-center gap-1.5 text-sm">
            <Radio className={cn("size-3.5", live && "animate-pulse text-success")} /> Live updates
          </Label>
          <Switch id="activity-live" checked={live} onCheckedChange={setLive} />
        </div>
      </aside>

      <div className="grid min-w-0 content-start gap-4">
        <Card className="gap-3">
          <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center">
            <div className="grid flex-1 gap-1">
              <CardTitle className="flex items-center gap-2">
                {rows.length} event{rows.length === 1 ? "" : "s"} {loading && <Spinner />}
              </CardTitle>
              <CardDescription>{day === "all" ? "Per day, last 14 days" : `Per hour, ${dayLabel(day).toLowerCase()}`}</CardDescription>
            </div>
            <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row">
              <SearchInput value={activityQuery} onChange={setActivityQuery} placeholder="Player, moderator, reason, room" className="sm:w-64" />
              <Select value={day} onValueChange={setDay}>
                <SelectTrigger className="w-full sm:w-40">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {DAY_OPTIONS.map((d) => (
                    <SelectItem key={d.value} value={d.value}>
                      {d.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </CardHeader>
          <CardContent>
            <Histogram logs={rows} day={day} />
          </CardContent>
        </Card>

        <Card className="gap-0 py-2">
          <CardContent className="px-2">
            {groups.length ? (
              <>
                {groups.map((group) => (
                  <section key={group.day} className="grid gap-0.5 py-1">
                    <div className="flex items-center gap-3 px-3 pt-2 pb-1">
                      <span className="text-xs font-semibold text-muted-foreground">{dayLabel(group.day)}</span>
                      <Separator className="flex-1" />
                      <span className="text-xs text-muted-foreground tabular-nums">{group.entries.length}</span>
                    </div>
                    {group.entries.map((log) => (
                      <EventRow key={log.id} log={log} onOpen={() => setSelected(log)} />
                    ))}
                  </section>
                ))}
                <div className="flex flex-col items-center gap-2 py-3">
                  {rows.length > limit && (
                    <Button variant="outline" size="sm" onClick={() => setLimit(limit + STEP)}>
                      Show {Math.min(STEP, rows.length - limit)} more
                    </Button>
                  )}
                  <span className="text-xs text-muted-foreground">
                    {Math.min(limit, rows.length)} of {rows.length}
                  </span>
                </div>
              </>
            ) : (
              <Empty className="py-20">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <Inbox />
                  </EmptyMedia>
                  <EmptyTitle>{loading ? "Loading events…" : "No events"}</EmptyTitle>
                  <EmptyDescription>Nothing matches this period and filter.</EmptyDescription>
                </EmptyHeader>
              </Empty>
            )}
          </CardContent>
        </Card>
      </div>
      <EventSheet log={selected} onClose={() => setSelected(null)} />
    </div>
  )
}
