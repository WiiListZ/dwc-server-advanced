import { ChevronRight, Crown, Flag, Globe, LogIn, Lock, Trash2 } from "lucide-react"
import { toast } from "sonner"

import { useConfirm } from "@/components/confirm"
import { RoomTileView, stateTone } from "@/components/room-tile"
import { StatusBadge } from "@/components/status-badge"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemMedia, ItemTitle } from "@/components/ui/item"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { errorMessage, list, post, type RaceEndStatus, type Room } from "@/lib/api"
import { initials, playerName, playerStateLabel, relativeTime, roomStateLabel, roomType, sortedPlayers } from "@/lib/format"
import { usePanel } from "@/pages/panel/panel-context"

const RACE_END_RESULTS: Record<string, string> = {
  finished: "Race ended",
  waiting_for_race: "Waiting for race start",
  server_decision_message: "Notice shown",
  server_decision_message_failed: "Notice not shown",
  native_timeout: "Race timer expired",
  native_timeout_failed: "Timer had no effect",
  not_in_race: "Not in a race",
}

const raceEndResultLabel = (result: string) => RACE_END_RESULTS[result] ?? result

function RaceEndSummary({ status }: { status: RaceEndStatus }) {
  const confirmed = status.payload > 0 ? `${status.finished}/${status.payload} confirmed in game` : ""
  const lines = [
    status.payload > 0 && `In-game command sent to ${status.payload}`,
    status.disconnect > 0 && `connections closed for ${status.disconnect}`,
    confirmed,
  ].filter(Boolean)
  return (
    <div className="grid gap-1 rounded-lg border p-3 text-sm">
      <span className="flex items-center gap-1.5 font-medium">
        <Flag className="size-3.5" /> Race end requested {relativeTime(status.requested)}
      </span>
      <span className="text-muted-foreground">{lines.join(" · ") || "No player could be reached."}</span>
      {status.fallback && <span className="text-muted-foreground">Not every payload player confirmed in time, so their connections were closed.</span>}
    </div>
  )
}

export function RoomTile({ room }: { room: Room }) {
  const { openRoom, account } = usePanel()
  return <RoomTileView room={room} account={account} onOpen={() => openRoom(room.id)} />
}

export function RoomSheet() {
  const { roomId, closeRoom, dashboard, account, can, openPlayer, openJoinDialog, refreshDashboard } = usePanel()
  const { confirm } = useConfirm()
  const room = dashboard.rooms.find((r) => r.id === roomId)
  const players = room ? sortedPlayers(room, account) : []

  async function deleteRoom(room: Room) {
    const ok = await confirm({
      title: `Delete room ${room.id}?`,
      description: `This kicks ${players.length} player${players.length === 1 ? "" : "s"} from the room.`,
      confirmLabel: "Delete room",
      destructive: true,
    })
    if (!ok) return
    try {
      const response = await post<{ room?: { id?: string; players?: unknown[] } }>("/api/panel/delete-room", { room_id: room.id })
      const kicked = list(response.room?.players).length
      toast.success(`Deleted room ${room.id}`, { description: `Kicked ${kicked} player${kicked === 1 ? "" : "s"}.` })
      closeRoom()
      void refreshDashboard()
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  async function endRace(room: Room) {
    const ok = await confirm({
      title: `End ${room.race?.num ? `race ${room.race.num}` : "the race"}?`,
      description:
        "Players with the payload get the in-game race-end command. Players without it have their connections to the room closed, which ends their race. If a payload player does not confirm within 20 seconds, its connections are closed too.",
      confirmLabel: "End race",
      destructive: true,
    })
    if (!ok) return
    try {
      const response = await post<{ room?: { payload_targets?: number; disconnect_targets?: number } }>("/api/panel/end-game", { room_id: room.id, end_mode: "auto" })
      const payload = Number(response.room?.payload_targets || 0)
      const disconnect = Number(response.room?.disconnect_targets || 0)
      const parts = [payload && `in-game command to ${payload}`, disconnect && `connections closed for ${disconnect}`].filter(Boolean)
      toast.success("Race end requested", { description: parts.length ? `${parts.join(", ")}. Results appear in the room.` : "No player could be reached." })
      void refreshDashboard()
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  return (
    <Dialog open={!!roomId} onOpenChange={(open) => !open && closeRoom()}>
      <DialogContent className="flex max-h-[85vh] flex-col sm:max-w-lg">
        {room ? (
          <>
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                {roomType(room) === "private" ? <Lock className="size-4 text-warning" /> : <Globe className="size-4 text-chart-1" />}
                {room.game}
              </DialogTitle>
              <DialogDescription className="font-mono">{room.id}</DialogDescription>
              <div className="flex flex-wrap gap-1.5 pt-1">
                <StatusBadge tone={roomType(room) === "private" ? "warning" : "info"}>{roomType(room)}</StatusBadge>
                <StatusBadge tone={stateTone(room.state)}>{roomStateLabel(room.state)}</StatusBadge>
                {room.race?.num ? <Badge variant="outline">Race {room.race.num}</Badge> : null}
                {room.rk && <Badge variant="outline">{room.rk}</Badge>}
                {room.created && <Badge variant="outline">opened {relativeTime(room.created)}</Badge>}
              </div>
            </DialogHeader>
            <div className="grid min-h-0 gap-2 overflow-y-auto">
              {room.race_end && <RaceEndSummary status={room.race_end} />}
              <h3 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">Players · {players.length}</h3>
              <ItemGroup className="gap-1.5">
                {players.map(([slot, p]) => {
                  const host = !!room.host_pid && String(room.host_pid) === String(p.pid ?? "")
                  return (
                    <Item key={slot} variant="outline" size="sm" asChild>
                      <button type="button" className="text-left" onClick={() => openPlayer({ pid: p.pid, fc: p.fc, name: p.name })}>
                        <ItemMedia>
                          <Avatar className="size-8">
                            <AvatarFallback className="text-[11px]">{initials(playerName(p))}</AvatarFallback>
                          </Avatar>
                        </ItemMedia>
                        <ItemContent className="min-w-0">
                          <ItemTitle className="flex items-center gap-1.5">
                            <span className="truncate">{playerName(p)}</span>
                            {host && <Crown className="size-3.5 text-warning" aria-label="Host" />}
                          </ItemTitle>
                          <ItemDescription className="font-mono text-xs">{[p.fc && `FC ${p.fc}`, p.pid && `PID ${p.pid}`, `slot ${slot}`].filter(Boolean).join(" · ")}</ItemDescription>
                        </ItemContent>
                        <ItemActions>
                          {p.pid && room.race_end?.results?.[String(p.pid)] && <Badge variant="outline">{raceEndResultLabel(room.race_end.results[String(p.pid)])}</Badge>}
                          <StatusBadge tone={p.state === "playing" ? "success" : "neutral"}>{playerStateLabel(p.state)}</StatusBadge>
                          <ChevronRight className="size-4 text-muted-foreground" />
                        </ItemActions>
                      </button>
                    </Item>
                  )
                })}
              </ItemGroup>
            </div>
            <DialogFooter className="flex-wrap sm:flex-row">
              {roomType(room) !== "private" && can("manage_rooms_join") && (
                <Button className="flex-1" onClick={() => openJoinDialog(room.id)}>
                  <LogIn data-icon="inline-start" /> Join room
                </Button>
              )}
              {can("manage_rooms_end_game") && room.game === "mariokartwii" && room.state === "playing" && room.race && (
                <Button variant="outline" className="flex-1" onClick={() => void endRace(room)}>
                  <Flag data-icon="inline-start" /> End race
                </Button>
              )}
              {can("manage_rooms_delete") && (
                <Button variant="destructive" className="flex-1" onClick={() => void deleteRoom(room)}>
                  <Trash2 data-icon="inline-start" /> Delete
                </Button>
              )}
            </DialogFooter>
          </>
        ) : (
          <DialogHeader>
            <DialogTitle>Room closed</DialogTitle>
            <DialogDescription>This room no longer exists.</DialogDescription>
          </DialogHeader>
        )}
      </DialogContent>
    </Dialog>
  )
}
