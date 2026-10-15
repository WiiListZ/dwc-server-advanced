import { ArrowLeft, ArrowRight, Check, CircleCheck, CircleX, Container, Database, FolderOpen, KeyRound, Network, RefreshCw, Rocket, ShieldCheck, Sparkles } from "lucide-react"
import { useEffect, useState } from "react"

import { LogoIcon } from "@/components/brand"
import { ModeToggle } from "@/components/mode-toggle"
import { SettingsFields } from "@/components/settings-form"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { errorMessage } from "@/lib/api"
import type { ServerSettings } from "@/lib/server-settings"
import { cn } from "@/lib/utils"

const token = new URLSearchParams(window.location.search).get("token") ?? ""

async function setupApi<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method: body === undefined ? "GET" : "POST",
    headers: { Accept: "application/json", "Content-Type": "application/json", "X-Setup-Token": token },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`)
  return data as T
}

type DbStatus = { ok: boolean; hasSchema?: boolean; hasOwner?: boolean; error?: string }
type DockerStatus = { available: boolean; reason?: string; version?: string }
type FolderStatus = { path: string; exists: boolean; writable: boolean; hasConfig: boolean; entries: number; error?: string }
type DockerDatabase = { databaseAddress: string; databaseName: string; username: string; password: string; composeFile: string; reused: boolean }
type SetupState = { version: string; defaults: ServerSettings; addresses: string[]; port: string; dataDir: string; dataDirFixed: boolean; docker: DockerStatus }

const STEPS = [
  { id: "welcome", label: "Welcome", icon: Sparkles },
  { id: "folder", label: "Folder", icon: FolderOpen },
  { id: "database", label: "Database", icon: Database },
  { id: "network", label: "Network", icon: Network },
  { id: "rules", label: "Rules", icon: ShieldCheck },
  { id: "owner", label: "Owner", icon: KeyRound },
  { id: "finish", label: "Finish", icon: Rocket },
] as const

export function SetupPage() {
  const [step, setStep] = useState(0)
  const [settings, setSettings] = useState<ServerSettings | null>(null)
  const [meta, setMeta] = useState<Omit<SetupState, "defaults"> | null>(null)
  const [folder, setFolder] = useState("")
  const [folderStatus, setFolderStatus] = useState<FolderStatus | null>(null)
  const [checkingFolder, setCheckingFolder] = useState(false)
  const [docker, setDocker] = useState<DockerStatus | null>(null)
  const [dockerBusy, setDockerBusy] = useState(false)
  const [dockerError, setDockerError] = useState("")
  const [dockerDb, setDockerDb] = useState<DockerDatabase | null>(null)
  const [loadError, setLoadError] = useState("")
  const [db, setDb] = useState<DbStatus | null>(null)
  const [testing, setTesting] = useState(false)
  const [owner, setOwner] = useState({ username: "", password: "", confirm: "" })
  const [finishing, setFinishing] = useState(false)
  const [finishError, setFinishError] = useState("")
  const [done, setDone] = useState<{ url: string; ready: boolean } | null>(null)

  useEffect(() => {
    if (!token) return
    setupApi<SetupState>("/api/setup/state")
      .then((state) => {
        setSettings({ ...state.defaults, password: "" })
        setMeta(state)
        setFolder(state.dataDir)
        setDocker(state.docker)
      })
      .catch((error) => setLoadError(errorMessage(error)))
  }, [])

  async function checkFolder() {
    setCheckingFolder(true)
    try {
      setFolderStatus(await setupApi<FolderStatus>("/api/setup/folder", { path: folder, create: true }))
    } catch (error) {
      setFolderStatus({ path: folder, exists: false, writable: false, hasConfig: false, entries: 0, error: errorMessage(error) })
    } finally {
      setCheckingFolder(false)
    }
  }

  async function refreshDocker() {
    setDockerError("")
    try {
      setDocker(await setupApi<DockerStatus>("/api/setup/docker"))
    } catch (error) {
      setDockerError(errorMessage(error))
    }
  }

  async function createDockerDatabase() {
    if (!settings) return
    setDockerBusy(true)
    setDockerError("")
    try {
      const created = await setupApi<DockerDatabase>("/api/setup/docker-database", { dataDir: folderStatus?.path ?? folder })
      setDockerDb(created)
      const next = { ...settings, databaseAddress: created.databaseAddress, databaseName: created.databaseName, username: created.username, password: created.password }
      setSettings(next)
      await testDatabase(next)
    } catch (error) {
      setDockerError(errorMessage(error))
    } finally {
      setDockerBusy(false)
    }
  }

  async function testDatabase(values = settings) {
    if (!values) return
    setTesting(true)
    try {
      setDb(
        await setupApi<DbStatus>("/api/setup/test-database", {
          databaseAddress: values.databaseAddress,
          databaseName: values.databaseName,
          username: values.username,
          password: values.password,
        }),
      )
    } catch (error) {
      setDb({ ok: false, error: errorMessage(error) })
    } finally {
      setTesting(false)
    }
  }

  async function finish() {
    if (!settings) return
    setFinishing(true)
    setFinishError("")
    try {
      const result = await setupApi<{ nasPort: string }>("/api/setup/complete", {
        settings,
        owner: { username: owner.username, password: owner.password },
        dataDir: folderStatus?.path ?? folder,
      })
      const port = result.nasPort === "80" ? "" : `:${result.nasPort}`
      const url = `${window.location.protocol}//${window.location.hostname}${port}/login`
      setDone({ url, ready: false })
      waitForServer(url, (ready) => setDone({ url, ready }))
    } catch (error) {
      setFinishError(errorMessage(error))
    } finally {
      setFinishing(false)
    }
  }

  if (!token) {
    return (
      <Shell>
        <Card className="w-full max-w-lg">
          <CardHeader>
            <CardTitle>Server setup</CardTitle>
            <CardDescription>This server has not been configured yet.</CardDescription>
          </CardHeader>
          <CardContent>
            <Alert>
              <KeyRound />
              <AlertTitle>Open the link from the server console</AlertTitle>
              <AlertDescription>The console that started the server printed a private link with a setup token. Open that link to continue.</AlertDescription>
            </Alert>
          </CardContent>
        </Card>
      </Shell>
    )
  }

  if (done) {
    return (
      <Shell>
        <Card className="w-full max-w-lg text-center">
          <CardHeader className="items-center">
            {done.ready ? <CircleCheck className="mx-auto size-10 text-success" /> : <Spinner className="mx-auto size-8" />}
            <CardTitle className="text-xl">{done.ready ? "Your server is ready" : "Starting the server…"}</CardTitle>
            <CardDescription>{done.ready ? "Sign in with the owner account you just created." : "config.yml is saved. The game services and the website are starting."}</CardDescription>
          </CardHeader>
          <CardFooter className="justify-center">
            <Button asChild disabled={!done.ready}>
              <a href={done.url}>
                Go to login <ArrowRight data-icon="inline-end" />
              </a>
            </Button>
          </CardFooter>
        </Card>
      </Shell>
    )
  }

  if (loadError) {
    return (
      <Shell>
        <Alert variant="destructive" className="max-w-lg">
          <CircleX />
          <AlertTitle>Cannot start the setup</AlertTitle>
          <AlertDescription>{loadError}</AlertDescription>
        </Alert>
      </Shell>
    )
  }

  if (!settings || !meta) {
    return (
      <Shell>
        <Skeleton className="h-96 w-full max-w-3xl" />
      </Shell>
    )
  }

  const current = STEPS[step].id
  const ownerNeeded = !db?.hasOwner
  const ownerValid = !ownerNeeded || (owner.username.trim() !== "" && owner.password.length >= 8 && owner.password === owner.confirm)
  const folderReady = !!folderStatus?.writable && folderStatus.path !== "" && !folderStatus.error
  const canNext = current === "folder" ? folderReady : current === "database" ? !!db?.ok : current === "owner" ? ownerValid : true

  return (
    <Shell>
      <div className="grid w-full max-w-4xl gap-6 md:grid-cols-[200px_1fr]">
        <ol className="flex gap-1 overflow-x-auto md:flex-col">
          {STEPS.map((s, i) => (
            <li key={s.id}>
              <button
                type="button"
                disabled={i > step}
                onClick={() => setStep(i)}
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm whitespace-nowrap text-muted-foreground transition-colors",
                  i === step && "bg-muted font-medium text-foreground",
                  i < step && "hover:bg-muted/60",
                )}
              >
                <span className={cn("flex size-6 items-center justify-center rounded-full border text-xs", i < step && "border-primary bg-primary text-primary-foreground")}>
                  {i < step ? <Check className="size-3.5" /> : i + 1}
                </span>
                {s.label}
              </button>
            </li>
          ))}
        </ol>

        <Card>
          {current === "welcome" && (
            <>
              <CardHeader>
                <CardTitle className="text-2xl">Welcome to DWC Server Advanced</CardTitle>
                <CardDescription>
                  Version {meta.version}. This wizard writes config.yml, prepares the database and creates the owner account. You can change every setting later from the panel.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <FieldGroup>
                  <SettingsFields section="general" value={settings} onChange={setSettings} setup />
                </FieldGroup>
              </CardContent>
            </>
          )}

          {current === "folder" && (
            <>
              <CardHeader>
                <CardTitle>Server folder</CardTitle>
                <CardDescription>
                  Where the server keeps its files: config.yml, certificates, payload/, logs/ and state/. The folder is created if it does not exist. You can manage its files later in Panel → Server.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <FieldGroup>
                  <Field>
                    <FieldLabel htmlFor="data-dir">Folder</FieldLabel>
                    <Input
                      id="data-dir"
                      className="font-mono"
                      value={folder}
                      disabled={meta.dataDirFixed}
                      onChange={(e) => (setFolder(e.target.value), setFolderStatus(null))}
                      placeholder={meta.dataDir}
                    />
                    <FieldDescription>
                      {meta.dataDirFixed
                        ? "Chosen with --dir or DWC_HOME when the server was started, so it cannot be changed here."
                        : "An absolute path, or a path relative to the folder the server was started from. If you pick another folder, the server writes dwc-home.txt next to the program so it finds the folder again on the next start."}
                    </FieldDescription>
                  </Field>
                  <div className="flex flex-wrap items-center gap-3">
                    <Button type="button" variant="outline" onClick={() => void checkFolder()} disabled={checkingFolder || !folder.trim()}>
                      {checkingFolder ? <Spinner /> : <FolderOpen data-icon="inline-start" />} Use this folder
                    </Button>
                    {folderStatus && (
                      <span className={cn("flex items-center gap-1.5 text-sm", folderReady ? "text-success" : "text-destructive")}>
                        {folderReady ? <CircleCheck className="size-4" /> : <CircleX className="size-4" />}
                        {folderStatus.error ?? (folderStatus.entries ? `Ready: ${folderStatus.path} (${folderStatus.entries} item${folderStatus.entries === 1 ? "" : "s"} already inside)` : `Ready: ${folderStatus.path}`)}
                      </span>
                    )}
                  </div>
                  {folderStatus?.hasConfig && (
                    <Alert>
                      <CircleX />
                      <AlertTitle>This folder already has a config.yml</AlertTitle>
                      <AlertDescription>Finishing the setup replaces it with the settings chosen here.</AlertDescription>
                    </Alert>
                  )}
                </FieldGroup>
              </CardContent>
            </>
          )}

          {current === "database" && (
            <>
              <CardHeader>
                <CardTitle>PostgreSQL database</CardTitle>
                <CardDescription>Let the setup create the database with Docker, or connect to a PostgreSQL server you already have. Empty databases are set up for you.</CardDescription>
              </CardHeader>
              <CardContent>
                <FieldGroup>
                  <div className="grid gap-3 rounded-lg border p-4">
                    <div className="flex items-start gap-3">
                      <Container className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
                      <div className="grid gap-1">
                        <span className="font-medium">Create the database with Docker</span>
                        <span className="text-sm text-muted-foreground">
                          {docker?.available
                            ? `Docker ${docker.version ?? ""} found. The setup writes postgres/docker-compose.yml in the server folder with a generated password, starts PostgreSQL on 127.0.0.1 and fills in the fields below. The first start downloads the PostgreSQL image and can take a few minutes.`
                            : docker?.reason ?? "Checking Docker…"}
                        </span>
                      </div>
                    </div>
                    <div className="flex flex-wrap items-center gap-3">
                      {docker?.available ? (
                        <Button type="button" onClick={() => void createDockerDatabase()} disabled={dockerBusy}>
                          {dockerBusy ? <Spinner /> : <Container data-icon="inline-start" />} {dockerBusy ? "Starting PostgreSQL…" : dockerDb ? "Start it again" : "Create database"}
                        </Button>
                      ) : (
                        <Button type="button" variant="outline" onClick={() => void refreshDocker()}>
                          <RefreshCw data-icon="inline-start" /> Check again
                        </Button>
                      )}
                      {dockerDb && !dockerError && (
                        <span className="flex items-center gap-1.5 text-sm text-success">
                          <CircleCheck className="size-4" /> {dockerDb.reused ? "Existing Docker database started" : "Docker database created"} · {dockerDb.composeFile}
                        </span>
                      )}
                    </div>
                    {dockerError && <p className="text-sm whitespace-pre-line text-destructive">{dockerError}</p>}
                  </div>
                  <Separator />
                  <SettingsFields section="database" value={settings} onChange={(s) => (setSettings(s), setDb(null))} setup />
                  <div className="flex flex-wrap items-center gap-3">
                    <Button type="button" variant="outline" onClick={() => void testDatabase()} disabled={testing || dockerBusy}>
                      {testing ? <Spinner /> : <Database data-icon="inline-start" />} Test connection
                    </Button>
                    {db && (
                      <span className={cn("flex items-center gap-1.5 text-sm", db.ok ? "text-success" : "text-destructive")}>
                        {db.ok ? <CircleCheck className="size-4" /> : <CircleX className="size-4" />}
                        {db.ok ? (db.hasSchema ? (db.hasOwner ? "Connected. Existing server data and owner found." : "Connected. Existing tables found.") : "Connected. The tables will be created.") : db.error}
                      </span>
                    )}
                  </div>
                </FieldGroup>
              </CardContent>
            </>
          )}

          {current === "network" && (
            <>
              <CardHeader>
                <CardTitle>Network</CardTitle>
                <CardDescription>
                  Where the game services and the website listen. This machine: {meta.addresses.join(", ") || "unknown"}. Point Nintendo's hostnames to this server (docs/HOSTNAMES.md).
                </CardDescription>
              </CardHeader>
              <CardContent>
                <FieldGroup>
                  <SettingsFields section="network" value={settings} onChange={setSettings} setup />
                  <Separator />
                  <SettingsFields section="https" value={settings} onChange={setSettings} setup />
                </FieldGroup>
              </CardContent>
            </>
          )}

          {current === "rules" && (
            <>
              <CardHeader>
                <CardTitle>Clients, moderation and updates</CardTitle>
                <CardDescription>Sensible defaults are already chosen.</CardDescription>
              </CardHeader>
              <CardContent>
                <FieldGroup>
                  <SettingsFields section="clients" value={settings} onChange={setSettings} setup />
                  <Separator />
                  <SettingsFields section="moderation" value={settings} onChange={setSettings} setup />
                  <Separator />
                  <SettingsFields section="updates" value={settings} onChange={setSettings} setup />
                </FieldGroup>
              </CardContent>
            </>
          )}

          {current === "owner" && (
            <>
              <CardHeader>
                <CardTitle>Owner account</CardTitle>
                <CardDescription>
                  {ownerNeeded ? "The owner always has every right and manages the other accounts." : "This database already has an owner account; sign in with it after the setup."}
                </CardDescription>
              </CardHeader>
              {ownerNeeded && (
                <CardContent>
                  <FieldGroup>
                    <Field>
                      <FieldLabel htmlFor="owner-user">Username</FieldLabel>
                      <Input id="owner-user" autoComplete="username" value={owner.username} onChange={(e) => setOwner({ ...owner, username: e.target.value })} />
                    </Field>
                    <div className="grid gap-4 sm:grid-cols-2">
                      <Field>
                        <FieldLabel htmlFor="owner-pass">Password</FieldLabel>
                        <Input id="owner-pass" type="password" autoComplete="new-password" value={owner.password} onChange={(e) => setOwner({ ...owner, password: e.target.value })} />
                        <FieldDescription>At least 8 characters.</FieldDescription>
                      </Field>
                      <Field>
                        <FieldLabel htmlFor="owner-confirm">Repeat password</FieldLabel>
                        <Input id="owner-confirm" type="password" autoComplete="new-password" value={owner.confirm} onChange={(e) => setOwner({ ...owner, confirm: e.target.value })} />
                        {owner.confirm && owner.confirm !== owner.password && <FieldDescription className="text-destructive">Passwords do not match.</FieldDescription>}
                      </Field>
                    </div>
                  </FieldGroup>
                </CardContent>
              )}
            </>
          )}

          {current === "finish" && (
            <>
              <CardHeader>
                <CardTitle>Ready to start</CardTitle>
                <CardDescription>Review the main choices. Everything can be changed later in Panel → Server.</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-4">
                <dl className="grid gap-2 text-sm sm:grid-cols-[180px_1fr]">
                  {[
                    ["Server name", settings.serverName],
                    ["Server folder", folderStatus?.path ?? folder],
                    ["Database", `${settings.username}@${settings.databaseAddress}/${settings.databaseName}`],
                    ["Website", `${settings.nasAddress}:${settings.nasPort}${settings.enableHttps ? ` · HTTPS ${settings.nasPortHttps}` : ""}`],
                    ["GameSpy", settings.gsAddress],
                    ["Revoked bans", settings.keepRevokedBans ? `kept, visible ${settings.banHistoryDays} days` : `deleted on unban, history ${settings.banHistoryDays} days`],
                    ["Profiles", settings.profilesEnabled ? "on" : "off"],
                    ["Updates", settings.updatesEnabled ? settings.updatesRepository : "off"],
                    ["Owner", ownerNeeded ? owner.username : "existing account"],
                  ].map(([k, v]) => (
                    <div key={k} className="contents">
                      <dt className="text-muted-foreground">{k}</dt>
                      <dd className="font-medium break-all">{v}</dd>
                    </div>
                  ))}
                </dl>
                {finishError && (
                  <Alert variant="destructive">
                    <CircleX />
                    <AlertTitle>Setup failed</AlertTitle>
                    <AlertDescription>{finishError}</AlertDescription>
                  </Alert>
                )}
              </CardContent>
            </>
          )}

          <CardFooter className="justify-between">
            <Button type="button" variant="ghost" disabled={step === 0 || finishing} onClick={() => setStep(step - 1)}>
              <ArrowLeft data-icon="inline-start" /> Back
            </Button>
            {current === "finish" ? (
              <Button type="button" onClick={() => void finish()} disabled={finishing}>
                {finishing ? <Spinner /> : <Rocket data-icon="inline-start" />} Save and start the server
              </Button>
            ) : (
              <Button type="button" onClick={() => setStep(step + 1)} disabled={!canNext}>
                Next <ArrowRight data-icon="inline-end" />
              </Button>
            )}
          </CardFooter>
        </Card>
      </div>
    </Shell>
  )
}

function Shell({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-svh flex-col bg-background">
      <header className="flex h-14 items-center gap-2.5 border-b px-4">
        <LogoIcon />
        <span className="text-sm font-semibold">DWC Server Advanced</span>
        <span className="text-sm text-muted-foreground">· First-run setup</span>
        <div className="ml-auto">
          <ModeToggle />
        </div>
      </header>
      <main className="flex flex-1 items-start justify-center p-4 py-10">{children}</main>
    </div>
  )
}

// waitForServer polls the new website until it answers.
function waitForServer(url: string, onChange: (ready: boolean) => void) {
  const started = Date.now()
  const tick = () => {
    fetch(url, { mode: "no-cors", cache: "no-store" })
      .then(() => onChange(true))
      .catch(() => {
        if (Date.now() - started < 120_000) window.setTimeout(tick, 1500)
      })
  }
  window.setTimeout(tick, 2500)
}
