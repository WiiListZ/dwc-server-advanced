import { CircleAlert, CircleUser, DoorOpen, House, LogIn, LogOut, Settings, Shield, UserRound, UserX, Users } from "lucide-react"

import { Brand, BrandMark } from "@/components/brand"
import { ModeToggle } from "@/components/mode-toggle"
import { profileHref, UserAvatar } from "@/components/user-avatar"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { post, type Session } from "@/lib/api"
import { roleLabel } from "@/lib/format"

export async function logout(redirect = "/") {
  await post("/api/panel/logout").catch(() => undefined)
  window.location.replace(redirect)
}

export function AccountMenu({ session, onLogout }: { session: Session | null; onLogout?: () => void }) {
  const account = session?.account
  if (!session?.authenticated || !account) {
    return (
      <Button asChild size="sm">
        <a href="/login">
          <LogIn data-icon="inline-start" /> Login
        </a>
      </Button>
    )
  }
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="rounded-full" aria-label="Account menu">
          <UserAvatar name={account.username} src={session.avatar_url} className="size-7" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel className="grid">
          <span className="truncate">{account.username}</span>
          <span className="text-xs font-normal text-muted-foreground">{roleLabel(account.role)}</span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuGroup>
          {account.role !== "player" && (
            <DropdownMenuItem asChild>
              <a href="/panel/dashboard">
                <Shield /> Moderation panel
              </a>
            </DropdownMenuItem>
          )}
          {session.features?.profiles && (
            <DropdownMenuItem asChild>
              <a href={profileHref(account.username)}>
                <CircleUser /> My profile
              </a>
            </DropdownMenuItem>
          )}
          <DropdownMenuItem asChild>
            <a href="/settings">
              <Settings /> Settings
            </a>
          </DropdownMenuItem>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={() => (onLogout ? onLogout() : void logout())}>
          <LogOut /> Logout
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export type PublicTab = "home" | "rooms" | "players" | "bans"
type Page = PublicTab | "errors" | "settings" | "profile"

export function SiteHeader({
  session,
  page,
  subtitle,
  counts,
}: {
  session: Session | null
  page: Page
  subtitle: string
  counts?: Partial<Record<PublicTab, number>>
}) {
  const nav = [
    { id: "home", href: "/#home", label: "Home", icon: House },
    { id: "rooms", href: "/#rooms", label: "Rooms", icon: DoorOpen },
    { id: "players", href: "/#players", label: "Players", icon: Users },
    { id: "bans", href: "/#bans", label: "Ban list", icon: UserX },
    { id: "errors", href: "/errors", label: "Error codes", icon: CircleAlert },
  ] as const
  return (
    <header className="sticky top-0 z-40 border-b bg-background/80 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="mx-auto flex h-14 max-w-6xl items-center gap-3 px-4">
        <div className="hidden sm:block">
          <Brand subtitle={subtitle} />
        </div>
        <a href="/" className="sm:hidden" aria-label="Home">
          <BrandMark />
        </a>
        <nav className="flex min-w-0 flex-1 items-center gap-1 sm:ml-4">
          {nav.map((item) => {
            const count = counts?.[item.id as PublicTab]
            return (
              <Button key={item.id} asChild variant={page === item.id ? "secondary" : "ghost"} size="sm">
                <a href={item.href} aria-label={item.label} aria-current={page === item.id ? "page" : undefined}>
                  <item.icon data-icon="inline-start" />
                  <span className="hidden lg:inline">{item.label}</span>
                  {count !== undefined && <span className="hidden text-xs text-muted-foreground tabular-nums lg:inline">{count}</span>}
                </a>
              </Button>
            )
          })}
          {session?.authenticated && (
            <Button asChild variant={page === "settings" ? "secondary" : "ghost"} size="sm" className="hidden sm:inline-flex">
              <a href="/settings" aria-label="Account">
                <UserRound data-icon="inline-start" />
                <span className="hidden lg:inline">Account</span>
              </a>
            </Button>
          )}
        </nav>
        <div className="ml-auto flex shrink-0 items-center gap-1">
          <ModeToggle />
          <AccountMenu session={session} />
        </div>
      </div>
    </header>
  )
}

export function PageHeading({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow?: string
  title: string
  description?: string
  actions?: React.ReactNode
}) {
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div className="grid gap-1">
        {eyebrow && <p className="text-xs font-medium tracking-wider text-muted-foreground uppercase">{eyebrow}</p>}
        <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">{title}</h1>
        {description && <p className="max-w-2xl text-sm text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  )
}
