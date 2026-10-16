import { Check, ChevronDown, Gamepad2, Link2, Monitor, Pencil, ShieldCheck, SlidersHorizontal, Unlink, UserRound } from "lucide-react"
import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { useConfirm } from "@/components/confirm"
import { SiteHeader } from "@/components/site-header"
import { UserAvatar } from "@/components/user-avatar"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { StatusBadge } from "@/components/status-badge"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty"
import { Field, FieldContent, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Item, ItemActions, ItemContent, ItemDescription, ItemGroup, ItemTitle } from "@/components/ui/item"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { Switch } from "@/components/ui/switch"
import {
  api,
  errorMessage,
  list,
  post,
  type Account,
  type ConsoleSummary,
  type OpenHostMode,
  type OpenHostProfile,
  type Session,
} from "@/lib/api"
import { fmtDate, roleLabel } from "@/lib/format"
import { PAGE_SIZES, loadDisplaySettings, saveDisplaySettings, useSession } from "@/lib/hooks"
import { LOG_TYPES, RIGHT_GROUPS } from "@/lib/rights"
import { OpenHostProfileRow } from "@/pages/settings-open-host"
import { PublicProfileCard } from "@/pages/settings-profile"

const hasLinkedProfile = (account?: Account) => !!(account && (account.pid || account.linked_profiles?.length))

function RightsCard({ account }: { account: Account }) {
  const owner = account.role === "owner"
  const rights = account.rights
  const groups = RIGHT_GROUPS.map((group) => ({
    title: group.title,
    items: group.items.filter((item) => owner || item.keys.some((key) => rights?.[key])).map((item) => item.label),
  })).filter((group) => group.items.length)
  const logs = owner ? LOG_TYPES.map((t) => t.label) : LOG_TYPES.filter((t) => rights?.log_types?.includes(t.key)).map((t) => t.label)

  return (
    <Card>
      <CardHeader>
        <CardTitle>Account rights</CardTitle>
        <CardDescription>{owner ? "The owner always has every right." : "Rights granted to this account by the owner."}</CardDescription>
      </CardHeader>
      <CardContent>
        {groups.length ? (
          <div className="grid gap-5">
            {groups.map((group) => (
              <div key={group.title} className="grid content-start gap-2">
                <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">{group.title}</p>
                <ul className="grid content-start gap-1.5">
                  {group.items.map((label) => (
                    <li key={label} className="flex gap-2 text-sm">
                      <Check className="mt-0.5 size-4 shrink-0 text-success" /> {label}
                    </li>
                  ))}
                  {group.title === "Logs access" && (owner || rights?.view_logs) && (
                    <li className="text-xs text-muted-foreground">{logs.length ? `Visible log types: ${logs.join(", ")}` : "No log types visible."}</li>
                  )}
                </ul>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No special rights. Link a console to use Open Host and report players.</p>
        )}
      </CardContent>
    </Card>
  )
}

function ConsoleLinkCard({ session, onLinked }: { session: Session; onLinked: (account?: Account) => void }) {
  const { confirm } = useConfirm()
  const account = session.account!
  const [identifier, setIdentifier] = useState("")
  const [placeholder, setPlaceholder] = useState("")
  const [pending, setPending] = useState<{ pid: number; placeholder: string } | null>(null)
  const [code, setCode] = useState("")
  const [busy, setBusy] = useState(false)
  const linked = hasLinkedProfile(account)

  async function start(event: React.FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      const response = await post<{ pid: number; notice?: string }>("/api/panel/link/start", { identifier, placeholder })
      setPending({ pid: response.pid, placeholder })
      setCode("")
      toast.success(response.notice || "Link code sent", { description: "Log in on the console to read the code." })
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function complete(event: React.FormEvent) {
    event.preventDefault()
    if (!pending) return
    setBusy(true)
    try {
      const response = await post<{ account?: Account }>("/api/panel/link/complete", { pid: pending.pid, code, placeholder: pending.placeholder })
      setPending(null)
      setIdentifier("")
      setPlaceholder("")
      toast.success("Console linked")
      onLinked(response.account)
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function unlinkAll() {
    const ok = await confirm({
      title: "Unlink all consoles?",
      description: "Bans on the consoles stay applied.",
      confirmLabel: "Unlink all",
      destructive: true,
    })
    if (!ok) return
    try {
      const response = await post<{ account?: Account }>("/api/panel/link/unlink")
      toast.success("Consoles unlinked")
      onLinked(response.account)
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Link2 className="size-4" /> Console link
        </CardTitle>
        <CardDescription>
          Link one or more consoles by entering a code on any in-game profile on the console. Every profile on a linked console shares ban status.
        </CardDescription>
        <CardAction>{linked ? <StatusBadge tone="success">Linked</StatusBadge> : <Badge variant="outline">Not linked</Badge>}</CardAction>
      </CardHeader>
      <CardContent className="grid gap-6">
        {linked && (
          <Item variant="muted">
            <ItemContent>
              <ItemTitle>Console link active</ItemTitle>
              <ItemDescription className="font-mono text-xs">
                {[account.pid && `Primary PID ${account.pid}`, account.fc && `FC ${account.fc}`, account.linked_profiles?.length && `${account.linked_profiles.length} linked profiles`]
                  .filter(Boolean)
                  .join(" · ")}
              </ItemDescription>
            </ItemContent>
            <ItemActions>
              <Button variant="destructive" size="sm" onClick={() => void unlinkAll()}>
                <Unlink data-icon="inline-start" /> Unlink all
              </Button>
            </ItemActions>
          </Item>
        )}
        {pending ? (
          <form onSubmit={complete} className="grid gap-4">
            <FieldGroup className="sm:max-w-sm">
              <Field>
                <FieldLabel htmlFor="link-code">Link code</FieldLabel>
                <Input
                  id="link-code"
                  autoFocus
                  required
                  inputMode="numeric"
                  maxLength={6}
                  placeholder="000000"
                  className="font-mono tracking-[0.4em]"
                  value={code}
                  onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
                />
                <FieldDescription>Sent to profile {pending.pid}. Log in on the console to see it.</FieldDescription>
              </Field>
            </FieldGroup>
            <div className="flex gap-2">
              <Button type="submit" disabled={busy || code.length < 6}>
                {busy && <Spinner />} Link console
              </Button>
              <Button type="button" variant="outline" onClick={() => setPending(null)}>
                Cancel
              </Button>
            </div>
          </form>
        ) : (
          <form onSubmit={start} className="grid gap-4">
            <FieldGroup className="grid gap-4 sm:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="link-identifier">Friend code or profile ID</FieldLabel>
                <Input id="link-identifier" required placeholder="1234-5678-9012" value={identifier} onChange={(e) => setIdentifier(e.target.value)} />
              </Field>
              <Field>
                <FieldLabel htmlFor="link-placeholder">Console nickname</FieldLabel>
                <Input id="link-placeholder" maxLength={80} placeholder="Living room Wii" value={placeholder} onChange={(e) => setPlaceholder(e.target.value)} />
              </Field>
            </FieldGroup>
            <div>
              <Button type="submit" disabled={busy || !identifier.trim()}>
                {busy && <Spinner />} Send link code
              </Button>
            </div>
          </form>
        )}
      </CardContent>
    </Card>
  )
}

function consoleMeta(c: ConsoleSummary) {
  return [
    `ID ${c.id}`,
    c.type || "Wii",
    c.console_fc && `FC ${c.console_fc}`,
    `assigned ${fmtDate(c.assigned_at || c.linked_at, "-")}`,
    `last login ${fmtDate(c.last_login, "-")}`,
  ]
    .filter(Boolean)
    .join(" · ")
}

function ConsoleRow({ data, onChange, onUnlinked }: { data: ConsoleSummary; onChange: (c: ConsoleSummary[]) => void; onUnlinked: (a?: Account) => void }) {
  const { confirm } = useConfirm()
  const [name, setName] = useState(data.placeholder || "")
  const [renaming, setRenaming] = useState(false)
  const [savingName, setSavingName] = useState(false)
  const [open, setOpen] = useState(true)
  const profiles = list(data.profiles)
  const primary = profiles.find((p) => p.primary)

  async function saveName() {
    setSavingName(true)
    try {
      const response = await api<{ consoles: ConsoleSummary[] }>("/api/panel/console/me", { method: "PATCH", body: { console_id: data.id, placeholder: name.trim() } })
      toast.success("Console nickname saved")
      onChange(list(response.consoles))
      setRenaming(false)
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setSavingName(false)
    }
  }

  async function unlink() {
    const ok = await confirm({ title: `Unlink ${data.placeholder || `console ${data.id}`}?`, confirmLabel: "Unlink", destructive: true })
    if (!ok) return
    try {
      const response = await post<{ account?: Account }>("/api/panel/link/unlink", { console_id: data.id })
      toast.success("Console unlinked")
      onUnlinked(response.account)
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  async function setPrimary(pid: string) {
    try {
      const response = await post<{ consoles: ConsoleSummary[] }>("/api/panel/console/primary", { pid: Number(pid) })
      toast.success("Primary profile updated")
      onChange(list(response.consoles))
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="rounded-lg border">
      <div className="flex flex-wrap items-center gap-3 p-3">
        <CollapsibleTrigger asChild>
          <Button variant="ghost" size="icon-sm" aria-label="Toggle console">
            <ChevronDown className={open ? "" : "-rotate-90"} />
          </Button>
        </CollapsibleTrigger>
        <div className="grid min-w-0 flex-1">
          <span className="flex items-center gap-2 font-medium">
            <Monitor className="size-4 text-muted-foreground" /> {data.placeholder || `Console ${data.id}`}
            {data.has_ban && <StatusBadge tone="danger">banned</StatusBadge>}
          </span>
          <span className="truncate font-mono text-xs text-muted-foreground">{consoleMeta(data)}</span>
        </div>
        <div className="flex items-center gap-2">
          <Button size="icon-sm" variant="outline" aria-label="Rename console" title="Rename console" onClick={() => { setName(data.placeholder || ""); setRenaming(true) }}>
            <Pencil />
          </Button>
          <Button size="sm" variant="destructive" onClick={() => void unlink()}>
            Unlink
          </Button>
        </div>
      </div>
      <Dialog open={renaming} onOpenChange={setRenaming}>
        <DialogContent>
          <form className="grid gap-5" onSubmit={(event) => { event.preventDefault(); void saveName() }}>
            <DialogHeader>
              <DialogTitle>Rename console</DialogTitle>
              <DialogDescription>Choose a nickname for {data.placeholder || `console ${data.id}`}.</DialogDescription>
            </DialogHeader>
            <Field>
              <FieldLabel htmlFor={`console-name-${data.id}`}>Console nickname</FieldLabel>
              <Input id={`console-name-${data.id}`} maxLength={80} placeholder={`Console ${data.id}`} value={name} onChange={(event) => setName(event.target.value)} autoFocus />
            </Field>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setRenaming(false)}>Cancel</Button>
              <Button type="submit" disabled={savingName}>{savingName ? <Spinner /> : null} Save</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <CollapsibleContent>
        <Separator />
        <div className="p-3">
          {profiles.length ? (
            <RadioGroup value={primary ? String(primary.pid) : ""} onValueChange={(v) => void setPrimary(v)} className="gap-1.5">
              {profiles.map((p) => (
                <Label key={p.pid} htmlFor={`primary-${p.pid}`} className="flex items-center gap-3 rounded-md border p-3 font-normal has-data-[state=checked]:border-primary/50 has-data-[state=checked]:bg-muted/50">
                  <RadioGroupItem id={`primary-${p.pid}`} value={String(p.pid)} />
                  <div className="grid flex-1">
                    <span className="font-medium">{p.name || `PID ${p.pid}`}</span>
                    <span className="font-mono text-xs text-muted-foreground">{[`PID ${p.pid}`, p.fc && `FC ${p.fc}`].filter(Boolean).join(" · ")}</span>
                  </div>
                  {p.has_ban && <StatusBadge tone="danger">banned</StatusBadge>}
                  {p.primary && <Badge>Primary</Badge>}
                </Label>
              ))}
            </RadioGroup>
          ) : (
            <p className="text-sm text-muted-foreground">No game profiles seen on this console yet. Log in once on the Wii to populate this list.</p>
          )}
        </div>
      </CollapsibleContent>
    </Collapsible>
  )
}

export function SettingsPage() {
  const { session, setSession, reload } = useSession()
  const [consoles, setConsoles] = useState<ConsoleSummary[] | null>(null)
  const [openHost, setOpenHost] = useState<{ modes: OpenHostMode[]; profiles: OpenHostProfile[] } | null>(null)
  const [display, setDisplay] = useState(loadDisplaySettings)
  const account = session?.account
  const linked = hasLinkedProfile(account)

  useEffect(() => {
    if (session && !session.authenticated) window.location.replace("/login")
  }, [session])

  const loadConsoles = useCallback(async () => {
    try {
      const response = await api<{ linked: boolean; consoles: ConsoleSummary[] }>("/api/panel/console/me")
      setConsoles(response?.linked ? list(response.consoles) : [])
    } catch (error) {
      toast.error(errorMessage(error))
      setConsoles([])
    }
  }, [])

  const loadOpenHost = useCallback(async () => {
    try {
      const response = await api<{ modes: OpenHostMode[]; profiles: OpenHostProfile[] }>("/api/panel/my/open-host")
      setOpenHost({ modes: list(response.modes), profiles: list(response.profiles) })
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }, [])

  useEffect(() => {
    if (!session?.authenticated) return
    void loadConsoles()
    if (linked) void loadOpenHost()
    else setOpenHost(null)
  }, [session?.authenticated, linked, loadConsoles, loadOpenHost])

  function accountChanged(next?: Account) {
    if (next && session) setSession({ ...session, account: next })
    else void reload()
    void loadConsoles()
  }

  function updateDisplay(next: typeof display) {
    setDisplay(next)
    saveDisplaySettings(next)
    toast.success("Preferences saved")
  }

  if (!session?.authenticated || !account) {
    return (
      <div className="min-h-svh bg-background">
        <SiteHeader session={session} page="settings" subtitle="Account settings" />
        <main className="mx-auto grid max-w-4xl gap-4 px-4 py-8">
          <Skeleton className="h-10 w-48" />
          <Skeleton className="h-40 w-full" />
          <Skeleton className="h-60 w-full" />
        </main>
      </div>
    )
  }

  const profilesByConsole = new Map<string, OpenHostProfile[]>()
  for (const p of openHost?.profiles ?? []) {
    const key = String(p.console_id || "unknown")
    profilesByConsole.set(key, [...(profilesByConsole.get(key) ?? []), p])
  }

  const tabs = [
    { id: "profile", label: "Profile", icon: UserRound },
    { id: "consoles", label: "Consoles", icon: Monitor },
    { id: "open-host", label: "Open Host", icon: Gamepad2, hidden: !linked },
    { id: "preferences", label: "Preferences", icon: SlidersHorizontal },
  ].filter((t) => !t.hidden)

  return (
    <div className="min-h-svh bg-background">
      <SiteHeader session={session} page="settings" subtitle="Account settings" />
      <section className="border-b bg-muted/30">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center gap-5 px-4 py-8">
          <UserAvatar name={account.username} src={session.avatar_url} className="size-16" fallbackClassName="text-xl" />
          <div className="grid gap-1.5">
            <h1 className="text-2xl font-semibold tracking-tight">{account.username}</h1>
            <div className="flex flex-wrap items-center gap-1.5 text-sm text-muted-foreground">
              <Badge variant="secondary">
                <ShieldCheck /> {roleLabel(account.role)}
              </Badge>
              {account.fc && <Badge variant="outline" className="font-mono">FC {account.fc}</Badge>}
              {account.pid && <Badge variant="outline" className="font-mono">PID {account.pid}</Badge>}
              <span>member since {new Date(account.created).toLocaleDateString()}</span>
            </div>
          </div>
          {account.role !== "player" && (
            <Button asChild className="ml-auto">
              <a href="/panel/dashboard">Open moderation panel</a>
            </Button>
          )}
        </div>
      </section>
      <main className="mx-auto max-w-5xl px-4 py-8">
        <Tabs defaultValue="profile" orientation="vertical" className="flex-col gap-6 md:flex-row">
          <TabsList variant="line" className="h-fit w-full shrink-0 flex-row items-stretch overflow-x-auto md:w-48 md:flex-col">
            {tabs.map((t) => (
              <TabsTrigger key={t.id} value={t.id} className="justify-start gap-2 px-3 py-2">
                <t.icon /> {t.label}
              </TabsTrigger>
            ))}
          </TabsList>
          <div className="min-w-0 flex-1">
            <TabsContent value="profile" className="grid gap-6">
              {session.features?.profiles && <PublicProfileCard onAvatarChange={(url) => setSession({ ...session, avatar_url: url })} />}
              <RightsCard account={account} />
            </TabsContent>
            <TabsContent value="consoles" className="grid gap-6">
              <ConsoleLinkCard session={session} onLinked={accountChanged} />
              {consoles && consoles.length > 0 && (
                <Card>
                  <CardHeader>
                    <CardTitle>Assigned consoles</CardTitle>
                    <CardDescription>
                      {consoles.length} console{consoles.length === 1 ? "" : "s"} linked to this account. Nicknames are only shown to you. Pick the primary profile for each console.
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="grid gap-3">
                    {consoles.map((c) => (
                      <ConsoleRow key={c.id} data={c} onChange={setConsoles} onUnlinked={accountChanged} />
                    ))}
                  </CardContent>
                </Card>
              )}

            </TabsContent>
            <TabsContent value="open-host" className="grid gap-6">
              {linked && (
                <Card>
                  <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                      <Gamepad2 className="size-4" /> Open Host
                    </CardTitle>
                    <CardDescription>
                      Auto-accepts incoming friend requests so anyone with your friend code can join matches you host. Restricted users are still blocked. Changes apply live.
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="grid gap-4">
                    {!openHost ? (
                      <Skeleton className="h-24 w-full" />
                    ) : openHost.profiles.length ? (
                      <>
                        <div className="grid gap-2 sm:grid-cols-3">
                          {openHost.modes.map((m) => (
                            <div key={m.value} className="rounded-lg border p-3">
                              <p className="text-sm font-medium">{m.label}</p>
                              <p className="text-xs text-muted-foreground">{m.description}</p>
                            </div>
                          ))}
                        </div>
                        <ItemGroup className="gap-3">
                          {[...profilesByConsole.entries()].map(([consoleId, profiles]) => (
                            <div key={consoleId} className="grid gap-2">
                              <p className="flex items-center gap-2 text-xs font-medium text-muted-foreground uppercase">
                                <Monitor className="size-3.5" /> {profiles[0].console_name || `Console ${consoleId}`}
                              </p>
                              {profiles.map((p) => (
                                <OpenHostProfileRow key={p.pid} profile={p} modes={openHost.modes} />
                              ))}
                            </div>
                          ))}
                        </ItemGroup>
                      </>
                    ) : (
                      <Empty className="border">
                        <EmptyHeader>
                          <EmptyMedia variant="icon">
                            <Gamepad2 />
                          </EmptyMedia>
                          <EmptyTitle>No game profiles yet</EmptyTitle>
                          <EmptyDescription>Log in once on the Wii to populate this list.</EmptyDescription>
                        </EmptyHeader>
                      </Empty>
                    )}
                  </CardContent>
                </Card>
              )}

            </TabsContent>
            <TabsContent value="preferences" className="grid gap-6">
              <Card>
                <CardHeader>
                  <CardTitle>Display preferences</CardTitle>
                  <CardDescription>Saved in this browser. Applies to the public status page.</CardDescription>
                </CardHeader>
                <CardContent>
                  <FieldGroup>
                    <Field orientation="horizontal">
                      <FieldContent>
                        <FieldLabel htmlFor="auto-refresh">Auto refresh</FieldLabel>
                        <FieldDescription>Refresh the public status page every 10 seconds.</FieldDescription>
                      </FieldContent>
                      <Switch id="auto-refresh" checked={display.autoRefresh} onCheckedChange={(v) => updateDisplay({ ...display, autoRefresh: v })} />
                    </Field>
                    <Separator />
                    <Field orientation="horizontal">
                      <FieldContent>
                        <FieldLabel htmlFor="page-size">Rows per page</FieldLabel>
                        <FieldDescription>For the user and ban tables on the public status page.</FieldDescription>
                      </FieldContent>
                      <Select value={String(display.pageSize)} onValueChange={(v) => updateDisplay({ ...display, pageSize: Number(v) })}>
                        <SelectTrigger id="page-size" className="w-24">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {PAGE_SIZES.map((size) => (
                            <SelectItem key={size} value={String(size)}>
                              {size}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </Field>
                  </FieldGroup>
                </CardContent>
              </Card>
            </TabsContent>
          </div>
        </Tabs>
      </main>
    </div>
  )
}
