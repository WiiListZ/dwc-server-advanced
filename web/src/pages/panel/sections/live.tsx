import type { ColumnDef } from "@tanstack/react-table"
import { ChevronRight, DoorOpen, Plus, Timer, Users } from "lucide-react"
import { useEffect, useMemo, useState } from "react"

import { DataTable } from "@/components/data-table"
import { RoomFilterToggle, SearchInput, type RoomFilter } from "@/components/filters"
import { StatusBadge } from "@/components/status-badge"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import type { OnlineUser } from "@/lib/api"
import { initials, roomType, sortRoomsSelfFirst } from "@/lib/format"
import { useNow } from "@/lib/hooks"
import { ROOM_RIGHTS } from "@/lib/rights"
import type { ForceJoin } from "@/pages/panel/join-dialog"
import { usePanel } from "@/pages/panel/panel-context"
import { RoomTile } from "@/pages/panel/room-views"

function ForceJoinBanner({ join, onEnd }: { join: ForceJoin; onEnd: () => void }) {
  const now = useNow(1000)
  const remaining = Math.max(0, Math.ceil((new Date(join.expires).getTime() - now) / 1000))
  useEffect(() => {
    if (remaining <= 0) onEnd()
  }, [remaining, onEnd])
  if (remaining <= 0) return null
  return (
    <Alert>
      <Timer />
      <AlertTitle>
        Force join active · {Math.floor(remaining / 60)}:{String(remaining % 60).padStart(2, "0")}
      </AlertTitle>
      <AlertDescription>
        <span>
          <span className="font-mono">{join.identifier}</span> joins room <span className="font-mono">{join.roomId}</span> on their next search.
        </span>
      </AlertDescription>
    </Alert>
  )
}

const isMKW = (u: OnlineUser) => u.game === "mariokartwii" || String(u.game_code || "").startsWith("RMC")

export function LiveSection({ forceJoin, clearForceJoin }: { forceJoin: ForceJoin | null; clearForceJoin: () => void }) {
  const { dashboard, dashboardLoading, account, can, canAny, openPlayer, openCreateRoom } = usePanel()
  const [filter, setFilter] = useState<RoomFilter>("all")
  const [query, setQuery] = useState("")
  const showRooms = canAny(ROOM_RIGHTS)
  const showPlayers = can("view_users")

  const rooms = useMemo(
    () => sortRoomsSelfFirst(dashboard.rooms.filter((r) => filter === "all" || roomType(r) === filter), account),
    [dashboard.rooms, filter, account],
  )

  const roomOf = useMemo(() => {
    const map = new Map<string, string>()
    for (const room of dashboard.rooms) for (const p of Object.values(room.players || {})) if (p.pid) map.set(String(p.pid), room.id)
    return map
  }, [dashboard.rooms])

  const columns = useMemo<ColumnDef<OnlineUser>[]>(
    () => [
      {
        accessorKey: "name",
        header: "Player",
        cell: ({ row }) => (
          <div className="flex items-center gap-2.5">
            <Avatar className="size-8">
              <AvatarFallback className="text-[11px]">{initials(row.original.name || "?")}</AvatarFallback>
            </Avatar>
            <div className="grid">
              <span className="font-medium">{row.original.name || "-"}</span>
              <span className="font-mono text-xs text-muted-foreground">{[row.original.fc, row.original.pid && `PID ${row.original.pid}`].filter(Boolean).join(" · ")}</span>
            </div>
          </div>
        ),
      },
      { id: "game", header: "Game", accessorFn: (u) => u.game || u.game_code, cell: ({ row }) => <Badge variant="secondary">{row.original.game || row.original.game_code}</Badge> },
      {
        id: "where",
        header: "Where",
        accessorFn: (u) => (u.pid && roomOf.get(String(u.pid))) || "",
        cell: ({ row }) => {
          const room = row.original.pid && roomOf.get(String(row.original.pid))
          return room ? <span className="font-mono text-xs">{room}</span> : <span className="text-xs text-muted-foreground">Lobby</span>
        },
      },
      {
        id: "status",
        header: "Status",
        accessorFn: (u) => (u.restricted ? "Restricted" : "Online"),
        cell: ({ row }) => (
          <div className="flex flex-wrap gap-1">
            <StatusBadge tone={row.original.restricted ? "danger" : "success"}>{row.original.restricted ? "Restricted" : row.original.status || "Online"}</StatusBadge>
            {isMKW(row.original) && <StatusBadge tone={row.original.payload_features ? "info" : "neutral"}>{row.original.payload_features ? "Payload" : "No payload"}</StatusBadge>}
          </div>
        ),
      },
      { id: "open", header: "", enableSorting: false, cell: () => <ChevronRight className="ml-auto size-4 text-muted-foreground" /> },
    ],
    [roomOf],
  )

  return (
    <div className="grid gap-6">
      {forceJoin && <ForceJoinBanner join={forceJoin} onEnd={clearForceJoin} />}
      <Tabs defaultValue={showRooms ? "rooms" : "players"} className="gap-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <TabsList>
            {showRooms && (
              <TabsTrigger value="rooms">
                <DoorOpen /> Rooms <Badge variant="secondary">{dashboard.rooms.length}</Badge>
              </TabsTrigger>
            )}
            {showPlayers && (
              <TabsTrigger value="players">
                <Users /> Players <Badge variant="secondary">{dashboard.users.length}</Badge>
              </TabsTrigger>
            )}
          </TabsList>
          {can("manage_rooms_create") && (
            <Button size="sm" onClick={() => openCreateRoom()}>
              <Plus data-icon="inline-start" /> Create room
            </Button>
          )}
        </div>
        {showRooms && (
          <TabsContent value="rooms" className="grid gap-4">
            <RoomFilterToggle value={filter} onChange={setFilter} />
            {rooms.length ? (
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                {rooms.map((room) => (
                  <RoomTile key={room.id} room={room} />
                ))}
              </div>
            ) : (
              <Empty className="border py-16">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <DoorOpen />
                  </EmptyMedia>
                  <EmptyTitle>No rooms open</EmptyTitle>
                  <EmptyDescription>Rooms show up live as soon as players start matchmaking.</EmptyDescription>
                </EmptyHeader>
              </Empty>
            )}
          </TabsContent>
        )}
        {showPlayers && (
          <TabsContent value="players">
            <DataTable
              columns={columns}
              data={dashboard.users}
              loading={dashboardLoading}
              globalFilter={query}
              filterFn={(u, q) => [u.pid, u.fc, u.name, u.game, u.game_code].some((v) => String(v ?? "").toLowerCase().includes(q))}
              emptyTitle="Nobody is online"
              onRowClick={(u) => openPlayer({ pid: u.pid, fc: u.fc, name: u.name })}
              toolbar={<SearchInput value={query} onChange={setQuery} placeholder="Name, friend code or profile ID" />}
            />
          </TabsContent>
        )}
      </Tabs>
    </div>
  )
}
