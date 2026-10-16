import { Download, Search, Trash2, Trophy } from "lucide-react"
import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { useConfirm } from "@/components/confirm"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import { InputGroup, InputGroupAddon, InputGroupButton, InputGroupInput } from "@/components/ui/input-group"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemMedia, ItemTitle } from "@/components/ui/item"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { api, errorMessage, list, post, type MKWRanking } from "@/lib/api"
import { relativeTime } from "@/lib/format"
import { MKW_CUPS, MKW_REGIONS, courseName, formatScore, regionName } from "@/lib/mkw"
import { cn } from "@/lib/utils"
import { usePanel } from "@/pages/panel/panel-context"

const medal = ["bg-yellow-500/15 text-yellow-500 border-yellow-500/30", "bg-zinc-400/15 text-zinc-400 border-zinc-400/30", "bg-orange-600/15 text-orange-500 border-orange-500/30"]

function RankingRow({ r, rank, showCourse, onDelete }: { r: MKWRanking; rank?: number; showCourse?: boolean; onDelete: (r: MKWRanking) => void }) {
  const { openPlayer } = usePanel()
  return (
    <Item variant="outline" size="sm">
      {rank !== undefined && (
        <ItemMedia>
          <span className={cn("flex size-9 items-center justify-center rounded-lg border font-mono text-sm font-bold", medal[rank - 1] ?? "text-muted-foreground")}>{rank}</span>
        </ItemMedia>
      )}
      <ItemContent className="min-w-0">
        <ItemTitle className="font-mono text-base">{formatScore(r.score)}</ItemTitle>
        <ItemDescription className="flex flex-wrap items-center gap-x-2">
          {showCourse && <span className="font-sans font-medium text-foreground">{courseName(r.courseid)}</span>}
          <button type="button" className="font-mono text-xs hover:underline" onClick={() => openPlayer({ pid: r.pid, fc: r.fc })}>
            {r.fc || `PID ${r.pid}`}
          </button>
          <span className="text-xs">{regionName(r.regionid)}</span>
          {r.upload_time && <span className="text-xs">{relativeTime(r.upload_time)}</span>}
        </ItemDescription>
      </ItemContent>
      <ItemActions>
        {r.has_ghost ? (
          <Button asChild size="sm" variant="outline">
            <a href={`/api/panel/mkw/ghost?id=${encodeURIComponent(r.id)}`} download>
              <Download data-icon="inline-start" /> {(r.ghost_size / 1024).toFixed(1)} KB
            </a>
          </Button>
        ) : (
          <Badge variant="outline">no ghost</Badge>
        )}
        <Button size="icon-sm" variant="ghost" className="text-destructive" aria-label="Delete ranking" onClick={() => onDelete(r)}>
          <Trash2 />
        </Button>
      </ItemActions>
    </Item>
  )
}

export function GhostsSection() {
  const { confirm } = useConfirm()
  const [cup, setCup] = useState(0)
  const [course, setCourse] = useState(MKW_CUPS[0].courses[0])
  const [region, setRegion] = useState("0")
  const [top, setTop] = useState<MKWRanking[] | null>(null)
  const [identifier, setIdentifier] = useState("")
  const [profile, setProfile] = useState<{ pid: number; rankings: MKWRanking[] } | null>(null)
  const [searching, setSearching] = useState(false)

  const loadTop = useCallback(async () => {
    setTop(null)
    try {
      const response = await api<{ rankings: MKWRanking[] }>(`/api/panel/mkw/top10?region=${region}&course=${course}`)
      setTop(list(response.rankings))
    } catch (error) {
      toast.error(errorMessage(error))
      setTop([])
    }
  }, [course, region])
  useEffect(() => {
    void loadTop()
  }, [loadTop])

  async function remove(r: MKWRanking) {
    const ok = await confirm({ title: `Delete this ${formatScore(r.score)} time?`, description: `${courseName(r.courseid)} · PID ${r.pid}. The ghost is deleted too.`, confirmLabel: "Delete", destructive: true })
    if (!ok) return
    try {
      await post("/api/panel/mkw/delete-ranking", { id: r.id })
      toast.success("Ranking deleted")
      setTop((rows) => rows?.filter((x) => x.id !== r.id) ?? null)
      setProfile((p) => (p ? { ...p, rankings: p.rankings.filter((x) => x.id !== r.id) } : p))
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  async function search(event: React.FormEvent) {
    event.preventDefault()
    if (!identifier.trim()) return
    setSearching(true)
    try {
      const response = await api<{ pid: number; rankings: MKWRanking[] }>(`/api/panel/mkw/search?identifier=${encodeURIComponent(identifier.trim())}`)
      setProfile({ pid: response.pid, rankings: list(response.rankings).sort((a, b) => a.courseid - b.courseid) })
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setSearching(false)
    }
  }

  async function clearProfile() {
    const target = identifier.trim()
    if (!target) return
    const ok = await confirm({ title: `Wipe every time trial of ${target}?`, description: "Deletes all rankings and ghosts of this profile. This cannot be undone.", confirmLabel: "Wipe profile", destructive: true })
    if (!ok) return
    try {
      const response = await post<{ deleted: number; profile_id: number }>("/api/panel/mkw/clear-profile", { identifier: target })
      toast.success(`Deleted ${response.deleted} rankings for profile ${response.profile_id}`)
      setProfile(null)
      void loadTop()
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  return (
    <div className="grid gap-6 2xl:grid-cols-[1.3fr_1fr]">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Trophy className="size-4 text-warning" /> Time trial leaderboard
          </CardTitle>
          <CardDescription>Pick a cup and a track. The top 10 loads instantly.</CardDescription>
          <CardAction>{top === null && <Spinner />}</CardAction>
        </CardHeader>
        <CardContent className="grid gap-4">
          <form onSubmit={search} className="grid gap-1.5">
            <label htmlFor="ghost-player-search" className="text-sm font-medium">Search player times</label>
            <InputGroup>
              <InputGroupInput id="ghost-player-search" placeholder="Friend code or profile ID" value={identifier} onChange={(event) => setIdentifier(event.target.value)} />
              <InputGroupAddon align="inline-end">
                <InputGroupButton type="submit" disabled={searching || !identifier.trim()}>
                  {searching ? <Spinner /> : <Search />} Search
                </InputGroupButton>
              </InputGroupAddon>
            </InputGroup>
          </form>
          <Tabs
            value={String(cup)}
            onValueChange={(v) => {
              setCup(Number(v))
              setCourse(MKW_CUPS[Number(v)].courses[0])
            }}
          >
            <TabsList className="h-auto w-full flex-wrap">
              {MKW_CUPS.map((c, i) => (
                <TabsTrigger key={c.name} value={String(i)} className="flex-1">
                  {c.name}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
          <div className="grid grid-cols-2 gap-2 lg:grid-cols-4">
            {MKW_CUPS[cup].courses.map((id) => (
              <Button key={id} variant={course === id ? "default" : "outline"} className="h-auto justify-start py-2.5 text-left whitespace-normal" onClick={() => setCourse(id)}>
                {courseName(id)}
              </Button>
            ))}
          </div>
          <ToggleGroup type="single" variant="outline" size="sm" value={region} onValueChange={(v) => v && setRegion(v)} className="flex-wrap justify-start">
            {MKW_REGIONS.map((name, i) => (
              <ToggleGroupItem key={name} value={String(i)} className="px-3">
                {name}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          {top === null ? (
            <div className="grid gap-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-14" />
              ))}
            </div>
          ) : top.length ? (
            <ItemGroup className="gap-1.5">
              {top.map((r, i) => (
                <RankingRow key={r.id} r={r} rank={i + 1} onDelete={(x) => void remove(x)} />
              ))}
            </ItemGroup>
          ) : (
            <Empty className="border py-10">
              <EmptyHeader>
                <EmptyTitle>No times yet</EmptyTitle>
                <EmptyDescription>
                  Nobody uploaded a time on {courseName(course)} in {regionName(Number(region))}.
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          )}
        </CardContent>
      </Card>

      <Card className="h-fit">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Search className="size-4" /> Player times
          </CardTitle>
          <CardDescription>Every ranking of the selected profile, with ghost downloads.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4">
          {!profile && <p className="text-sm text-muted-foreground">Search by friend code or profile ID in the leaderboard to see all player times.</p>}
          {profile && (
            <>
              <div className="flex items-center justify-between">
                <p className="text-sm text-muted-foreground">
                  <span className="font-semibold text-foreground">{profile.rankings.length}</span> times for PID <span className="font-mono">{profile.pid}</span>
                </p>
                <Button size="sm" variant="destructive" onClick={() => void clearProfile()}>
                  Wipe profile
                </Button>
              </div>
              {profile.rankings.length ? (
                <ItemGroup className="gap-1.5">
                  {profile.rankings.map((r) => (
                    <RankingRow key={r.id} r={r} showCourse onDelete={(x) => void remove(x)} />
                  ))}
                </ItemGroup>
              ) : (
                <p className="text-sm text-muted-foreground">No rankings for this profile.</p>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
