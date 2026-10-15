import { Lock, Plus, X } from "lucide-react"
import { useState } from "react"
import { toast } from "sonner"

import { StatusBadge } from "@/components/status-badge"
import { Badge } from "@/components/ui/badge"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group"
import { Item, ItemActions, ItemContent, ItemDescription, ItemTitle } from "@/components/ui/item"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Spinner } from "@/components/ui/spinner"
import { errorMessage, post, type OpenHostMode, type OpenHostProfile } from "@/lib/api"

const displayFor = (pid: number) => `PID ${pid}`

function normalizeDisplays(pids: number[], display?: string[]) {
  return pids.map((pid, i) => (display?.[i] || "").trim() || displayFor(pid))
}

const isFriendCodeLike = (s: string) => /-/.test(s) || /^\d{12}$/.test(s.replace(/\s+/g, ""))

export function OpenHostProfileRow({ profile, modes }: { profile: OpenHostProfile; modes: OpenHostMode[] }) {
  const locked = profile.right_locked
  const [mode, setMode] = useState(profile.mode)
  const [entries, setEntries] = useState(() => profile.list ?? [])
  const [displays, setDisplays] = useState(() => normalizeDisplays(profile.list ?? [], profile.list_display))
  const [input, setInput] = useState("")
  const [busy, setBusy] = useState(false)

  async function save(next: { mode: string; list: number[]; display: string[]; identifiers?: string[] }) {
    if (locked) return
    const previous = { mode, entries, displays }
    setMode(next.mode)
    setEntries(next.list)
    setDisplays(next.display)
    setBusy(true)
    try {
      const response = await post<{ mode: string; list: number[]; list_display: string[] }>("/api/panel/my/open-host", {
        pid: profile.pid,
        mode: next.mode,
        list: next.list,
        list_display: next.display,
        list_identifiers: next.identifiers ?? [],
      })
      const list = Array.isArray(response.list) ? response.list : []
      setMode(response.mode)
      setEntries(list)
      setDisplays(normalizeDisplays(list, response.list_display))
      toast.success(`Open Host saved for ${profile.name || displayFor(profile.pid)}`)
    } catch (error) {
      setMode(previous.mode)
      setEntries(previous.entries)
      setDisplays(previous.displays)
      toast.error(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  function add() {
    const raw = input.trim()
    if (!raw || busy || locked) return
    if (isFriendCodeLike(raw)) {
      setInput("")
      void save({ mode, list: entries, display: displays, identifiers: [raw] })
      return
    }
    const n = parseInt(raw, 10)
    if (!Number.isFinite(n) || n <= 0) return void toast.error("Enter a positive profile ID or a friend code like 1234-5678-9012")
    if (n === profile.pid) return void toast.error("You cannot add your own profile ID")
    if (entries.includes(n)) return void toast.error(`Profile ID ${n} is already in the list`)
    setInput("")
    void save({ mode, list: [...entries, n], display: [...displays, displayFor(n)] })
  }

  function remove(index: number) {
    void save({ mode, list: entries.filter((_, i) => i !== index), display: displays.filter((_, i) => i !== index) })
  }

  const listEnabled = mode !== "disabled"
  const details = [`PID ${profile.pid}`, profile.fc && `FC ${profile.fc}`, profile.primary && "primary"].filter(Boolean).join(" · ")

  return (
    <Item variant="outline" className="flex-col items-stretch gap-3">
      <div className="flex flex-wrap items-center gap-3">
        <ItemContent className="min-w-0">
          <ItemTitle className="flex items-center gap-2">
            {profile.name || displayFor(profile.pid)}
            <StatusBadge tone={profile.online ? "success" : "neutral"}>{profile.online ? "online" : "offline"}</StatusBadge>
          </ItemTitle>
          <ItemDescription className="font-mono text-xs">{details}</ItemDescription>
        </ItemContent>
        <ItemActions>
          {busy && <Spinner />}
          <Select value={mode} disabled={busy || locked} onValueChange={(value) => void save({ mode: value, list: entries, display: displays })}>
            <SelectTrigger className="w-40" aria-label="Open Host mode">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {modes.map((m) => (
                <SelectItem key={m.value} value={m.value}>
                  {m.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </ItemActions>
      </div>
      {locked && (
        <StatusBadge tone="danger" className="w-fit gap-1">
          <Lock className="size-3" /> Open Host right revoked by a moderator
        </StatusBadge>
      )}
      {listEnabled && (
        <Field>
          <FieldLabel>{mode === "whitelist" ? "Whitelist" : "Blacklist"}</FieldLabel>
          <FieldDescription>
            {mode === "whitelist"
              ? "Only these players are auto-friended."
              : "These players are blocked from auto-friend and from your hosted matches."}
          </FieldDescription>
          <div className="flex flex-wrap gap-1.5">
            {entries.length ? (
              entries.map((pid, i) => (
                <Badge key={pid} variant="secondary" className="gap-1 pr-1">
                  {displays[i] || displayFor(pid)}
                  {!locked && (
                    <button
                      type="button"
                      className="rounded-sm p-0.5 hover:bg-background/60"
                      aria-label={`Remove ${displays[i] || displayFor(pid)}`}
                      disabled={busy}
                      onClick={() => remove(i)}
                    >
                      <X className="size-3" />
                    </button>
                  )}
                </Badge>
              ))
            ) : (
              <span className="text-sm text-muted-foreground">No entries yet.</span>
            )}
          </div>
          <InputGroup className="max-w-md">
            <InputGroupInput
              value={input}
              disabled={busy || locked}
              placeholder="Profile ID or friend code (1234-5678-9012)"
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault()
                  add()
                }
              }}
            />
            <InputGroupAddon align="inline-end">
              <InputGroupButton disabled={busy || locked || !input.trim()} onClick={add}>
                <Plus /> Add
              </InputGroupButton>
            </InputGroupAddon>
          </InputGroup>
        </Field>
      )}
    </Item>
  )
}
