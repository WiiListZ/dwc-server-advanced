import { Check, ChevronsUpDown, Gamepad2, Pencil, Plus, Power, Undo2 } from "lucide-react"
import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"

import { useConfirm } from "@/components/confirm"
import { SearchInput } from "@/components/filters"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/components/ui/command"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Field, FieldContent, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Switch } from "@/components/ui/switch"
import { Input } from "@/components/ui/input"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { Textarea } from "@/components/ui/textarea"
import { api, errorMessage, list, type DisabledGame } from "@/lib/api"
import { relativeTime } from "@/lib/format"
import { cn } from "@/lib/utils"

type GameEntry = { name: string; description: string }
type FormState = { id: number; gameCodePrefix: string; gameName: string; nasReturnCode: string; errorCode: string; message: string }
const EMPTY: FormState = { id: 0, gameCodePrefix: "", gameName: "", nasReturnCode: "110", errorCode: "", message: "" }

function GamePicker({ games, value, onChange }: { games: GameEntry[]; value: string; onChange: (name: string) => void }) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")
  const selected = games.find((g) => g.name === value)
  const matches = useMemo(() => {
    const q = search.trim().toLowerCase()
    const hits = q ? games.filter((g) => g.name.toLowerCase().includes(q) || g.description.toLowerCase().includes(q)) : games
    return hits.slice(0, 50)
  }, [games, search])
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button type="button" variant="outline" role="combobox" aria-expanded={open} className="w-full justify-between font-normal">
          <span className={cn("truncate", !value && "text-muted-foreground")}>{selected ? `${selected.description} (${selected.name})` : value || "Search game_list.tsv…"}</span>
          <ChevronsUpDown className="opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-(--radix-popover-trigger-width) p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput placeholder="Game title or GameSpy name" value={search} onValueChange={setSearch} />
          <CommandList>
            <CommandEmpty>No game found.</CommandEmpty>
            <CommandGroup>
              {value && (
                <CommandItem
                  value="__clear"
                  onSelect={() => {
                    onChange("")
                    setOpen(false)
                  }}
                >
                  <span className="text-muted-foreground">No GameSpy name (match by code only)</span>
                </CommandItem>
              )}
              {matches.map((g) => (
                <CommandItem
                  key={g.name}
                  value={`${g.description} ${g.name}`}
                  onSelect={() => {
                    onChange(g.name)
                    setOpen(false)
                  }}
                >
                  <Check className={cn(g.name === value ? "opacity-100" : "opacity-0")} />
                  <span className="min-w-0 flex-1 truncate">{g.description}</span>
                  <span className="shrink-0 font-mono text-xs text-muted-foreground">{g.name}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

function GameDialog({ form, games, onClose, onSaved }: { form: FormState | null; games: GameEntry[]; onClose: () => void; onSaved: () => void }) {
  const [state, setState] = useState<FormState>(EMPTY)
  const [busy, setBusy] = useState(false)
  // Game codes are 4 characters and the last one is the region (RMCP = PAL).
  // With "all regions" on, a full code is stored as its 3-character prefix.
  const [allRegions, setAllRegions] = useState(true)
  useEffect(() => {
    if (!form) return
    setState(form)
    setAllRegions(form.gameCodePrefix.trim().length !== 4)
  }, [form])
  const code = state.gameCodePrefix.trim().toUpperCase()
  const prefix = allRegions && code.length === 4 ? code.slice(0, 3) : code
  const editing = state.id > 0
  const set = (key: keyof FormState) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => setState((s) => ({ ...s, [key]: e.target.value }))

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    if (state.errorCode && !/^\d+$/.test(state.errorCode)) return void toast.error("Error code must be a number")
    setBusy(true)
    try {
      const saved = await api<DisabledGame & { kicked?: { pid: number; name?: string }[] }>("/api/panel/disabled-games", {
        method: editing ? "PATCH" : "POST",
        body: { ...state, gameCodePrefix: prefix, gameName: state.gameName.trim(), nasReturnCode: state.nasReturnCode.trim(), errorCode: Number(state.errorCode || 0) },
      })
      const kicked = saved.kicked?.length ?? 0
      toast.success(`${saved.gameName || saved.gameCodePrefix} ${editing ? "updated" : "disabled"}`, {
        description: kicked ? `${kicked} online player${kicked === 1 ? " was" : "s were"} disconnected.` : "Nobody was playing it.",
      })
      onSaved()
      onClose()
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={!!form} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit} className="grid gap-5">
          <DialogHeader>
            <DialogTitle>{editing ? "Edit disabled game" : "Disable a game"}</DialogTitle>
            <DialogDescription>Rejected at NAS login and again at GPCM login. Players online with this game are disconnected right away.</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel>Game</FieldLabel>
              <GamePicker games={games} value={state.gameName} onChange={(gameName) => setState((s) => ({ ...s, gameName }))} />
              <FieldDescription>Used by GPCM. Leave empty to block by game code only.</FieldDescription>
            </Field>
            <div className="grid gap-4 sm:grid-cols-3">
              <Field>
                <FieldLabel htmlFor="g-prefix">Game code</FieldLabel>
                <Input id="g-prefix" maxLength={4} placeholder="RMCP" className="font-mono uppercase" value={state.gameCodePrefix} onChange={set("gameCodePrefix")} />
              </Field>
              <Field>
                <FieldLabel htmlFor="g-nas">NAS code</FieldLabel>
                <Input id="g-nas" maxLength={3} inputMode="numeric" className="font-mono" value={state.nasReturnCode} onChange={set("nasReturnCode")} />
              </Field>
              <Field>
                <FieldLabel htmlFor="g-err">Error code</FieldLabel>
                <Input id="g-err" inputMode="numeric" placeholder={`20${state.nasReturnCode || "110"}`} className="font-mono" value={state.errorCode} onChange={set("errorCode")} />
              </Field>
            </div>
            <Field orientation="horizontal" className="-mt-2 justify-between rounded-lg border p-3">
              <FieldContent>
                <FieldLabel htmlFor="g-all-regions">All regions</FieldLabel>
                <FieldDescription>
                  {!prefix
                    ? "Enter a game code to block it by code, e.g. RMCP or RMC."
                    : prefix.length === 3
                      ? `No region given: blocks every region of ${prefix} (${["E", "P", "J", "K"].map((r) => prefix + r).join(", ")}…).`
                      : prefix.length < 3
                        ? `Blocks every game code starting with ${prefix}.`
                        : `Blocks only ${prefix}. Turn on to block every region of this game.`}
                </FieldDescription>
              </FieldContent>
              <Switch id="g-all-regions" checked={code.length < 4 || allRegions} disabled={code.length < 4} onCheckedChange={setAllRegions} />
            </Field>
            <FieldDescription className="-mt-3">On screen the NAS code shows as 20XXX.</FieldDescription>
            <Field>
              <FieldLabel htmlFor="g-msg">Message</FieldLabel>
              <Textarea id="g-msg" maxLength={500} placeholder="The Nintendo WFC service for this software has been discontinued." value={state.message} onChange={set("message")} />
              <FieldDescription>Shown to payload clients. Line breaks are kept.</FieldDescription>
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" variant={editing ? "default" : "destructive"} disabled={busy || (!state.gameCodePrefix.trim() && !state.gameName.trim())}>
              {busy ? <Spinner /> : <Power data-icon="inline-start" />} {editing ? "Save changes" : "Disable game"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export function GamesSection() {
  const { confirm } = useConfirm()
  const [disabled, setDisabled] = useState<DisabledGame[] | null>(null)
  const [games, setGames] = useState<GameEntry[]>([])
  const [form, setForm] = useState<FormState | null>(null)
  const [query, setQuery] = useState("")

  const load = useCallback(async () => {
    try {
      setDisabled(list(await api<DisabledGame[]>("/api/panel/disabled-games")))
    } catch (error) {
      toast.error(errorMessage(error))
      setDisabled([])
    }
  }, [])
  useEffect(() => {
    void load()
    api<GameEntry[]>("/api/panel/game-list")
      .then((rows) => setGames(list(rows)))
      .catch(() => setGames([]))
  }, [load])

  const titles = useMemo(() => new Map(games.map((g) => [g.name, g.description])), [games])
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    return (disabled ?? []).filter((g) => !q || [g.gameName, g.gameCodePrefix, titles.get(g.gameName)].some((v) => String(v ?? "").toLowerCase().includes(q)))
  }, [disabled, query, titles])

  async function enable(game: DisabledGame) {
    const label = titles.get(game.gameName) || game.gameName || game.gameCodePrefix
    const ok = await confirm({ title: `Re-enable ${label}?`, description: "Players can log in with this game again right away.", confirmLabel: "Re-enable" })
    if (!ok) return
    try {
      await api(`/api/panel/disabled-games?id=${encodeURIComponent(game.id)}`, { method: "DELETE" })
      toast.success(`${label} re-enabled`)
      void load()
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <SearchInput value={query} onChange={setQuery} placeholder="Filter disabled games" />
        <Button variant="destructive" onClick={() => setForm(EMPTY)}>
          <Plus data-icon="inline-start" /> Disable a game
        </Button>
      </div>
      {disabled === null ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-48" />
          ))}
        </div>
      ) : disabled.length === 0 ? (
        <Empty className="border py-20">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Gamepad2 />
            </EmptyMedia>
            <EmptyTitle>Every game is online</EmptyTitle>
            <EmptyDescription>No game is blocked. Disable one to reject its logins with a custom error code.</EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button variant="outline" onClick={() => setForm(EMPTY)}>
              <Plus data-icon="inline-start" /> Disable a game
            </Button>
          </EmptyContent>
        </Empty>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {visible.map((g) => (
            <Card key={g.id} className="gap-4">
              <CardHeader>
                <CardTitle className="truncate">{titles.get(g.gameName) || g.gameName || `Game code ${g.gameCodePrefix}*`}</CardTitle>
                <CardDescription className="flex flex-wrap gap-1.5 pt-1">
                  {g.gameName && <Badge variant="secondary" className="font-mono">{g.gameName}</Badge>}
                  {g.gameCodePrefix && <Badge variant="outline" className="font-mono">{g.gameCodePrefix}*</Badge>}
                </CardDescription>
                <CardAction>
                  <Badge variant="outline" className="gap-1">
                    <Power className="size-3" /> Off
                  </Badge>
                </CardAction>
              </CardHeader>
              <CardContent className="grid gap-3">
                <div className="grid grid-cols-2 gap-2 text-center">
                  <div className="rounded-md bg-muted/50 p-2">
                    <p className="font-mono text-lg font-semibold">20{g.nasReturnCode}</p>
                    <p className="text-[11px] text-muted-foreground">shown on screen</p>
                  </div>
                  <div className="rounded-md bg-muted/50 p-2">
                    <p className="font-mono text-lg font-semibold">{g.errorCode}</p>
                    <p className="text-[11px] text-muted-foreground">payload error</p>
                  </div>
                </div>
                {g.message && <p className="line-clamp-3 rounded-md border border-dashed p-2 text-sm whitespace-pre-line text-muted-foreground">{g.message}</p>}
              </CardContent>
              <CardFooter className="mt-auto flex items-center justify-between gap-2">
                <span className="text-xs text-muted-foreground">
                  {relativeTime(g.updated)}
                  {g.moderator ? ` · ${g.moderator}` : ""}
                </span>
                <div className="flex gap-1">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setForm({ id: g.id, gameCodePrefix: g.gameCodePrefix, gameName: g.gameName, nasReturnCode: g.nasReturnCode, errorCode: String(g.errorCode || ""), message: g.message })}
                  >
                    <Pencil data-icon="inline-start" /> Edit
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => void enable(g)}>
                    <Undo2 data-icon="inline-start" /> Enable
                  </Button>
                </div>
              </CardFooter>
            </Card>
          ))}
        </div>
      )}
      <GameDialog form={form} games={games} onClose={() => setForm(null)} onSaved={() => void load()} />
    </div>
  )
}
