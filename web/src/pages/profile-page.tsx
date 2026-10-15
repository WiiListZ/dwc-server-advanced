import { CalendarDays, EyeOff, Gamepad2, MoreHorizontal, Pencil, Timer, Trophy, Users } from "lucide-react"
import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { useConfirm } from "@/components/confirm"
import { SiteHeader } from "@/components/site-header"
import { UserAvatar } from "@/components/user-avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { ApiError, api, errorMessage, post, type PublicProfile } from "@/lib/api"
import { relativeTime, roleLabel, roomStateLabel } from "@/lib/format"
import { useInterval, useSession } from "@/lib/hooks"

function usernameFromPath() {
  const raw = window.location.pathname.replace(/^\/u\//, "").replace(/\/$/, "")
  try {
    return decodeURIComponent(raw)
  } catch {
    return raw
  }
}

function StatusLine({ status, linked }: { status: PublicProfile["status"]; linked: boolean }) {
  if (!linked) return <p className="text-sm text-muted-foreground">No console linked yet.</p>
  if (!status.online) {
    return (
      <p className="flex items-center gap-2 text-sm text-muted-foreground">
        <span className="size-2 rounded-full bg-muted-foreground/40" aria-hidden /> Offline
      </p>
    )
  }
  const room = status.room
  return (
    <div className="grid gap-1 text-sm">
      <p className="flex items-center gap-2 font-medium">
        <span className="size-2 rounded-full bg-foreground" aria-hidden />
        Playing {status.game || status.game_code}
        {status.name && <span className="font-normal text-muted-foreground">as {status.name}</span>}
      </p>
      <p className="pl-4 text-muted-foreground">
        {room
          ? `${room.host ? "Hosting" : "In"} a ${room.type === "private" ? "private" : "public"} room · ${room.players} player${room.players === 1 ? "" : "s"}${room.state ? ` · ${roomStateLabel(room.state)}` : ""}${room.rk ? ` · ${room.rk}` : ""}`
          : "In the menus, not in a room"}
      </p>
    </div>
  )
}

function Stat({ icon: Icon, label, value, hint }: { icon: typeof Trophy; label: string; value: React.ReactNode; hint?: string }) {
  return (
    <div className="grid gap-1 rounded-lg border p-4">
      <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Icon className="size-3.5" /> {label}
      </span>
      <span className="text-2xl font-semibold tracking-tight tabular-nums">{value}</span>
      {hint && <span className="text-xs text-muted-foreground">{hint}</span>}
    </div>
  )
}

export function ProfilePage() {
  const { session } = useSession()
  const { confirm } = useConfirm()
  const [username] = useState(usernameFromPath)
  const [profile, setProfile] = useState<PublicProfile | null>(null)
  const [missing, setMissing] = useState(false)

  const load = useCallback(async () => {
    try {
      setProfile(await api<PublicProfile>(`/api/public/profiles/${encodeURIComponent(username)}`))
    } catch (error) {
      if (error instanceof ApiError && error.status === 404) setMissing(true)
      else toast.error(errorMessage(error))
    }
  }, [username])

  useEffect(() => {
    void load()
  }, [load])
  useInterval(() => void load(), missing ? null : 15000)

  useEffect(() => {
    if (profile) document.title = `${profile.username} · ${document.title.split(" · ").pop()}`
  }, [profile])

  async function moderate(clear: "avatar" | "bio") {
    if (!profile) return
    const ok = await confirm({
      title: clear === "avatar" ? `Remove ${profile.username}'s avatar?` : `Clear ${profile.username}'s bio?`,
      description: "The user can set a new one afterwards.",
      confirmLabel: clear === "avatar" ? "Remove avatar" : "Clear bio",
      destructive: true,
    })
    if (!ok) return
    try {
      await post("/api/panel/profile-moderation", { username: profile.username, clear })
      toast.success(clear === "avatar" ? "Avatar removed" : "Bio cleared")
      void load()
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  const stats = profile?.stats

  return (
    <div className="min-h-svh bg-background">
      <SiteHeader session={session} page="profile" subtitle="Profile" />
      <main className="mx-auto grid max-w-3xl gap-6 px-4 py-10">
        {missing ? (
          <Empty className="border">
            <EmptyHeader>
              <EmptyTitle>Profile not found</EmptyTitle>
              <EmptyDescription>There is no account called {username}.</EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : !profile ? (
          <>
            <div className="flex items-center gap-5">
              <Skeleton className="size-24 rounded-full" />
              <div className="grid gap-2">
                <Skeleton className="h-8 w-48" />
                <Skeleton className="h-4 w-32" />
              </div>
            </div>
            <Skeleton className="h-40 w-full" />
          </>
        ) : (
          <>
            <section className="flex flex-col gap-5 sm:flex-row sm:items-center">
              <UserAvatar name={profile.username} src={profile.avatar_url} className="size-24" fallbackClassName="text-2xl" />
              <div className="grid min-w-0 flex-1 gap-2">
                <div className="flex flex-wrap items-center gap-2">
                  <h1 className="truncate text-3xl font-semibold tracking-tight">{profile.username}</h1>
                  {profile.role !== "player" && <Badge variant="secondary">{roleLabel(profile.role)}</Badge>}
                </div>
                <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
                  <CalendarDays className="size-3.5" /> Member since {new Date(profile.created).toLocaleDateString("en", { year: "numeric", month: "long" })}
                </p>
              </div>
              <div className="flex items-center gap-2">
                {profile.is_own && (
                  <Button asChild variant="outline" size="sm">
                    <a href="/settings">
                      <Pencil data-icon="inline-start" /> Edit profile
                    </a>
                  </Button>
                )}
                {profile.can_moderate && !profile.is_own && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="outline" size="icon-sm" aria-label="Moderate profile">
                        <MoreHorizontal />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuLabel>Moderation</DropdownMenuLabel>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem variant="destructive" disabled={!profile.avatar_url} onSelect={() => void moderate("avatar")}>
                        Remove avatar
                      </DropdownMenuItem>
                      <DropdownMenuItem variant="destructive" disabled={!profile.bio} onSelect={() => void moderate("bio")}>
                        Clear bio
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>
            </section>

            <Card>
              <CardContent className="grid gap-5">
                {profile.bio ? (
                  <p className="text-sm leading-relaxed break-words whitespace-pre-line">{profile.bio}</p>
                ) : (
                  <p className="text-sm text-muted-foreground">{profile.is_own ? "You have not written a bio yet." : "No bio yet."}</p>
                )}
                <Separator />
                <StatusLine status={profile.status} linked={profile.linked} />
              </CardContent>
            </Card>

            {stats ? (
              <Card>
                <CardHeader>
                  <CardTitle>Stats</CardTitle>
                  <CardDescription>
                    From the matches the server has logged for this account's consoles.
                    {profile.is_own && !profile.show_stats && " Hidden from everyone else: only you can see them."}
                  </CardDescription>
                </CardHeader>
                <CardContent className="grid gap-5">
                  <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                    <Stat icon={Users} label="Matches" value={stats.matches} hint={stats.last_match ? `last ${relativeTime(stats.last_match)}` : "none logged"} />
                    <Stat icon={Trophy} label="Versus rating" value={stats.vr ?? "-"} hint="Mario Kart Wii" />
                    <Stat icon={Trophy} label="Battle rating" value={stats.br ?? "-"} hint="Mario Kart Wii" />
                    <Stat icon={Timer} label="Time trials" value={stats.time_trials} hint="ghosts uploaded" />
                  </div>
                  {(stats.games.length > 0 || (profile.profiles?.length ?? 0) > 0) && (
                    <div className="grid gap-4 sm:grid-cols-2">
                      {stats.games.length > 0 && (
                        <div className="grid gap-2">
                          <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
                            <Gamepad2 className="size-3.5" /> Games played
                          </span>
                          <div className="flex flex-wrap gap-1.5">
                            {stats.games.map((game) => (
                              <Badge key={game} variant="outline">
                                {game}
                              </Badge>
                            ))}
                          </div>
                        </div>
                      )}
                      {(profile.profiles?.length ?? 0) > 0 && (
                        <div className="grid gap-2">
                          <span className="text-xs text-muted-foreground">Game profiles</span>
                          <ul className="grid gap-1 text-sm">
                            {profile.profiles!.map((p, i) => (
                              <li key={i} className="flex items-center justify-between gap-3">
                                <span className="truncate">{p.name || "Unnamed"}</span>
                                {p.fc && <span className="font-mono text-xs text-muted-foreground">{p.fc}</span>}
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                    </div>
                  )}
                </CardContent>
              </Card>
            ) : (
              !profile.show_stats && (
                <p className="flex items-center gap-2 text-sm text-muted-foreground">
                  <EyeOff className="size-4" /> {profile.username} keeps their stats private.
                </p>
              )
            )}
          </>
        )}
      </main>
    </div>
  )
}
