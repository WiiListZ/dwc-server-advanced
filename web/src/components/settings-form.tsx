import { Plus, Trash2 } from "lucide-react"
import type { ReactNode } from "react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Field, FieldContent, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { DEVICE_ID_MODES, LOG_LEVELS, LOG_OUTPUTS, WEBHOOK_EVENTS, type ServerSettings } from "@/lib/server-settings"

export type SettingsSection = "general" | "network" | "https" | "database" | "clients" | "moderation" | "updates" | "events" | "advanced"

type FormProps = {
  value: ServerSettings
  onChange: (next: ServerSettings) => void
}

function TextField({
  id,
  label,
  description,
  value,
  onChange,
  placeholder,
  type = "text",
  mono,
}: {
  id: string
  label: string
  description?: ReactNode
  value: string
  onChange: (v: string) => void
  placeholder?: string
  type?: string
  mono?: boolean
}) {
  return (
    <Field>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Input id={id} type={type} value={value} placeholder={placeholder} className={mono ? "font-mono" : undefined} onChange={(e) => onChange(e.target.value)} />
      {description && <FieldDescription>{description}</FieldDescription>}
    </Field>
  )
}

function SwitchField({ id, label, description, checked, onChange }: { id: string; label: string; description?: ReactNode; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <Field orientation="horizontal" className="justify-between">
      <FieldContent>
        <FieldLabel htmlFor={id}>{label}</FieldLabel>
        {description && <FieldDescription>{description}</FieldDescription>}
      </FieldContent>
      <Switch id={id} checked={checked} onCheckedChange={onChange} />
    </Field>
  )
}

function SelectField<T extends string | number>({
  id,
  label,
  description,
  value,
  options,
  onChange,
}: {
  id: string
  label: string
  description?: ReactNode
  value: T
  options: { value: T; label: string }[]
  onChange: (v: T) => void
}) {
  return (
    <Field>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Select value={String(value)} onValueChange={(v) => onChange((typeof value === "number" ? Number(v) : v) as T)}>
        <SelectTrigger id={id} className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((o) => (
            <SelectItem key={String(o.value)} value={String(o.value)}>
              {o.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {description && <FieldDescription>{description}</FieldDescription>}
    </Field>
  )
}

export function SettingsCard({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>
        <FieldGroup>{children}</FieldGroup>
      </CardContent>
    </Card>
  )
}

// SettingsFields renders one group of server settings. The setup wizard and
// the panel's server settings share it.
export function SettingsFields({ section, value: s, onChange, setup }: FormProps & { section: SettingsSection; setup?: boolean }) {
  const set = <K extends keyof ServerSettings>(key: K) => (v: ServerSettings[K]) => onChange({ ...s, [key]: v })

  switch (section) {
    case "general":
      return (
        <>
          <TextField id="serverName" label="Server name" description="Shown on the website and in login messages." value={s.serverName} onChange={set("serverName")} />
          <SwitchField id="showWelcomeMessage" label="Show Wii welcome popup" description="Show a welcome message when a Mario Kart Wii player with the payload connects." checked={s.showWelcomeMessage} onChange={set("showWelcomeMessage")} />
          {s.showWelcomeMessage && (
            <Field>
              <FieldLabel htmlFor="welcomeMessage">Welcome popup text</FieldLabel>
              <Textarea id="welcomeMessage" maxLength={190} rows={3} value={s.welcomeMessage} onChange={(event) => set("welcomeMessage")(event.target.value)} placeholder={`Welcome to\n${s.serverName || "your server"}!`} />
              <FieldDescription>Leave blank to use the server name. Line breaks are supported; longer text is shortened to fit the Wii popup.</FieldDescription>
            </Field>
          )}
          <SwitchField
            id="profilesEnabled"
            label="Public profiles"
            description="Accounts get a page at /u/<username> with avatar, bio, game status and stats. Off: no profile pages at all."
            checked={s.profilesEnabled}
            onChange={set("profilesEnabled")}
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <SelectField id="logLevel" label="Log level" value={s.logLevel} options={LOG_LEVELS} onChange={set("logLevel")} />
            <SelectField id="logOutput" label="Log output" value={s.logOutput} options={LOG_OUTPUTS} onChange={set("logOutput")} />
          </div>
        </>
      )
    case "network":
      return (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            <TextField id="gsAddress" label="GameSpy bind address" mono value={s.gsAddress} onChange={set("gsAddress")} placeholder="0.0.0.0" description="0.0.0.0 listens on every interface." />
            <TextField id="payloadServerAddress" label="Payload server" mono value={s.payloadServerAddress} onChange={set("payloadServerAddress")} placeholder="Empty: built-in" />
            <TextField id="nasAddress" label="NAS / website bind address" mono value={s.nasAddress} onChange={set("nasAddress")} placeholder="0.0.0.0" />
            <TextField id="nasPort" label="NAS / website port" mono value={s.nasPort} onChange={set("nasPort")} placeholder="80" description="Consoles always use port 80." />
          </div>
        </>
      )
    case "https":
      return (
        <>
          <SwitchField id="enableHttps" label="HTTPS" description="Needed by consoles with the SSL exploit and by modern browsers." checked={s.enableHttps} onChange={set("enableHttps")} />
          {s.enableHttps && (
            <div className="grid gap-4 sm:grid-cols-2">
              <TextField id="nasAddressHttps" label="HTTPS bind address" mono value={s.nasAddressHttps} onChange={set("nasAddressHttps")} placeholder="0.0.0.0" />
              <TextField id="nasPortHttps" label="HTTPS port" mono value={s.nasPortHttps} onChange={set("nasPortHttps")} placeholder="443" />
              <TextField id="certPath" label="Certificate" mono value={s.certPath} onChange={set("certPath")} placeholder="fullchain.pem" />
              <TextField id="keyPath" label="Private key" mono value={s.keyPath} onChange={set("keyPath")} placeholder="privkey.pem" />
            </div>
          )}
          <Separator />
          <SwitchField id="exploitWii" label="Wii SSL exploit" description="Serves the NAS certificate chain that unpatched Wii consoles accept." checked={s.enableHttpsExploitWii} onChange={set("enableHttpsExploitWii")} />
          {s.enableHttpsExploitWii && (
            <div className="grid gap-4 sm:grid-cols-2">
              <TextField id="certDerPathWii" label="Wii certificate chain (DER)" mono value={s.certDerPathWii} onChange={set("certDerPathWii")} placeholder="naswii-chain.der" />
              <TextField id="keyPathWii" label="Wii private key" mono value={s.keyPathWii} onChange={set("keyPathWii")} placeholder="naswii-signed-key.pem" />
            </div>
          )}
          <SwitchField id="exploitDS" label="DS SSL exploit" checked={s.enableHttpsExploitDS} onChange={set("enableHttpsExploitDS")} />
          {s.enableHttpsExploitDS && (
            <div className="grid gap-4 sm:grid-cols-3">
              <TextField id="certDerPathDS" label="DS certificate (DER)" mono value={s.certDerPathDS} onChange={set("certDerPathDS")} placeholder="nas-cert.der" />
              <TextField id="wiiCertDerPathDS" label="Wii client certificate" mono value={s.wiiCertDerPathDS} onChange={set("wiiCertDerPathDS")} placeholder="nwc.der" />
              <TextField id="keyPathDS" label="DS private key" mono value={s.keyPathDS} onChange={set("keyPathDS")} placeholder="nas-key.pem" />
            </div>
          )}
          <FieldDescription>Paths are relative to the server's data directory. See docs/HTTPS_CERTIFICATES.md to generate them.</FieldDescription>
        </>
      )
    case "database":
      return (
        <div className="grid gap-4 sm:grid-cols-2">
          <TextField id="databaseAddress" label="Host" mono value={s.databaseAddress} onChange={set("databaseAddress")} placeholder="127.0.0.1 or db:5432" />
          <TextField id="databaseName" label="Database" mono value={s.databaseName} onChange={set("databaseName")} placeholder="wwfc" />
          <TextField id="username" label="User" mono value={s.username} onChange={set("username")} placeholder="Database user" />
          <TextField
            id="password"
            label="Password"
            type="password"
            value={s.password ?? ""}
            onChange={set("password")}
            placeholder={s.hasPassword && !setup ? "Unchanged" : ""}
            description={s.hasPassword && !setup ? "Leave empty to keep the current password." : undefined}
          />
        </div>
      )
    case "clients":
      return (
        <>
          <SwitchField id="requirePayloadFeatures" label="Require payload features for Wii clients" description="Reject Wii clients without a verified payload at login. DNS-only and NoSSL clients cannot connect; Nintendo DS clients are unaffected. Leave off to allow both client types." checked={s.requirePayloadFeatures} onChange={set("requirePayloadFeatures")} />
          <SwitchField id="allowNoSSL" label="Allow NoSSL cheat / DNS-patched clients" description="Turn on for local Dolphin setups; see docs/LOCAL_SETUP.md." checked={s.allowNoSSLCheatClients} onChange={set("allowNoSSLCheatClients")} />
          <SwitchField id="allowDolphin" label="Allow default Dolphin keys" description="Lets Dolphin users connect without a dumped NAND." checked={s.allowDefaultDolphinKeys} onChange={set("allowDefaultDolphinKeys")} />
          <SwitchField id="allowNoDevice" label="Allow connecting without a device ID" checked={s.allowConnectWithoutDeviceID} onChange={set("allowConnectWithoutDeviceID")} />
          <SelectField id="multiDevice" label="Multiple device IDs per profile" value={s.allowMultipleDeviceIDs} options={DEVICE_ID_MODES} onChange={set("allowMultipleDeviceIDs")} />
        </>
      )
    case "moderation":
      return (
        <>
          <SwitchField
            id="keepRevoked"
            label="Keep lifted bans as “Revoked”"
            description="On: an unbanned player keeps the ban in the history, marked revoked. Off: unbanning deletes the ban from the database."
            checked={s.keepRevokedBans}
            onChange={set("keepRevokedBans")}
          />
          <Field className="sm:max-w-xs">
            <FieldLabel htmlFor="banHistoryDays">Ban history visibility (days)</FieldLabel>
            <Input id="banHistoryDays" type="number" min={1} max={3650} value={s.banHistoryDays} onChange={(e) => set("banHistoryDays")(Number(e.target.value))} />
            <FieldDescription>How long ended bans, revoked bans and kicks stay in the ban lists.</FieldDescription>
          </Field>
        </>
      )
    case "updates":
      return (
        <>
          <SwitchField id="updatesEnabled" label="Check for new releases" description="The owner gets a notice in the panel and chooses when to install." checked={s.updatesEnabled} onChange={set("updatesEnabled")} />
          <div className="grid gap-4 sm:grid-cols-2">
            <TextField id="updatesRepository" label="GitHub repository" mono value={s.updatesRepository} onChange={set("updatesRepository")} placeholder="WiiListZ/dwc-server-advanced" />
            <Field>
              <FieldLabel htmlFor="updatesInterval">Check every (minutes)</FieldLabel>
              <Input id="updatesInterval" type="number" min={5} max={10080} value={s.updatesCheckIntervalMinutes} onChange={(e) => set("updatesCheckIntervalMinutes")(Number(e.target.value))} />
            </Field>
          </div>
        </>
      )
    case "events":
      return (
        <>
          <SwitchField id="logEventsDb" label="Store events in the database" description="Needed for the Activity timeline in the panel." checked={s.logEventsToDatabase} onChange={set("logEventsToDatabase")} />
          <Separator />
          <div className="grid gap-3">
            <div className="flex items-center justify-between">
              <FieldLabel>Discord webhooks</FieldLabel>
              <Button type="button" size="sm" variant="outline" onClick={() => onChange({ ...s, webhooks: [...s.webhooks, { enabled: true, url: "", author: s.serverName, eventTypes: ["all"] }] })}>
                <Plus data-icon="inline-start" /> Add webhook
              </Button>
            </div>
            {s.webhooks.length === 0 && <FieldDescription>No webhooks. Add one to post events to a Discord channel.</FieldDescription>}
            {s.webhooks.map((hook, i) => {
              const update = (patch: Partial<typeof hook>) => onChange({ ...s, webhooks: s.webhooks.map((h, j) => (j === i ? { ...h, ...patch } : h)) })
              return (
                <div key={i} className="grid gap-3 rounded-lg border p-3">
                  <div className="flex items-center gap-2">
                    <Switch checked={hook.enabled} onCheckedChange={(enabled) => update({ enabled })} aria-label="Webhook enabled" />
                    <Input className="font-mono" placeholder="https://discord.com/api/webhooks/…" value={hook.url} onChange={(e) => update({ url: e.target.value })} />
                    <Button type="button" size="icon" variant="ghost" aria-label="Remove webhook" onClick={() => onChange({ ...s, webhooks: s.webhooks.filter((_, j) => j !== i) })}>
                      <Trash2 />
                    </Button>
                  </div>
                  <Input placeholder="Author name" value={hook.author} onChange={(e) => update({ author: e.target.value })} />
                  <ToggleGroup
                    type="multiple"
                    variant="outline"
                    size="sm"
                    value={hook.eventTypes}
                    onValueChange={(types) => update({ eventTypes: types.includes("all") && !hook.eventTypes.includes("all") ? ["all"] : types.filter((t) => t !== "all" || types.length === 1) })}
                    className="flex-wrap justify-start"
                  >
                    {WEBHOOK_EVENTS.map((t) => (
                      <ToggleGroupItem key={t} value={t} className="px-2 font-mono text-[11px]">
                        {t}
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                </div>
              )
            })}
          </div>
        </>
      )
    case "advanced":
      return (
        <>
          <FieldDescription>Internal RPC addresses between the frontend and backend processes. Keep them on 127.0.0.1 and never expose them.</FieldDescription>
          <div className="grid gap-4 sm:grid-cols-2">
            <TextField id="frontendAddress" label="Frontend RPC" mono value={s.frontendAddress} onChange={set("frontendAddress")} />
            <TextField id="backendAddress" label="Backend RPC" mono value={s.backendAddress} onChange={set("backendAddress")} />
            <TextField id="frontendBackendAddress" label="Frontend → backend" mono value={s.frontendBackendAddress} onChange={set("frontendBackendAddress")} />
            <TextField id="backendFrontendAddress" label="Backend → frontend" mono value={s.backendFrontendAddress} onChange={set("backendFrontendAddress")} />
          </div>
          {!setup && (
            <SwitchField
              id="regenerateApiSecret"
              label="Generate a new API secret on save"
              description={s.hasApiSecret ? "The secret for /api/ban, /api/kick and /api/unban is set. Turn on to replace it." : "A secret is generated automatically."}
              checked={!!s.regenerateApiSecret}
              onChange={set("regenerateApiSecret")}
            />
          )}
        </>
      )
  }
}
