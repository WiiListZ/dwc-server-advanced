import { ArrowDownToLine, Download, Eraser, Pause, Play, SquareTerminal } from "lucide-react"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"

import { SearchInput } from "@/components/filters"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { api, errorMessage, list } from "@/lib/api"
import { useInterval } from "@/lib/hooks"
import { cn } from "@/lib/utils"

type ConsoleLine = { seq: number; time: string; text: string }
type ConsoleResponse = { source: "server" | "backend"; lines: ConsoleLine[]; next: number; dropped: boolean }
type Level = "all" | "warn" | "error"

const MAX_LINES = 5000
const ANSI = /\x1b\[([0-9;]*)m/g

const COLORS: Record<string, string> = {
  "30": "text-muted-foreground",
  "31": "text-red-600 dark:text-red-400",
  "32": "text-green-600 dark:text-green-400",
  "33": "text-amber-600 dark:text-amber-400",
  "34": "text-blue-600 dark:text-blue-400",
  "35": "text-fuchsia-600 dark:text-fuchsia-400",
  "36": "text-cyan-600 dark:text-cyan-400",
  "37": "text-foreground",
  "90": "text-muted-foreground",
}
COLORS["91"] = COLORS["31"]
COLORS["92"] = COLORS["32"]
COLORS["93"] = COLORS["33"]
COLORS["94"] = COLORS["34"]
COLORS["95"] = COLORS["35"]
COLORS["96"] = COLORS["36"]
COLORS["97"] = COLORS["37"]

const stripAnsi = (text: string) => text.replace(ANSI, "")

/** Renders a line with its ANSI colours as spans. */
function AnsiLine({ text }: { text: string }) {
  const parts: { text: string; className: string }[] = []
  let color = ""
  let bold = false
  let last = 0
  for (const match of text.matchAll(ANSI)) {
    if (match.index > last) parts.push({ text: text.slice(last, match.index), className: cn(color, bold && "font-semibold") })
    for (const code of (match[1] || "0").split(";")) {
      if (code === "0" || code === "") {
        color = ""
        bold = false
      } else if (code === "1") bold = true
      else if (code === "22") bold = false
      else if (code === "39") color = ""
      else if (COLORS[code]) color = COLORS[code]
    }
    last = match.index + match[0].length
  }
  if (last < text.length) parts.push({ text: text.slice(last), className: cn(color, bold && "font-semibold") })
  return (
    <>
      {parts.map((part, i) => (
        <span key={i} className={part.className || undefined}>
          {part.text}
        </span>
      ))}
    </>
  )
}

function levelOf(text: string): "error" | "warn" | "other" {
  const plain = stripAnsi(text)
  if (/^\S+ \S+ E\[/.test(plain) || /^(panic|fatal error)/i.test(plain)) return "error"
  if (/^\S+ \S+ W\[/.test(plain)) return "warn"
  return "other"
}

/** The server console, as printed in the terminal, live. */
export function ServerConsoleCard() {
  const [lines, setLines] = useState<ConsoleLine[]>([])
  const [next, setNext] = useState(0)
  const [source, setSource] = useState<ConsoleResponse["source"]>("server")
  const [error, setError] = useState("")
  const [paused, setPaused] = useState(false)
  const [follow, setFollow] = useState(true)
  const [level, setLevel] = useState<Level>("all")
  const [query, setQuery] = useState("")
  const [clearedAt, setClearedAt] = useState(0)
  const scroller = useRef<HTMLDivElement>(null)
  const loading = useRef(false)

  const poll = useCallback(async () => {
    if (loading.current) return
    loading.current = true
    try {
      const response = await api<ConsoleResponse>(`/api/panel/server-console?since=${next}`)
      setSource(response.source)
      setError("")
      const incoming = list(response.lines)
      // A lower sequence number means the server restarted.
      if (response.next < next) setLines(incoming.slice(-MAX_LINES))
      else if (incoming.length) setLines((current) => [...current, ...incoming].slice(-MAX_LINES))
      setNext(response.next)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      loading.current = false
    }
  }, [next])

  useEffect(() => {
    void poll()
    // Only the first load; later polls run on the interval.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  useInterval(() => void poll(), paused ? null : 1500)

  const visible = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return lines.filter((line) => {
      if (line.seq <= clearedAt) return false
      if (level !== "all") {
        const lineLevel = levelOf(line.text)
        if (level === "error" && lineLevel !== "error") return false
        if (level === "warn" && lineLevel === "other") return false
      }
      return !needle || stripAnsi(line.text).toLowerCase().includes(needle)
    })
  }, [lines, level, query, clearedAt])

  useEffect(() => {
    const el = scroller.current
    if (follow && el) el.scrollTop = el.scrollHeight
  }, [visible, follow])

  function onScroll() {
    const el = scroller.current
    if (!el) return
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 24
    if (atBottom !== follow) setFollow(atBottom)
  }

  function download() {
    const text = visible.map((line) => stripAnsi(line.text)).join("\n") + "\n"
    const url = URL.createObjectURL(new Blob([text], { type: "text/plain" }))
    const a = document.createElement("a")
    a.href = url
    a.download = `console-${new Date().toISOString().replace(/[:.]/g, "-")}.txt`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <SquareTerminal className="size-4" /> Console
        </CardTitle>
        <CardDescription>
          What the server prints in its terminal, live. The last {MAX_LINES.toLocaleString()} lines are kept; older ones are in the logs/ folder.
        </CardDescription>
        <CardAction className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => setPaused(!paused)}>
            {paused ? <Play data-icon="inline-start" /> : <Pause data-icon="inline-start" />} {paused ? "Resume" : "Pause"}
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className="grid gap-3">
        {source === "backend" && (
          <Alert>
            <SquareTerminal />
            <AlertTitle>Showing the backend only</AlertTitle>
            <AlertDescription>The frontend process did not answer, so lines printed by it (game connections, restarts) are missing.</AlertDescription>
          </Alert>
        )}
        <div className="flex flex-wrap items-center gap-2">
          <ToggleGroup type="single" variant="outline" size="sm" value={level} onValueChange={(v) => v && setLevel(v as Level)}>
            <ToggleGroupItem value="all">All</ToggleGroupItem>
            <ToggleGroupItem value="warn">Warnings</ToggleGroupItem>
            <ToggleGroupItem value="error">Errors</ToggleGroupItem>
          </ToggleGroup>
          <SearchInput value={query} onChange={setQuery} placeholder="Filter lines" className="w-full sm:w-64" />
          <div className="ml-auto flex gap-1">
            <Button variant="ghost" size="icon-sm" aria-label="Clear the view" onClick={() => setClearedAt(lines.at(-1)?.seq ?? 0)}>
              <Eraser />
            </Button>
            <Button variant="ghost" size="icon-sm" aria-label="Download the visible lines" onClick={download}>
              <Download />
            </Button>
          </div>
        </div>
        <div className="relative">
          <div
            ref={scroller}
            onScroll={onScroll}
            className="h-[60vh] min-h-72 overflow-auto rounded-lg border bg-muted/30 p-3 font-mono text-xs leading-relaxed"
            role="log"
            aria-live="off"
          >
            {visible.length === 0 ? (
              <p className="text-muted-foreground">{error || (lines.length ? "No line matches the filter." : "Waiting for output…")}</p>
            ) : (
              visible.map((line) => (
                <div key={line.seq} className="break-all whitespace-pre-wrap">
                  <AnsiLine text={line.text} />
                </div>
              ))
            )}
          </div>
          {!follow && (
            <Button size="sm" variant="secondary" className="absolute right-4 bottom-4 shadow" onClick={() => setFollow(true)}>
              <ArrowDownToLine data-icon="inline-start" /> Latest
            </Button>
          )}
        </div>
        <p className="text-xs text-muted-foreground">
          {visible.length.toLocaleString()} of {lines.length.toLocaleString()} lines{paused ? " · paused" : ""}
          {error && lines.length ? ` · ${error}` : ""}
        </p>
      </CardContent>
    </Card>
  )
}
