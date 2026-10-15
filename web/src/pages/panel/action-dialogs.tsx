import { Ban, DoorOpen, MessageSquare, UserMinus } from "lucide-react"
import { useEffect, useMemo, useState } from "react"
import { toast } from "sonner"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { Textarea } from "@/components/ui/textarea"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { errorMessage, post } from "@/lib/api"
import { initials } from "@/lib/format"
import { cn } from "@/lib/utils"
import { playerKey, playerLabel, usePanel, type ActionKind, type PlayerRef } from "@/pages/panel/panel-context"

const DURATIONS = [
  { value: "3m", label: "3m" },
  { value: "1h", label: "1h" },
  { value: "1d", label: "1d" },
  { value: "3d", label: "3d" },
  { value: "1w", label: "1w" },
  { value: "4w", label: "4w" },
  { value: "6mo", label: "6mo" },
  { value: "permanent", label: "Forever" },
]

const REASONS: Record<ActionKind, string[]> = {
  ban: ["Cheating", "Invalid ghost upload", "Offensive name", "Disconnecting rooms on purpose", "Harassment"],
  kick: ["Disrupting the room", "AFK in a race", "Offensive name", "Requested by the room host"],
  message: [],
}

const META: Record<ActionKind, { title: string; icon: typeof Ban; submit: string; destructive: boolean }> = {
  ban: { title: "Ban player", icon: Ban, submit: "Apply ban", destructive: true },
  kick: { title: "Kick player", icon: UserMinus, submit: "Send kick", destructive: true },
  message: { title: "Send login message", icon: MessageSquare, submit: "Queue message", destructive: false },
}

// PlayerField takes a friend code or profile ID and suggests online players.
export function PlayerField({ value, onChange, autoFocus }: { value: string; onChange: (value: string, player?: PlayerRef) => void; autoFocus?: boolean }) {
  const { dashboard } = usePanel()
  const [focused, setFocused] = useState(false)
  const suggestions = useMemo(() => {
    const q = value.trim().toLowerCase()
    if (!q) return []
    return dashboard.users
      .filter((u) => [u.name, u.fc, u.pid].some((v) => String(v ?? "").toLowerCase().includes(q)))
      .filter((u) => String(u.pid ?? "") !== value.trim() && (u.fc ?? "") !== value.trim())
      .slice(0, 5)
  }, [dashboard.users, value])

  return (
    <Field>
      <FieldLabel htmlFor="action-player">Player</FieldLabel>
      <div className="relative">
        <Input
          id="action-player"
          autoFocus={autoFocus}
          required
          autoComplete="off"
          placeholder="Friend code, profile ID or online name"
          value={value}
          onFocus={() => setFocused(true)}
          onBlur={() => window.setTimeout(() => setFocused(false), 150)}
          onChange={(e) => onChange(e.target.value)}
        />
        {focused && suggestions.length > 0 && (
          <div className="absolute inset-x-0 top-full z-50 mt-1 grid gap-0.5 rounded-lg border bg-popover p-1 shadow-md">
            {suggestions.map((u) => (
              <button
                key={`${u.pid}-${u.fc}`}
                type="button"
                className="flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent"
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => onChange(String(u.pid || u.fc), { pid: u.pid, fc: u.fc, name: u.name })}
              >
                <Avatar className="size-6">
                  <AvatarFallback className="text-[10px]">{initials(u.name || "?")}</AvatarFallback>
                </Avatar>
                <span className="font-medium">{u.name}</span>
                <span className="ml-auto font-mono text-xs text-muted-foreground">{u.fc || u.pid}</span>
              </button>
            ))}
          </div>
        )}
      </div>
      <FieldDescription>Online players are suggested as you type.</FieldDescription>
    </Field>
  )
}

function PlayerChip({ player }: { player: PlayerRef }) {
  return (
    <div className="flex items-center gap-3 rounded-lg border bg-muted/40 p-3">
      <Avatar className="size-9">
        <AvatarFallback>{initials(playerLabel(player))}</AvatarFallback>
      </Avatar>
      <div className="grid">
        <span className="font-medium">{playerLabel(player)}</span>
        <span className="font-mono text-xs text-muted-foreground">{[player.fc && `FC ${player.fc}`, player.pid && `PID ${player.pid}`].filter(Boolean).join(" · ")}</span>
      </div>
    </div>
  )
}

export function ActionDialog() {
  const { action, closeAction, refreshDashboard, can } = usePanel()
  const kind = action?.kind ?? "ban"
  const preset = action?.player
  const [identifier, setIdentifier] = useState("")
  const [duration, setDuration] = useState("1d")
  const [reason, setReason] = useState("")
  const [notes, setNotes] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!action) return
    setIdentifier(action.player ? playerKey(action.player) : "")
    setDuration("1d")
    setReason("")
    setNotes("")
  }, [action])

  const meta = META[kind]
  const allowed = kind === "ban" ? can("ban") : kind === "kick" ? can("kick") : can("message")
  const maxLength = kind === "message" ? 500 : 200
  const target = identifier.trim()

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    if (!target || !reason.trim()) return
    setBusy(true)
    try {
      if (kind === "ban") {
        await post("/api/panel/ban", { identifier: target, duration, reason: reason.trim(), reason_hidden: notes.trim(), tos: true })
        toast.success(`Ban applied to ${preset ? playerLabel(preset) : target}`, { description: DURATIONS.find((d) => d.value === duration)?.label })
      } else if (kind === "kick") {
        await post("/api/panel/kick", { identifier: target, reason: reason.trim() })
        toast.success(`Kick sent to ${preset ? playerLabel(preset) : target}`)
      } else {
        const response = await post<{ notice?: string }>("/api/panel/login-message", { identifier: target, message: reason.trim() })
        toast.success(response.notice || "Login message queued for the next login")
      }
      closeAction()
      void refreshDashboard()
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={!!action} onOpenChange={(open) => !open && closeAction()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-5">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <meta.icon className={cn("size-4", meta.destructive && "text-destructive")} /> {meta.title}
            </DialogTitle>
            <DialogDescription>
              {kind === "ban"
                ? "Bans the whole console: every profile on it is blocked."
                : kind === "kick"
                  ? "Disconnects the player from the service right away."
                  : "Shown the next time the player logs in (payload clients)."}
            </DialogDescription>
          </DialogHeader>
          {!allowed && <p className="text-sm text-destructive">Your account does not have this right.</p>}
          <FieldGroup>
            {preset && playerKey(preset) === target ? <PlayerChip player={preset} /> : <PlayerField value={identifier} onChange={(v) => setIdentifier(v)} autoFocus={!preset} />}
            {kind === "ban" && (
              <Field>
                <FieldLabel>Duration</FieldLabel>
                <ToggleGroup type="single" variant="outline" value={duration} onValueChange={(v) => v && setDuration(v)} className="flex-wrap">
                  {DURATIONS.map((d) => (
                    <ToggleGroupItem key={d.value} value={d.value} className="px-3 data-[state=on]:bg-primary data-[state=on]:text-primary-foreground">
                      {d.label}
                    </ToggleGroupItem>
                  ))}
                </ToggleGroup>
              </Field>
            )}
            <Field>
              <FieldLabel htmlFor="action-reason">{kind === "message" ? "Message" : kind === "ban" ? "Public reason" : "Reason"}</FieldLabel>
              {REASONS[kind].length > 0 && (
                <div className="flex flex-wrap gap-1.5">
                  {REASONS[kind].map((text) => (
                    <Badge key={text} asChild variant={reason === text ? "default" : "outline"} className="cursor-pointer">
                      <button type="button" onClick={() => setReason(text)}>
                        {text}
                      </button>
                    </Badge>
                  ))}
                </div>
              )}
              <Textarea id="action-reason" autoFocus={!!preset} required maxLength={maxLength} value={reason} onChange={(e) => setReason(e.target.value)} />
              <FieldDescription>
                {kind === "ban" ? "Shown on the public ban list. " : ""}
                {reason.length}/{maxLength}
              </FieldDescription>
            </Field>
            {kind === "ban" && (
              <Field>
                <FieldLabel htmlFor="action-notes">Private notes</FieldLabel>
                <Textarea id="action-notes" className="min-h-16" placeholder="Only moderators see this" value={notes} onChange={(e) => setNotes(e.target.value)} />
              </Field>
            )}
          </FieldGroup>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={closeAction}>
              Cancel
            </Button>
            <Button type="submit" variant={meta.destructive ? "destructive" : "default"} disabled={busy || !allowed || !target || !reason.trim()}>
              {busy ? <Spinner /> : <meta.icon data-icon="inline-start" />} {meta.submit}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export function CreateRoomDialog() {
  const { createRoomHost, closeCreateRoom, refreshDashboard } = usePanel()
  const [identifier, setIdentifier] = useState("")
  const [type, setType] = useState("public")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (createRoomHost !== null) {
      setIdentifier(createRoomHost)
      setType("public")
    }
  }, [createRoomHost])

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      const response = await post<{ room?: { id?: string; status?: string } }>("/api/panel/create-room", { identifier: identifier.trim(), type })
      const room = response.room || {}
      toast.success(room.status === "existing" ? `Host is already in room ${room.id}` : `Created room ${room.id ?? ""}`)
      closeCreateRoom()
      void refreshDashboard()
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={createRoomHost !== null} onOpenChange={(open) => !open && closeCreateRoom()}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="grid gap-5">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <DoorOpen className="size-4" /> Create room
            </DialogTitle>
            <DialogDescription>Opens a matchmaking room hosted by an online profile.</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <PlayerField value={identifier} onChange={(v) => setIdentifier(v)} autoFocus />
            <Field>
              <FieldLabel>Room type</FieldLabel>
              <ToggleGroup type="single" variant="outline" value={type} onValueChange={(v) => v && setType(v)} className="w-full">
                <ToggleGroupItem value="public" className="flex-1">
                  Public
                </ToggleGroupItem>
                <ToggleGroupItem value="private" className="flex-1">
                  Private
                </ToggleGroupItem>
              </ToggleGroup>
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={closeCreateRoom}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy || !identifier.trim()}>
              {busy && <Spinner />} Create room
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
