import type { ColumnDef } from "@tanstack/react-table"
import { Activity, Crown, DoorOpen, Flag, Gamepad2, Globe, Lock, RefreshCw } from "lucide-react"
import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"

import { banKindColumn, banPeriodColumn, banReasonColumn, banRemainingColumn, banTargetColumn } from "@/components/ban-columns"
import { useConfirm } from "@/components/confirm"
import { DataTable } from "@/components/data-table"
import { BanFilterToggle, RoomFilterToggle, SearchInput, type RoomFilter } from "@/components/filters"
import { PrivateKickDialog, type KickTarget } from "@/components/private-kick-dialog"
import { RoomTileView, stateTone } from "@/components/room-tile"
import { SiteHeader, type PublicTab } from "@/components/site-header"
import { StatusBadge } from "@/components/status-badge"
import { profileHref, UserAvatar, useProfileLinks } from "@/components/user-avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemMedia, ItemTitle } from "@/components/ui/item"
import { Progress } from "@/components/ui/progress"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { api, errorMessage, list, post, type Account, type BanRecord, type OnlineUser, type ProfileLink, type Room, type RoomPlayer, type Stats } from "@/lib/api"
import {
  accountProfileIDs,
  filterBans,
  playerIdentifier,
  playerMatchesAccount,
  playerName,
  playerStateLabel,
  roomContainsAccount,
  roomStateLabel,
  roomType,
  sortRoomsSelfFirst,
  sortedPlayers,
  type BanFilter,
} from "@/lib/format"
import { loadDisplaySettings, useInterval, useSession } from "@/lib/hooks"

const userColumns = (links: Map<string, ProfileLink>): ColumnDef<OnlineUser>[] => [
  {
    accessorKey: "name",
    header: "Player",
    cell: ({ row }) => {
      const link = row.original.fc ? links.get(row.original.fc) : undefined
      return (
        <div className="flex items-center gap-2.5">
          <UserAvatar name={row.original.name} src={link?.avatar_url} className="size-8" />
          <div className="grid">
            <span className="font-medium">{row.original.name || "-"}</span>
            <span className="font-mono text-xs text-muted-foreground">{row.original.fc || "-"}</span>
          </div>
        </div>
      )
    },
  },
  {
    id: "profile",
    header: "Profile",
    accessorFn: (u) => (u.fc ? links.get(u.fc)?.username ?? "" : ""),
    cell: ({ row }) => {
      const link = row.original.fc ? links.get(row.original.fc) : undefined
      return link ? (
        <a href={profileHref(link.username)} className="font-medium underline-offset-4 hover:underline" onClick={(e) => e.stopPropagation()}>
          {link.username}
        </a>
      ) : (
        <span className="text-muted-foreground">-</span>
      )
    },
  },
  { id: "game", header: "Game", accessorFn: (u) => u.game || u.game_code, cell: ({ row }) => <Badge variant="secondary">{row.original.game || row.original.game_code}</Badge> },
]

const banColumns: ColumnDef<BanRecord>[] = [banKindColumn(), banTargetColumn(false), banReasonColumn(), banPeriodColumn(), banRemainingColumn()]

function canKick(room: Room, player: RoomPlayer, account?: Account) {
  return (
    !!account &&
    account.role === "player" &&
    !!account.pid &&
    !!account.rights?.private_room_kick &&
    roomType(room) === "private" &&
    String(room.host_pid || "") === String(account.pid) &&
    String(player.pid || "") !== String(account.pid) &&
    !!playerIdentifier(player)
  )
}

function canReport(room: Room, player: RoomPlayer, account?: Account) {
  const ids = accountProfileIDs(account)
  return !!account && !!room.id && roomContainsAccount(room, account) && !(player.pid && ids.has(String(player.pid))) && !!playerIdentifier(player)
}

function TopGames({ stats }: { stats: Stats }) {
  const rows = Object.entries(stats)
    .filter(([game]) => game !== "global")
    .map(([game, s]) => ({ game, ...s }))
    .sort((a, b) => b.online - a.online)
    .slice(0, 5)
  const max = Math.max(1, ...rows.map((r) => r.online))
  if (!rows.length) return <p className="text-sm text-muted-foreground">Nobody is playing right now.</p>
  return (
    <ol className="grid gap-3">
      {rows.map((r, i) => (
        <li key={r.game} className="grid gap-1.5">
          <div className="flex items-center gap-2 text-sm">
            <span className="w-4 text-muted-foreground tabular-nums">{i + 1}</span>
            <span className="truncate font-medium">{r.game}</span>
            <span className="ml-auto text-muted-foreground tabular-nums">
              {r.online} <span className="text-xs">online</span>
            </span>
          </div>
          <Progress value={(r.online / max) * 100} className="h-1.5" />
        </li>
      ))}
    </ol>
  )
}

const PUBLIC_TABS: PublicTab[] = ["rooms", "players", "bans"]

function tabFromHash(): PublicTab {
  const hash = window.location.hash.replace(/^#/, "") as PublicTab
  return PUBLIC_TABS.includes(hash) ? hash : "rooms"
}

export function PublicPage() {
  const { session } = useSession()
  const { prompt } = useConfirm()
  const [settings] = useState(loadDisplaySettings)
  const [stats, setStats] = useState<Stats>({})
  const [users, setUsers] = useState<OnlineUser[]>([])
  const [rooms, setRooms] = useState<Room[]>([])
  const [bans, setBans] = useState<BanRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [updated, setUpdated] = useState<Date | null>(null)
  const [roomFilter, setRoomFilter] = useState<RoomFilter>("all")
  const [banFilter, setBanFilter] = useState<BanFilter>("all")
  const [banQuery, setBanQuery] = useState("")
  const [userQuery, setUserQuery] = useState("")
  const [openRoomId, setOpenRoomId] = useState<string | null>(null)
  const [kickTarget, setKickTarget] = useState<KickTarget | null>(null)
  const [tab, setTab] = useState<PublicTab>(tabFromHash)
  const account = session?.account
  const profileLinks = useProfileLinks(!!session?.features?.profiles, users)
  const visibleUserColumns = useMemo(() => {
    const columns = userColumns(profileLinks)
    return session?.features?.profiles ? columns : columns.filter((c) => c.id !== "profile")
  }, [profileLinks, session?.features?.profiles])

  useEffect(() => {
    const onHash = () => setTab(tabFromHash())
    window.addEventListener("hashchange", onHash)
    return () => window.removeEventListener("hashchange", onHash)
  }, [])

  const load = useCallback(async () => {
    try {
      const [s, u, r, b] = await Promise.all([
        api<Stats>("/api/stats"),
        api<OnlineUser[]>("/api/public/users"),
        api<Room[]>("/api/public/rooms"),
        api<BanRecord[]>("/api/public/bans"),
      ])
      setStats(s || {})
      setUsers(list(u))
      setRooms(list(r))
      setBans(list(b))
      setUpdated(new Date())
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])
  useInterval(() => void load(), settings.autoRefresh ? 10000 : null)

  const visibleRooms = useMemo(() => sortRoomsSelfFirst(rooms.filter((r) => roomFilter === "all" || roomType(r) === roomFilter), account), [rooms, roomFilter, account])
  const visibleBans = useMemo(() => filterBans(bans, banFilter, banQuery, account), [bans, banFilter, banQuery, account])
  const myRoom = account ? rooms.find((r) => roomContainsAccount(r, account)) : undefined
  const openRoom = rooms.find((r) => r.id === openRoomId)

  async function report(room: Room, player: RoomPlayer) {
    const reason = await prompt({
      title: `Report ${playerName(player)}`,
      description: "Moderators see this report together with the room it came from.",
      label: "What happened?",
      maxLength: 500,
      confirmLabel: "Submit report",
    })
    if (!reason) return
    try {
      await post("/api/panel/my/report", { room_id: room.id, pid: Number(player.pid || 0), fc: player.fc || "", reason })
      toast.success(`Report submitted for ${playerName(player)}`)
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  const global = stats.global ?? { online: 0, active: 0, groups: 0 }

  return (
    <div className="min-h-svh bg-background">
      <SiteHeader session={session} page={tab} subtitle="Public status" counts={{ rooms: rooms.length, players: users.length }} />
      {tab === "rooms" && (
        <section className="border-b">
          <div className="mx-auto grid max-w-6xl gap-8 px-4 py-12 lg:grid-cols-[1.4fr_1fr] lg:items-center">
            <div className="grid gap-5">
              <div className="grid gap-2">
                <h1 className="text-5xl font-semibold tracking-tight tabular-nums sm:text-7xl">{loading ? <Skeleton className="h-16 w-40" /> : global.online}</h1>
                <p className="text-lg text-muted-foreground">players connected to Nintendo Wi-Fi Connection right now.</p>
              </div>
              <div className="flex flex-wrap gap-6 text-sm">
                <div className="flex items-center gap-2">
                  <Activity className="size-4 text-chart-2" />
                  <span className="font-semibold tabular-nums">{global.active}</span> in a match
                </div>
                <div className="flex items-center gap-2">
                  <DoorOpen className="size-4 text-chart-3" />
                  <span className="font-semibold tabular-nums">{rooms.length}</span> open rooms
                </div>
                <div className="flex items-center gap-2 text-muted-foreground">
                  <RefreshCw className="size-3.5" />
                  {updated ? `updated ${updated.toLocaleTimeString()}` : "loading…"}
                  <Button variant="link" size="sm" className="h-auto p-0" onClick={() => void load()} disabled={loading}>
                    {loading ? <Spinner /> : "refresh"}
                  </Button>
                </div>
              </div>
              {myRoom && (
                <Item variant="outline" className="max-w-lg" asChild>
                  <button type="button" className="text-left" onClick={() => setOpenRoomId(myRoom.id)}>
                    <ItemMedia>
                      <Gamepad2 className="size-5 text-primary" />
                    </ItemMedia>
                    <ItemContent>
                      <ItemTitle>You are in a room</ItemTitle>
                      <ItemDescription>
                        {myRoom.game} · {roomType(myRoom)} · {Object.keys(myRoom.players).length} players
                      </ItemDescription>
                    </ItemContent>
                    <ItemActions>
                      <Badge>Open</Badge>
                    </ItemActions>
                  </button>
                </Item>
              )}
            </div>
            <div className="rounded-xl border bg-card p-5">
              <p className="mb-4 text-xs font-medium tracking-wide text-muted-foreground uppercase">Most played now</p>
              {loading ? <Skeleton className="h-40" /> : <TopGames stats={stats} />}
            </div>
          </div>
        </section>
      )}

      <main className="mx-auto grid max-w-6xl gap-6 px-4 py-8">
        {tab === "rooms" && (
          <section className="grid gap-4">
            <div className="grid gap-1">
              <h2 className="text-xl font-semibold tracking-tight">Rooms</h2>
              <p className="text-sm text-muted-foreground">Matchmaking rooms open right now. Click one to see who is inside.</p>
            </div>
            <RoomFilterToggle value={roomFilter} onChange={setRoomFilter} />
            {visibleRooms.length ? (
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {visibleRooms.map((room) => (
                  <RoomTileView key={room.id} room={room} account={account} onOpen={() => setOpenRoomId(room.id)} />
                ))}
              </div>
            ) : (
              <Empty className="border py-16">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <DoorOpen />
                  </EmptyMedia>
                  <EmptyTitle>No rooms open</EmptyTitle>
                  <EmptyDescription>Rooms show up here as soon as players start matchmaking.</EmptyDescription>
                </EmptyHeader>
              </Empty>
            )}
                    </section>
        )}
        {tab === "players" && (
          <section className="grid gap-4">
            <div className="grid gap-1">
              <h2 className="text-xl font-semibold tracking-tight">Players</h2>
              <p className="text-sm text-muted-foreground">Everyone connected to the service.</p>
            </div>
            <DataTable
              columns={visibleUserColumns}
              data={users}
              loading={loading}
              globalFilter={userQuery}
              pageSize={settings.pageSize}
              emptyTitle="Nobody is online"
              toolbar={<SearchInput value={userQuery} onChange={setUserQuery} placeholder="Name, friend code or game" />}
            />
                    </section>
        )}
        {tab === "bans" && (
          <section className="grid gap-4">
            <div className="grid gap-1">
              <h2 className="text-xl font-semibold tracking-tight">Ban list</h2>
              <p className="text-sm text-muted-foreground">Public bans and room kicks.</p>
            </div>
            <DataTable
              columns={banColumns}
              data={visibleBans}
              loading={loading}
              pageSize={settings.pageSize}
              emptyTitle="No bans found"
              toolbar={
                <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                  <SearchInput value={banQuery} onChange={setBanQuery} placeholder="Name, friend code, reason" />
                  <BanFilterToggle value={banFilter} onChange={setBanFilter} showMine={!!account?.fc} />
                </div>
              }
            />
                    </section>
        )}
      </main>

      <Dialog open={!!openRoomId} onOpenChange={(o) => !o && setOpenRoomId(null)}>
        <DialogContent className="flex max-h-[85vh] flex-col sm:max-w-lg">
          {openRoom ? (
            <>
              <DialogHeader>
                <DialogTitle className="flex items-center gap-2">
                  {roomType(openRoom) === "private" ? <Lock className="size-4 text-warning" /> : <Globe className="size-4 text-chart-1" />}
                  {openRoom.game}
                </DialogTitle>
                <DialogDescription className="font-mono">{openRoom.id}</DialogDescription>
                <div className="flex flex-wrap gap-1.5 pt-1">
                  <StatusBadge tone={roomType(openRoom) === "private" ? "warning" : "info"}>{roomType(openRoom)}</StatusBadge>
                  <StatusBadge tone={stateTone(openRoom.state)}>{roomStateLabel(openRoom.state)}</StatusBadge>
                  {openRoom.rk && <Badge variant="outline">{openRoom.rk}</Badge>}
                </div>
              </DialogHeader>
              <ItemGroup className="min-h-0 gap-1.5 overflow-y-auto">
                {sortedPlayers(openRoom, account).map(([slot, p]) => {
                  const self = playerMatchesAccount(p, account)
                  const host = !!openRoom.host_pid && String(openRoom.host_pid) === String(p.pid ?? "")
                  const link = p.fc ? profileLinks.get(p.fc) : undefined
                  return (
                    <Item key={slot} variant={self ? "muted" : "outline"} size="sm">
                      <ItemMedia>
                        <UserAvatar name={playerName(p)} src={link?.avatar_url} className="size-8" />
                      </ItemMedia>
                      <ItemContent className="min-w-0">
                        <ItemTitle className="flex items-center gap-1.5">
                          <span className="truncate">{playerName(p)}</span>
                          {host && <Crown className="size-3.5 text-warning" aria-label="Host" />}
                          {self && <Badge variant="secondary">You</Badge>}
                          {link && (
                            <a href={profileHref(link.username)} className="truncate text-xs font-normal text-muted-foreground underline-offset-4 hover:underline">
                              @{link.username}
                            </a>
                          )}
                        </ItemTitle>
                        <ItemDescription className="font-mono text-xs">
                          {p.fc ? `FC ${p.fc} · ` : ""}
                          {playerStateLabel(p.state)}
                        </ItemDescription>
                      </ItemContent>
                      <ItemActions>
                        {canReport(openRoom, p, account) && (
                          <Button size="xs" variant="outline" onClick={() => void report(openRoom, p)}>
                            <Flag data-icon="inline-start" /> Report
                          </Button>
                        )}
                        {canKick(openRoom, p, account) && (
                          <Button size="xs" variant="destructive" onClick={() => setKickTarget({ room: openRoom, player: p })}>
                            Kick
                          </Button>
                        )}
                      </ItemActions>
                    </Item>
                  )
                })}
              </ItemGroup>
              {!account && <p className="text-xs text-muted-foreground">Log in and link your console to report players in your room.</p>}
            </>
          ) : (
            <DialogHeader>
              <DialogTitle>Room closed</DialogTitle>
              <DialogDescription>This room no longer exists.</DialogDescription>
            </DialogHeader>
          )}
        </DialogContent>
      </Dialog>
      <PrivateKickDialog target={kickTarget} onClose={() => setKickTarget(null)} onDone={() => void load()} />
    </div>
  )
}
