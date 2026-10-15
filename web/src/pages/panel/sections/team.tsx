import { Crown, Eraser, Link2, Plus, ShieldCheck, Trash2 } from "lucide-react"
import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"

import { useConfirm } from "@/components/confirm"
import { SearchInput } from "@/components/filters"
import { StatusBadge } from "@/components/status-badge"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import { Field, FieldContent, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { Switch } from "@/components/ui/switch"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { api, errorMessage, list, type Account, type Rights } from "@/lib/api"
import { fmtDate, initials, roleLabel } from "@/lib/format"
import { EMPTY_RIGHTS, LOG_TYPES, RIGHT_GROUPS, rightsSummary } from "@/lib/rights"
import { cn } from "@/lib/utils"
import { useLiveArea } from "@/pages/panel/panel-context"

const ITEMS = RIGHT_GROUPS.flatMap((g) => g.items)
type Draft = { items: Record<string, boolean>; logs: string[] }

function draftFrom(account: Account): Draft {
  const rights = account.rights ?? EMPTY_RIGHTS
  return { items: Object.fromEntries(ITEMS.map((item) => [item.id, item.keys.some((k) => rights[k])])), logs: [...(rights.log_types ?? [])] }
}

function rightsFrom(draft: Draft): Rights {
  const rights: Rights = { ...EMPTY_RIGHTS, log_types: [] }
  for (const item of ITEMS) if (draft.items[item.id]) for (const key of item.keys) rights[key] = true
  if (rights.view_logs) rights.log_types = LOG_TYPES.map((t) => t.key).filter((k) => draft.logs.includes(k))
  return rights
}

const roleTone = (role: string) => (role === "owner" ? "info" : role === "moderator" ? "warning" : "neutral")

function NewModeratorDialog({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [busy, setBusy] = useState(false)
  async function submit(event: React.FormEvent) {
    event.preventDefault()
    setBusy(true)
    try {
      await api("/api/panel/accounts", { method: "POST", body: { username: username.trim(), password } })
      toast.success(`Moderator ${username.trim()} created`)
      setUsername("")
      setPassword("")
      onCreated()
      onClose()
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <form onSubmit={submit} className="grid gap-5">
          <DialogHeader>
            <DialogTitle>New moderator</DialogTitle>
            <DialogDescription>Starts with the default moderator rights. Adjust them right after.</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="nm-user">Username</FieldLabel>
              <Input id="nm-user" autoFocus autoComplete="off" required value={username} onChange={(e) => setUsername(e.target.value)} />
            </Field>
            <Field>
              <FieldLabel htmlFor="nm-pass">Password</FieldLabel>
              <Input id="nm-pass" type="password" autoComplete="new-password" required value={password} onChange={(e) => setPassword(e.target.value)} />
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy || !username.trim() || !password}>
              {busy && <Spinner />} Create
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function AccountDetail({ account, onChanged }: { account: Account; onChanged: () => void }) {
  const { confirm } = useConfirm()
  const [draft, setDraft] = useState<Draft>(() => draftFrom(account))
  const [busy, setBusy] = useState(false)
  useEffect(() => setDraft(draftFrom(account)), [account])

  const original = useMemo(() => JSON.stringify(rightsFrom(draftFrom(account))), [account])
  const dirty = JSON.stringify(rightsFrom(draft)) !== original
  const owner = account.role === "owner"

  async function save() {
    setBusy(true)
    try {
      await api("/api/panel/accounts", { method: "PATCH", body: { id: account.id, rights: rightsFrom(draft) } })
      toast.success(`Rights saved for ${account.username}`)
      onChanged()
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function clear() {
    const ok = await confirm({ title: `Clear rights for ${account.username}?`, description: "The account becomes a regular user account.", confirmLabel: "Clear rights", destructive: true })
    if (!ok) return
    try {
      await api("/api/panel/accounts", { method: "PATCH", body: { id: account.id, role: "player" } })
      toast.success("Rights cleared")
      onChanged()
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  async function remove() {
    const ok = await confirm({ title: `Delete ${account.username}?`, description: "Removes the site login only. Game profiles and bans are kept.", confirmLabel: "Delete account", destructive: true })
    if (!ok) return
    try {
      await api("/api/panel/accounts", { method: "DELETE", body: { id: account.id } })
      toast.success("Account deleted")
      onChanged()
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  return (
    <Card className="gap-0 py-0">
      <CardHeader className="gap-3 border-b py-6">
        <div className="flex items-center gap-4">
          <Avatar className="size-12">
            <AvatarFallback>{initials(account.username)}</AvatarFallback>
          </Avatar>
          <div className="grid gap-1">
            <CardTitle className="flex items-center gap-2 text-xl">
              {account.username} {owner && <Crown className="size-4 text-warning" />}
            </CardTitle>
            <CardDescription className="flex flex-wrap items-center gap-1.5">
              <StatusBadge tone={roleTone(account.role)}>{roleLabel(account.role)}</StatusBadge>
              <span>joined {fmtDate(account.created)}</span>
            </CardDescription>
          </div>
        </div>
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Link2 className="size-4" />
          {account.pid || account.fc ? (
            <span>
              {account.name || "Linked profile"} <span className="font-mono text-xs">{[account.fc && `FC ${account.fc}`, account.pid && `PID ${account.pid}`].filter(Boolean).join(" · ")}</span>
            </span>
          ) : (
            "No console linked"
          )}
        </div>
      </CardHeader>
      <CardContent className="grid gap-6 py-6">
        {owner ? (
          <Empty className="border">
            <EmptyHeader>
              <EmptyTitle className="flex items-center gap-2">
                <ShieldCheck className="size-4" /> All rights
              </EmptyTitle>
              <EmptyDescription>The owner always has every right. This cannot be changed.</EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <>
            <div className="grid gap-6 md:grid-cols-2">
              {RIGHT_GROUPS.map((group) => (
                <section key={group.title} className="grid content-start gap-3">
                  <h3 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">{group.title}</h3>
                  {group.items.map((item) => (
                    <div key={item.id} className="grid gap-2">
                      <Field orientation="horizontal" className="justify-between">
                        <FieldContent>
                          <FieldLabel htmlFor={`sw-${item.id}`} className="font-normal">
                            {item.label}
                          </FieldLabel>
                        </FieldContent>
                        <Switch
                          id={`sw-${item.id}`}
                          checked={!!draft.items[item.id]}
                          onCheckedChange={(on) => setDraft((d) => ({ items: { ...d.items, [item.id]: on }, logs: item.logsToggle && !on ? [] : item.logsToggle && on && !d.logs.length ? LOG_TYPES.map((t) => t.key) : d.logs }))}
                        />
                      </Field>
                      {item.logsToggle && draft.items[item.id] && (
                        <ToggleGroup type="multiple" variant="outline" size="sm" value={draft.logs} onValueChange={(logs) => setDraft((d) => ({ ...d, logs }))} className="flex-wrap justify-start">
                          {LOG_TYPES.map((t) => (
                            <ToggleGroupItem key={t.key} value={t.key} className="px-2 text-xs">
                              {t.label}
                            </ToggleGroupItem>
                          ))}
                        </ToggleGroup>
                      )}
                    </div>
                  ))}
                </section>
              ))}
            </div>
            <Separator />
            <section className="grid gap-3">
              <h3 className="text-xs font-medium tracking-wide text-destructive uppercase">Danger zone</h3>
              <div className="flex flex-wrap gap-2">
                {account.role === "moderator" && (
                  <Button variant="outline" size="sm" onClick={() => void clear()}>
                    <Eraser data-icon="inline-start" /> Clear all rights
                  </Button>
                )}
                <Button variant="destructive" size="sm" onClick={() => void remove()}>
                  <Trash2 data-icon="inline-start" /> Delete account
                </Button>
              </div>
            </section>
          </>
        )}
      </CardContent>
      {!owner && (
        <div className={cn("sticky bottom-0 flex items-center gap-3 rounded-b-xl border-t bg-card/95 px-6 py-3 backdrop-blur transition-opacity", !dirty && "opacity-60")}>
          <span className="text-sm text-muted-foreground">{dirty ? "Unsaved changes" : "All changes saved"}</span>
          <div className="ml-auto flex gap-2">
            <Button variant="ghost" size="sm" disabled={!dirty || busy} onClick={() => setDraft(draftFrom(account))}>
              Reset
            </Button>
            <Button size="sm" disabled={!dirty || busy} onClick={() => void save()}>
              {busy && <Spinner />} Save rights
            </Button>
          </div>
        </div>
      )}
    </Card>
  )
}

export function TeamSection() {
  const [accounts, setAccounts] = useState<Account[] | null>(null)
  const [selected, setSelected] = useState<number | null>(null)
  const [query, setQuery] = useState("")
  const [creating, setCreating] = useState(false)

  const load = useCallback(async () => {
    try {
      const rows = list(await api<Account[]>("/api/panel/accounts"))
      const order = { owner: 0, moderator: 1, player: 2 }
      rows.sort((a, b) => order[a.role] - order[b.role] || a.username.localeCompare(b.username))
      setAccounts(rows)
      setSelected((cur) => (cur && rows.some((a) => a.id === cur) ? cur : (rows.find((a) => a.role !== "owner")?.id ?? rows[0]?.id ?? null)))
    } catch (error) {
      toast.error(errorMessage(error))
      setAccounts([])
    }
  }, [])
  useEffect(() => {
    void load()
  }, [load])
  useLiveArea("accounts", () => void load())

  const visible = (accounts ?? []).filter((a) => !query.trim() || a.username.toLowerCase().includes(query.trim().toLowerCase()))
  const current = accounts?.find((a) => a.id === selected)
  const moderators = accounts?.filter((a) => a.role === "moderator").length ?? 0

  return (
    <div className="grid gap-4 lg:grid-cols-[320px_1fr]">
      <Card className="h-fit gap-0 py-0">
        <CardHeader className="gap-2 border-b p-3">
          <div className="flex gap-2">
            <SearchInput value={query} onChange={setQuery} placeholder="Find account" className="sm:w-full" />
            <Button size="icon" onClick={() => setCreating(true)} aria-label="New moderator">
              <Plus />
            </Button>
          </div>
        </CardHeader>
        <div className="scrollbar-thin overflow-y-auto overscroll-contain lg:max-h-[calc(100svh-14rem)]">
          <div className="grid gap-0.5 p-1.5">
            {!accounts
              ? Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-14" />)
              : visible.map((a) => (
                  <button
                    key={a.id}
                    type="button"
                    onClick={() => setSelected(a.id)}
                    className={cn("flex items-center gap-3 rounded-lg px-3 py-2.5 text-left hover:bg-muted/60", selected === a.id && "bg-muted")}
                  >
                    <Avatar className="size-9">
                      <AvatarFallback className="text-xs">{initials(a.username)}</AvatarFallback>
                    </Avatar>
                    <div className="grid min-w-0 flex-1">
                      <span className="truncate text-sm font-medium">{a.username}</span>
                      <span className="truncate text-xs text-muted-foreground">{a.role === "owner" ? "All rights" : `${rightsSummary(a).length} rights`}</span>
                    </div>
                    <StatusBadge tone={roleTone(a.role)}>{roleLabel(a.role)}</StatusBadge>
                  </button>
                ))}
          </div>
        </div>
        <div className="border-t p-3 text-xs text-muted-foreground">
          {accounts?.length ?? 0} accounts · <Badge variant="outline">{moderators} moderator{moderators === 1 ? "" : "s"}</Badge>
        </div>
      </Card>
      {current ? (
        <AccountDetail key={current.id} account={current} onChanged={() => void load()} />
      ) : (
        <Card>
          <CardContent>
            <Empty>
              <EmptyHeader>
                <EmptyTitle>Select an account</EmptyTitle>
                <EmptyDescription>Pick an account to review its rights.</EmptyDescription>
              </EmptyHeader>
            </Empty>
          </CardContent>
        </Card>
      )}
      <NewModeratorDialog open={creating} onClose={() => setCreating(false)} onCreated={() => void load()} />
    </div>
  )
}
