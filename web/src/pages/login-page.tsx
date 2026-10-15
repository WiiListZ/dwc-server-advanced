import { KeyRound, ShieldCheck, UserPlus } from "lucide-react"
import { useEffect, useState } from "react"

import { Brand } from "@/components/brand"
import { ModeToggle } from "@/components/mode-toggle"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { errorMessage, post } from "@/lib/api"
import { useSession } from "@/lib/hooks"

type Mode = "login" | "register" | "setup"

const copy: Record<Mode, { title: string; description: string; submit: string }> = {
  login: { title: "Welcome back", description: "Sign in to your website account.", submit: "Sign in" },
  register: {
    title: "Create an account",
    description: "Create a site account, then link it to your in-game profile from Settings.",
    submit: "Create account",
  },
  setup: {
    title: "Create the owner account",
    description: "This first account becomes the owner and always has every right.",
    submit: "Create owner",
  },
}

export function LoginPage() {
  const { session } = useSession()
  const [mode, setMode] = useState<Mode>("login")
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!session) return
    if (session.authenticated) window.location.replace(session.account?.role === "player" ? "/settings" : "/panel")
    else if (session.setup_required) setMode("setup")
  }, [session])

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError("")
    const endpoint = mode === "setup" ? "/api/panel/setup" : mode === "register" ? "/api/panel/register" : "/api/panel/login"
    try {
      await post(endpoint, { username: username.trim(), password })
      window.location.replace("/")
    } catch (err) {
      setError(errorMessage(err))
      setBusy(false)
    }
  }

  const text = copy[mode]
  const Icon = mode === "setup" ? ShieldCheck : mode === "register" ? UserPlus : KeyRound

  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      <div className="flex flex-col gap-4 p-6 md:p-10">
        <div className="flex items-center justify-between">
          <Brand subtitle="Account" />
          <ModeToggle />
        </div>
        <div className="flex flex-1 items-center justify-center">
          <Card className="w-full max-w-sm">
            <CardHeader>
              <div className="mb-2 flex size-10 items-center justify-center rounded-lg bg-muted">
                <Icon className="size-5" />
              </div>
              <CardTitle className="text-xl">{text.title}</CardTitle>
              <CardDescription>{text.description}</CardDescription>
            </CardHeader>
            <form onSubmit={submit}>
              <CardContent>
                <FieldGroup>
                  {error && (
                    <Alert variant="destructive">
                      <AlertTitle>Could not continue</AlertTitle>
                      <AlertDescription>{error}</AlertDescription>
                    </Alert>
                  )}
                  <Field>
                    <FieldLabel htmlFor="username">Username</FieldLabel>
                    <Input id="username" autoComplete="username" required value={username} onChange={(e) => setUsername(e.target.value)} />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="password">Password</FieldLabel>
                    <Input
                      id="password"
                      type="password"
                      required
                      autoComplete={mode === "login" ? "current-password" : "new-password"}
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                    />
                  </Field>
                  <Button type="submit" className="w-full" disabled={busy || !session}>
                    {busy && <Spinner />} {text.submit}
                  </Button>
                </FieldGroup>
              </CardContent>
            </form>
            {mode !== "setup" && (
              <CardFooter className="justify-center text-sm text-muted-foreground">
                {mode === "login" ? "No account yet?" : "Already registered?"}
                <Button variant="link" size="sm" className="px-1.5" onClick={() => setMode(mode === "login" ? "register" : "login")}>
                  {mode === "login" ? "Register" : "Sign in"}
                </Button>
              </CardFooter>
            )}
          </Card>
        </div>
      </div>
      <div className="relative hidden overflow-hidden border-l bg-muted lg:block">
        <div className="relative flex h-full flex-col justify-end gap-3 p-10">
          <p className="text-3xl font-semibold tracking-tight">Nintendo Wi-Fi Connection, revived.</p>
          <p className="max-w-md text-muted-foreground">
            Play DS and Wii games online again. Link your console to manage Open Host, review your rights and follow live matchmaking.
          </p>
        </div>
      </div>
    </div>
  )
}
