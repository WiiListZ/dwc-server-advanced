import { Ban, CircleUser, DoorOpen, Download, Home, LogOut, Menu, MessageSquare, Plus, RefreshCw, Search, Settings, UserMinus } from "lucide-react"
import { useEffect, useState } from "react"

import { BrandMark } from "@/components/brand"
import { ModeToggle } from "@/components/mode-toggle"
import { logout } from "@/components/site-header"
import { profileHref, UserAvatar } from "@/components/user-avatar"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Kbd, KbdGroup } from "@/components/ui/kbd"
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { roleLabel } from "@/lib/format"
import { serverName, useSession } from "@/lib/hooks"
import { cn } from "@/lib/utils"
import { ActionDialog, CreateRoomDialog } from "@/pages/panel/action-dialogs"
import { CommandMenu, useCommandMenu } from "@/pages/panel/command-menu"
import { JoinRoomDialog, type ForceJoin } from "@/pages/panel/join-dialog"
import { NAV_ITEMS, navItem } from "@/pages/panel/nav"
import { PanelProvider, canModerate, sectionAllowed, usePanel } from "@/pages/panel/panel-context"
import { PlayerSheet } from "@/pages/panel/player-sheet"
import { RoomSheet } from "@/pages/panel/room-views"
import { ActivitySection } from "@/pages/panel/sections/activity"
import { BansSection } from "@/pages/panel/sections/bans"
import { DashboardSection } from "@/pages/panel/sections/dashboard"
import { GamesSection } from "@/pages/panel/sections/games"
import { GhostsSection } from "@/pages/panel/sections/ghosts"
import { LiveSection } from "@/pages/panel/sections/live"
import { ReportsSection } from "@/pages/panel/sections/reports"
import { ServerSection, type UpdateStatus } from "@/pages/panel/sections/server"
import { TeamSection } from "@/pages/panel/sections/team"
import { api } from "@/lib/api"

function NavLinks({ onNavigate, vertical }: { onNavigate?: () => void; vertical?: boolean }) {
  const { account, section, navigate, dashboard } = usePanel()
  return (
    <nav className={cn("flex gap-1", vertical ? "flex-col" : "items-center")}>
      {NAV_ITEMS.filter((item) => sectionAllowed(item.id, account)).map((item) => {
        const active = section === item.id
        const count = item.id === "live" ? dashboard.users.length : undefined
        return (
          <Button
            key={item.id}
            variant="ghost"
            size="sm"
            className={cn("relative justify-start gap-2 text-muted-foreground", active && "bg-muted text-foreground", vertical && "h-10 justify-start")}
            onClick={() => {
              navigate(item.id)
              onNavigate?.()
            }}
          >
            <item.icon />
            <span className={cn(!vertical && "hidden xl:inline")}>{item.label}</span>
            {count ? <span className="rounded-full bg-success/15 px-1.5 text-[10px] font-semibold text-success tabular-nums">{count}</span> : null}
          </Button>
        )
      })}
    </nav>
  )
}

// UpdateBadge tells the owner a new release is waiting.
function UpdateBadge() {
  const { isOwner, navigate, section } = usePanel()
  const [status, setStatus] = useState<UpdateStatus | null>(null)
  useEffect(() => {
    if (!isOwner) return
    const load = () => api<UpdateStatus>("/api/panel/update").then(setStatus).catch(() => undefined)
    load()
    const id = window.setInterval(load, 10 * 60_000)
    return () => window.clearInterval(id)
  }, [isOwner, section])
  if (!status?.available || !status.latest) return null
  const label = status.schedule ? `Update ${status.latest.tag} scheduled` : `Update ${status.latest.tag}`
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button size="sm" variant="secondary" className="hidden sm:inline-flex" aria-label={label} onClick={() => navigate("server")}>
          <Download data-icon="inline-start" />
          <span className="hidden 2xl:inline">{label}</span>
        </Button>
      </TooltipTrigger>
      <TooltipContent className="2xl:hidden">{label}</TooltipContent>
    </Tooltip>
  )
}

function QuickActions() {
  const { account, can, openAction, openCreateRoom } = usePanel()
  if (!canModerate(account) && !can("manage_rooms_create")) return null
  return (
    <DropdownMenu>
      <Tooltip>
        <TooltipTrigger asChild>
          <DropdownMenuTrigger asChild>
            <Button size="icon" aria-label="Quick actions">
              <Plus />
            </Button>
          </DropdownMenuTrigger>
        </TooltipTrigger>
        <TooltipContent>Quick actions</TooltipContent>
      </Tooltip>
      <DropdownMenuContent align="end" className="w-52">
        <DropdownMenuLabel>Quick actions</DropdownMenuLabel>
        {can("message") && (
          <DropdownMenuItem onSelect={() => openAction("message")}>
            <MessageSquare /> Send login message
          </DropdownMenuItem>
        )}
        {can("manage_rooms_create") && (
          <DropdownMenuItem onSelect={() => openCreateRoom()}>
            <DoorOpen /> Create room
          </DropdownMenuItem>
        )}
        {can("kick") && (
          <DropdownMenuItem onSelect={() => openAction("kick")}>
            <UserMinus /> Kick a player
          </DropdownMenuItem>
        )}
        {can("ban") && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={() => openAction("ban")}>
              <Ban /> Ban a player
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function AccountMenu() {
  const { account, session } = usePanel()
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" className="rounded-full" aria-label="Account">
          <UserAvatar name={account.username} src={session.avatar_url} className="size-8" fallbackClassName="text-xs" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel className="grid">
          <span>{account.username}</span>
          <span className="text-xs font-normal text-muted-foreground">{roleLabel(account.role)}</span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <a href="/">
            <Home /> Public status page
          </a>
        </DropdownMenuItem>
        {session.features?.profiles && (
          <DropdownMenuItem asChild>
            <a href={profileHref(account.username)}>
              <CircleUser /> My profile
            </a>
          </DropdownMenuItem>
        )}
        <DropdownMenuItem asChild>
          <a href="/settings">
            <Settings /> Account settings
          </a>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={() => void logout()}>
          <LogOut /> Logout
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function PanelShell() {
  const { section, dashboard, dashboardUpdated, refreshDashboard } = usePanel()
  const { open, setOpen } = useCommandMenu()
  const [forceJoin, setForceJoin] = useState<ForceJoin | null>(null)
  const [refreshing, setRefreshing] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)
  const item = navItem(section)
  const online = dashboard.stats.global?.online ?? 0

  useEffect(() => {
    document.title = `${item.label} · ${serverName()}`
  }, [item])

  async function refresh() {
    setRefreshing(true)
    await refreshDashboard()
    setRefreshing(false)
  }

  return (
    <div className="min-h-svh bg-background">
      <header className="sticky top-0 z-40 border-b bg-background/80 backdrop-blur-md supports-[backdrop-filter]:bg-background/60">
        <div className="mx-auto flex h-14 max-w-[1600px] items-center gap-2 px-4">
          <Sheet open={menuOpen} onOpenChange={setMenuOpen}>
            <SheetTrigger asChild>
              <Button variant="ghost" size="icon" className="lg:hidden" aria-label="Open navigation">
                <Menu />
              </Button>
            </SheetTrigger>
            <SheetContent side="left" className="w-72 p-4">
              <SheetHeader className="px-0">
                <SheetTitle className="flex items-center gap-2">
                  <BrandMark /> {serverName()}
                </SheetTitle>
              </SheetHeader>
              <NavLinks vertical onNavigate={() => setMenuOpen(false)} />
            </SheetContent>
          </Sheet>
          <a href="/" className="mr-2 flex items-center gap-2.5">
            <BrandMark />
            <span className="hidden max-w-44 truncate text-sm font-semibold whitespace-nowrap md:inline">{serverName()}</span>
          </a>
          <div className="hidden lg:block">
            <NavLinks />
          </div>
          <div className="ml-auto flex items-center gap-1.5">
            <Button variant="outline" size="sm" className="hidden w-60 justify-start text-muted-foreground 2xl:flex" onClick={() => setOpen(true)}>
              <Search data-icon="inline-start" /> Find player or room…
              <KbdGroup className="ml-auto">
                <Kbd>Ctrl</Kbd>
                <Kbd>K</Kbd>
              </KbdGroup>
            </Button>
            <Button variant="ghost" size="icon" className="2xl:hidden" aria-label="Search" onClick={() => setOpen(true)}>
              <Search />
            </Button>
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="hidden items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium sm:flex">
                  <span className="relative flex size-2">
                    <span className="absolute inline-flex size-full animate-ping rounded-full bg-success opacity-60" />
                    <span className="relative inline-flex size-2 rounded-full bg-success" />
                  </span>
                  <span className="tabular-nums">{online}</span> online
                </span>
              </TooltipTrigger>
              <TooltipContent>{dashboardUpdated ? `Updated ${dashboardUpdated.toLocaleTimeString()}` : "Connecting…"}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon" aria-label="Refresh" onClick={() => void refresh()} disabled={refreshing}>
                  {refreshing ? <Spinner /> : <RefreshCw />}
                </Button>
              </TooltipTrigger>
              <TooltipContent>Refresh</TooltipContent>
            </Tooltip>
            <UpdateBadge />
            <ModeToggle />
            <QuickActions />
            <AccountMenu />
          </div>
        </div>
      </header>
      <main className="mx-auto grid max-w-[1600px] gap-6 px-4 py-6 md:px-6">
        {section !== "dashboard" && (
          <div className="grid gap-1">
            <h1 className="flex items-center gap-2.5 text-2xl font-semibold tracking-tight">
              <item.icon className="size-5 text-muted-foreground" /> {item.label}
            </h1>
            <p className="text-sm text-muted-foreground">{item.description}</p>
          </div>
        )}
        {section === "dashboard" && <DashboardSection />}
        {section === "live" && <LiveSection forceJoin={forceJoin} clearForceJoin={() => setForceJoin(null)} />}
        {section === "reports" && <ReportsSection />}
        {section === "bans" && <BansSection />}
        {section === "activity" && <ActivitySection />}
        {section === "ghosts" && <GhostsSection />}
        {section === "games" && <GamesSection />}
        {section === "team" && <TeamSection />}
        {section === "server" && <ServerSection />}
      </main>
      <CommandMenu open={open} onOpenChange={setOpen} />
      <PlayerSheet />
      <RoomSheet />
      <ActionDialog />
      <CreateRoomDialog />
      <JoinRoomDialog onStarted={setForceJoin} />
    </div>
  )
}

export function PanelApp() {
  const { session } = useSession()

  useEffect(() => {
    if (!session) return
    if (!session.authenticated) window.location.replace("/login")
    else if (session.account?.role === "player") window.location.replace("/settings")
  }, [session])

  if (!session?.authenticated || !session.account || session.account.role === "player") {
    return (
      <div className="min-h-svh">
        <div className="h-14 border-b" />
        <div className="mx-auto grid max-w-[1600px] gap-4 p-6">
          <Skeleton className="h-9 w-80" />
          <div className="grid gap-4 sm:grid-cols-5">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-32" />
            ))}
          </div>
          <Skeleton className="h-72" />
        </div>
      </div>
    )
  }

  return (
    <PanelProvider session={session}>
      <PanelShell />
    </PanelProvider>
  )
}
