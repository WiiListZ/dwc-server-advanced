import { BookOpen, CalendarClock, CircleAlert, Download, ExternalLink, HardDrive, KeyRound, RefreshCw, RotateCcw, Save, Settings2, SquareTerminal, X } from "lucide-react"
import { Suspense, lazy, useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"

import { useConfirm } from "@/components/confirm"
import { SettingsFields, type SettingsSection } from "@/components/settings-form"
import { Alert, AlertAction, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { api, errorMessage } from "@/lib/api"
import { fmtDate, relativeTime } from "@/lib/format"
import type { ServerSettings } from "@/lib/server-settings"
import { ServerConsoleCard } from "@/pages/panel/sections/server-console"
import { ServerFilesCard } from "@/pages/panel/sections/server-files"

export type UpdateStatus = {
  current: string
  latest?: { tag: string; name: string; notes: string; url: string; published: string; asset?: string }
  available: boolean
  mode: "self" | "notify" | "disabled"
  mode_reason?: string
  last_checked?: string
  error?: string
  schedule?: { version: string; at: string; moderator: string }
  installing: boolean
  platform: string
}

// waitForRestart polls until the server answers with a different version.
export function waitForRestart(previous: string, onBack: (version: string) => void) {
  const started = Date.now()
  const tick = () => {
    api<UpdateStatus>("/api/panel/update")
      .then((s) => {
        if (s.current !== previous || Date.now() - started > 15_000) onBack(s.current)
        else window.setTimeout(tick, 1500)
      })
      .catch(() => {
        if (Date.now() - started < 180_000) window.setTimeout(tick, 1500)
      })
  }
  window.setTimeout(tick, 3000)
}

function toLocalInput(date: Date) {
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function presetTime(preset: string): Date {
  const d = new Date()
  if (preset === "1h") return new Date(d.getTime() + 3600_000)
  if (preset === "4h") return new Date(d.getTime() + 4 * 3600_000)
  d.setHours(4, 0, 0, 0)
  if (d.getTime() <= Date.now()) d.setDate(d.getDate() + 1)
  return d
}

function ScheduleDialog({ open, onClose, onScheduled, version }: { open: boolean; onClose: () => void; onScheduled: (s: UpdateStatus) => void; version: string }) {
  const [preset, setPreset] = useState("night")
  const [custom, setCustom] = useState(() => toLocalInput(presetTime("night")))
  const [busy, setBusy] = useState(false)
  const when = preset === "custom" ? new Date(custom) : presetTime(preset)

  async function submit() {
    if (Number.isNaN(when.getTime())) return void toast.error("Pick a valid date and time")
    setBusy(true)
    try {
      const status = await api<UpdateStatus>("/api/panel/update", { method: "POST", body: { when: when.toISOString() } })
      toast.success(`Update to ${version} scheduled`, { description: fmtDate(when.toISOString()) })
      onScheduled(status)
      onClose()
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Install {version} later</DialogTitle>
          <DialogDescription>The server installs the update by itself at the chosen time and restarts for a few seconds.</DialogDescription>
        </DialogHeader>
        <FieldGroup>
          <Field>
            <FieldLabel>When</FieldLabel>
            <ToggleGroup type="single" variant="outline" value={preset} onValueChange={(v) => v && setPreset(v)} className="flex-wrap justify-start">
              <ToggleGroupItem value="1h" className="px-3">In 1 hour</ToggleGroupItem>
              <ToggleGroupItem value="4h" className="px-3">In 4 hours</ToggleGroupItem>
              <ToggleGroupItem value="night" className="px-3">At 04:00</ToggleGroupItem>
              <ToggleGroupItem value="custom" className="px-3">Custom</ToggleGroupItem>
            </ToggleGroup>
          </Field>
          {preset === "custom" && (
            <Field>
              <FieldLabel htmlFor="update-at">Date and time</FieldLabel>
              <Input id="update-at" type="datetime-local" value={custom} onChange={(e) => setCustom(e.target.value)} />
            </Field>
          )}
          <FieldDescription>Installs {Number.isNaN(when.getTime()) ? "…" : `${fmtDate(when.toISOString())} (${relativeTime(when.toISOString())})`}. Players online at that time are disconnected briefly.</FieldDescription>
        </FieldGroup>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={() => void submit()} disabled={busy}>
            {busy ? <Spinner /> : <CalendarClock data-icon="inline-start" />} Schedule
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function UpdatesCard() {
  const { confirm } = useConfirm()
  const [status, setStatus] = useState<UpdateStatus | null>(null)
  const [checking, setChecking] = useState(false)
  const [restarting, setRestarting] = useState(false)
  const [scheduling, setScheduling] = useState(false)

  const load = useCallback(() => {
    api<UpdateStatus>("/api/panel/update")
      .then(setStatus)
      .catch((error) => toast.error(errorMessage(error)))
  }, [])
  useEffect(load, [load])

  async function check() {
    setChecking(true)
    try {
      const s = await api<UpdateStatus>("/api/panel/update?action=check", { method: "POST", body: {} })
      setStatus(s)
      toast.success(s.available ? `Version ${s.latest?.tag} is available` : "You are on the latest version")
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setChecking(false)
    }
  }

  async function installNow() {
    if (!status?.latest) return
    const ok = await confirm({
      title: `Install ${status.latest.tag} now?`,
      description: "The server downloads the update and restarts. Players are disconnected for a few seconds.",
      confirmLabel: "Install and restart",
    })
    if (!ok) return
    try {
      await api("/api/panel/update", { method: "POST", body: { when: "now" } })
      setRestarting(true)
      waitForRestart(status.current, (version) => {
        toast.success(`Server restarted on ${version}`)
        window.location.reload()
      })
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  async function cancelSchedule() {
    try {
      setStatus(await api<UpdateStatus>("/api/panel/update", { method: "DELETE" }))
      toast.success("Scheduled update cancelled")
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  if (!status) return <Skeleton className="h-48" />

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          Version {status.current} {status.available ? <Badge>Update available</Badge> : status.latest ? <Badge variant="outline">Up to date</Badge> : null}
        </CardTitle>
        <CardDescription>
          {status.platform}
          {status.last_checked ? ` · checked ${relativeTime(status.last_checked)}` : ""}
          {status.mode === "disabled" ? " · update checks are off" : ""}
        </CardDescription>
        <CardAction>
          <Button size="sm" variant="outline" onClick={() => void check()} disabled={checking || status.mode === "disabled"}>
            {checking ? <Spinner /> : <RefreshCw data-icon="inline-start" />} Check now
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className="grid gap-4">
        {status.error && (
          <Alert variant="destructive">
            <CircleAlert />
            <AlertTitle>Last check failed</AlertTitle>
            <AlertDescription>{status.error}</AlertDescription>
          </Alert>
        )}
        {restarting && (
          <Alert>
            <Spinner />
            <AlertTitle>Installing and restarting…</AlertTitle>
            <AlertDescription>This page reloads when the server is back.</AlertDescription>
          </Alert>
        )}
        {status.schedule && (
          <Alert>
            <CalendarClock />
            <AlertTitle>
              {status.schedule.version} installs {fmtDate(status.schedule.at)}
            </AlertTitle>
            <AlertDescription>
              Scheduled by {status.schedule.moderator} · {relativeTime(status.schedule.at)}
            </AlertDescription>
            <AlertAction>
              <Button size="sm" variant="ghost" onClick={() => void cancelSchedule()}>
                <X data-icon="inline-start" /> Cancel
              </Button>
            </AlertAction>
          </Alert>
        )}
        {status.available && status.latest && (
          <div className="grid gap-3 rounded-lg border p-4">
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-semibold">{status.latest.name || status.latest.tag}</span>
              <span className="text-sm text-muted-foreground">released {relativeTime(status.latest.published)}</span>
              <Button asChild size="sm" variant="link" className="ml-auto h-auto p-0">
                <a href={status.latest.url} target="_blank" rel="noreferrer">
                  Release page <ExternalLink data-icon="inline-end" />
                </a>
              </Button>
            </div>
            {status.latest.notes && <pre className="max-h-56 overflow-y-auto rounded-md bg-muted/50 p-3 font-sans text-sm whitespace-pre-wrap">{status.latest.notes}</pre>}
            {status.mode !== "self" && <p className="text-sm text-muted-foreground">{status.mode_reason}</p>}
          </div>
        )}
        {!status.latest && !status.error && status.mode !== "disabled" && <p className="text-sm text-muted-foreground">No release published yet.</p>}
      </CardContent>
      {status.available && status.mode === "self" && (
        <CardFooter className="flex-wrap gap-2">
          <Button onClick={() => void installNow()} disabled={restarting || status.installing}>
            <Download data-icon="inline-start" /> Install now and restart
          </Button>
          <Button variant="outline" onClick={() => setScheduling(true)} disabled={restarting}>
            <CalendarClock data-icon="inline-start" /> Install later
          </Button>
        </CardFooter>
      )}
      {status.latest && <ScheduleDialog open={scheduling} onClose={() => setScheduling(false)} onScheduled={setStatus} version={status.latest.tag} />}
    </Card>
  )
}

const TABS: { id: SettingsSection; label: string; description: string }[] = [
  { id: "general", label: "General", description: "Server name and logging." },
  { id: "network", label: "Network", description: "Addresses and ports of the game services and the website." },
  { id: "https", label: "HTTPS", description: "Certificates and the Wii/DS SSL exploits." },
  { id: "database", label: "Database", description: "PostgreSQL connection." },
  { id: "clients", label: "Clients", description: "Which consoles and emulators may connect." },
  { id: "moderation", label: "Moderation", description: "What happens to lifted bans and how long history stays visible." },
  { id: "updates", label: "Updates", description: "Release checks from GitHub." },
  { id: "events", label: "Events", description: "Event log and Discord webhooks." },
  { id: "advanced", label: "Advanced", description: "Internal addresses and the API secret." },
]

const ServerDocsCard = lazy(() => import("@/pages/panel/sections/server-docs").then((m) => ({ default: m.ServerDocsCard })))

const SERVER_TABS = [
  { id: "settings", label: "Settings", icon: Settings2 },
  { id: "console", label: "Console", icon: SquareTerminal },
  { id: "files", label: "Files", icon: HardDrive },
  { id: "docs", label: "Docs", icon: BookOpen },
] as const
type ServerTab = (typeof SERVER_TABS)[number]["id"]

function tabFromHash(): ServerTab {
  const hash = window.location.hash.replace(/^#/, "")
  return SERVER_TABS.some((t) => t.id === hash) ? (hash as ServerTab) : "settings"
}

export function ServerSection() {
  const { confirm } = useConfirm()
  const [tab, setTab] = useState<ServerTab>(tabFromHash)

  useEffect(() => {
    const onHash = () => setTab(tabFromHash())
    window.addEventListener("hashchange", onHash)
    return () => window.removeEventListener("hashchange", onHash)
  }, [])

  function changeTab(value: string) {
    setTab(value as ServerTab)
    window.history.replaceState(null, "", value === "settings" ? window.location.pathname : `#${value}`)
  }
  const [saved, setSaved] = useState<ServerSettings | null>(null)
  const [draft, setDraft] = useState<ServerSettings | null>(null)
  const [path, setPath] = useState("")
  const [busy, setBusy] = useState(false)
  const [needsRestart, setNeedsRestart] = useState(false)
  const [restarting, setRestarting] = useState(false)
  const [certGenerating, setCertGenerating] = useState(false)

  useEffect(() => {
    api<{ settings: ServerSettings; path: string }>("/api/panel/server-settings")
      .then((r) => {
        setSaved(r.settings)
        setDraft({ ...r.settings, password: "" })
        setPath(r.path)
      })
      .catch((error) => toast.error(errorMessage(error)))
  }, [])

  const dirty = useMemo(() => !!saved && !!draft && JSON.stringify({ ...draft, password: "" }) !== JSON.stringify({ ...saved, password: "" }) || !!draft?.password, [saved, draft])

  async function save() {
    if (!draft) return
    setBusy(true)
    try {
      const r = await api<{ settings: ServerSettings; restartRequired: boolean }>("/api/panel/server-settings", { method: "PUT", body: draft })
      setSaved(r.settings)
      setDraft({ ...r.settings, password: "" })
      if (r.restartRequired) setNeedsRestart(true)
      toast.success("Settings saved", { description: r.restartRequired ? "Some changes apply after a restart." : "Changes are already active." })
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function restart() {
    const ok = await confirm({ title: "Restart the server?", description: "Players are disconnected for a few seconds.", confirmLabel: "Restart" })
    if (!ok) return
    try {
      const status = await api<UpdateStatus>("/api/panel/update")
      await api("/api/panel/server-restart", { method: "POST", body: {} })
      setRestarting(true)
      waitForRestart(`${status.current}#restart`, () => window.location.reload())
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  async function generateCertificates() {
    setCertGenerating(true)
    try {
      await api("/api/panel/https-certificates", { method: "POST", body: {} })
      setNeedsRestart(true)
      toast.success("HTTPS certificate files are ready", { description: "Restart the server to load newly created files." })
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setCertGenerating(false)
    }
  }

  return (
    <Tabs value={tab} onValueChange={changeTab} className="gap-6">
      <TabsList>
        {SERVER_TABS.map((t) => (
          <TabsTrigger key={t.id} value={t.id} className="gap-1.5 px-3">
            <t.icon /> {t.label}
          </TabsTrigger>
        ))}
      </TabsList>
      <TabsContent value="console">
        <ServerConsoleCard />
      </TabsContent>
      <TabsContent value="files">
        <ServerFilesCard />
      </TabsContent>
      <TabsContent value="docs">
        <Suspense fallback={<Skeleton className="h-96" />}>
          <ServerDocsCard />
        </Suspense>
      </TabsContent>
      <TabsContent value="settings" className="grid gap-6">
        <UpdatesCard />
        {needsRestart && (
          <Alert>
            <RotateCcw />
            <AlertTitle>{restarting ? "Restarting…" : "Restart to apply the saved changes"}</AlertTitle>
            <AlertDescription>Network, HTTPS, database and logging changes are read when the server starts.</AlertDescription>
            <AlertAction>
              <Button size="sm" onClick={() => void restart()} disabled={restarting}>
                {restarting ? <Spinner /> : <RotateCcw data-icon="inline-start" />} Restart now
              </Button>
            </AlertAction>
          </Alert>
        )}
        {!draft ? (
          <Skeleton className="h-96" />
        ) : (
          <Card>
            <CardHeader>
              <CardTitle>Server settings</CardTitle>
              <CardDescription>
                Saved to <code className="font-mono">{path}</code>.
              </CardDescription>
              <CardAction>
                <Button variant="outline" size="sm" onClick={() => void restart()} disabled={restarting}>
                  <RotateCcw data-icon="inline-start" /> Restart server
                </Button>
              </CardAction>
            </CardHeader>
            <CardContent>
              <Tabs defaultValue="general" orientation="vertical" className="flex-col gap-6 md:flex-row">
                <TabsList variant="line" className="h-fit w-full shrink-0 flex-row items-stretch overflow-x-auto md:w-44 md:flex-col">
                  {TABS.map((t) => (
                    <TabsTrigger key={t.id} value={t.id} className="justify-start px-3 py-2">
                      {t.label}
                    </TabsTrigger>
                  ))}
                </TabsList>
                <div className="min-w-0 flex-1">
                  {TABS.map((t) => (
                    <TabsContent key={t.id} value={t.id}>
                      <FieldGroup>
                        <div className="grid gap-1">
                          <h3 className="font-semibold">{t.label}</h3>
                          <p className="text-sm text-muted-foreground">{t.description}</p>
                        </div>
                        <SettingsFields section={t.id} value={draft} onChange={setDraft} />
                        {t.id === "https" && (
                          <div className="grid gap-2 rounded-lg border p-4">
                            <div className="font-medium">Certificate files</div>
                            <p className="text-sm text-muted-foreground">Generate missing HTTPS certificates from the saved settings. Existing certificates and keys are preserved. Restart the server afterwards.</p>
                            <Button type="button" variant="outline" className="w-fit" onClick={() => void generateCertificates()} disabled={certGenerating || busy || dirty || !draft.enableHttps}>
                              {certGenerating ? <Spinner /> : <KeyRound data-icon="inline-start" />} {certGenerating ? "Generating…" : "Generate certificate files"}
                            </Button>
                            {dirty && <p className="text-sm text-muted-foreground">Save the settings before generating certificates.</p>}
                            {!draft.enableHttps && <p className="text-sm text-muted-foreground">Enable HTTPS to generate certificates.</p>}
                          </div>
                        )}
                      </FieldGroup>
                    </TabsContent>
                  ))}
                </div>
              </Tabs>
            </CardContent>
            <CardFooter className="justify-end gap-2 border-t">
              <span className="mr-auto text-sm text-muted-foreground">{dirty ? "Unsaved changes" : "All changes saved"}</span>
              <Button variant="ghost" disabled={!dirty || busy} onClick={() => saved && setDraft({ ...saved, password: "" })}>
                Reset
              </Button>
              <Button disabled={!dirty || busy} onClick={() => void save()}>
                {busy ? <Spinner /> : <Save data-icon="inline-start" />} Save settings
              </Button>
            </CardFooter>
          </Card>
        )}
      </TabsContent>
    </Tabs>
  )
}
