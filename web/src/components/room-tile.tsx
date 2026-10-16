import { ChevronRight, Clock3, Globe, Lock } from "lucide-react"

import { StatusBadge, type Tone } from "@/components/status-badge"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Card } from "@/components/ui/card"
import type { Account, Room } from "@/lib/api"
import { fmtOptionalDate, initials, playerName, roomContainsAccount, roomStateLabel, roomType, sortedPlayers } from "@/lib/format"
import { cn } from "@/lib/utils"

export const stateTone = (state?: string): Tone => (state === "playing" ? "success" : state === "matchmaking" ? "info" : "neutral")

export function RoomTileView({ room, account, onOpen }: { room: Room; account?: Account; onOpen: () => void }) {
  const players = sortedPlayers(room, account)
  const shown = players.slice(0, 6)
  const isPrivate = roomType(room) === "private"
  const mine = roomContainsAccount(room, account)
  return (
    <Card
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && onOpen()}
      className={cn(
        "group cursor-pointer gap-3 p-4 transition-colors hover:border-foreground/20 hover:bg-muted/30 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none",
        mine && "border-primary bg-primary/5 ring-2 ring-primary/50",
      )}
    >
      <div className="flex items-start gap-3">
        <div className={cn("flex size-9 shrink-0 items-center justify-center", isPrivate ? "text-warning" : "text-chart-1")}>
          {isPrivate ? <Lock className="size-4" /> : <Globe className="size-4" />}
        </div>
        <div className="grid min-w-0 flex-1 gap-0.5">
          <span className="flex items-center gap-1.5 truncate text-sm font-semibold">
            {room.game || "game"} {mine && <Badge className="h-4 px-1.5 text-[10px]">You</Badge>}
          </span>
          <span className="truncate font-mono text-xs text-muted-foreground">{room.id}</span>
        </div>
        <ChevronRight className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
      </div>
      <div className="flex flex-wrap items-center gap-1.5">
        <Badge variant="outline">{isPrivate ? "Private" : "Public"}</Badge>
        <StatusBadge tone={stateTone(room.state)}>{roomStateLabel(room.state)}</StatusBadge>
        {room.race?.num ? <Badge variant="outline">Race {room.race.num}</Badge> : null}
        {room.rk && <Badge variant="outline">{room.rk}</Badge>}
      </div>
      <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Clock3 className="size-3.5" /> Created {fmtOptionalDate(room.created)}
      </div>
      <div className="flex items-center justify-between">
        <div className="flex -space-x-2">
          {shown.map(([slot, p]) => (
            <Avatar key={slot} className="size-7 border-2 border-card">
              <AvatarFallback className="text-[10px]">{initials(playerName(p))}</AvatarFallback>
            </Avatar>
          ))}
          {players.length > shown.length && (
            <div className="flex size-7 items-center justify-center rounded-full border-2 border-card bg-muted text-[10px] font-medium">+{players.length - shown.length}</div>
          )}
        </div>
        <span className="text-xs text-muted-foreground tabular-nums">
          {players.length} player{players.length === 1 ? "" : "s"}
        </span>
      </div>
    </Card>
  )
}
