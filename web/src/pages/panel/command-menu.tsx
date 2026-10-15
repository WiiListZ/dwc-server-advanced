import { Ban, DoorOpen, Home, LogOut, MessageSquare, Moon, RefreshCw, Settings, Sun, UserMinus, UserRound } from "lucide-react"
import { useTheme } from "next-themes"
import { useEffect, useMemo, useState } from "react"

import { logout } from "@/components/site-header"
import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  CommandShortcut,
} from "@/components/ui/command"
import { NAV_ITEMS } from "@/pages/panel/nav"
import { sectionAllowed, usePanel } from "@/pages/panel/panel-context"

export function useCommandMenu() {
  const [open, setOpen] = useState(false)
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === "k" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault()
        setOpen((o) => !o)
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [])
  return { open, setOpen }
}

export function CommandMenu({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { account, navigate, dashboard, can, openPlayer, openRoom, openAction, openCreateRoom, refreshDashboard } = usePanel()
  const { resolvedTheme, setTheme } = useTheme()
  const sections = NAV_ITEMS.filter((item) => sectionAllowed(item.id, account))
  const players = useMemo(() => dashboard.users.filter((u) => u.pid || u.fc).slice(0, 300), [dashboard.users])
  const run = (fn: () => void) => {
    onOpenChange(false)
    fn()
  }

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} title="Command menu" description="Find a player, a room or an action." className="sm:max-w-xl">
      <Command>
        <CommandInput placeholder="Find a player, a room, or type an action…" />
        <CommandList className="max-h-[420px]">
          <CommandEmpty>No results found.</CommandEmpty>
          {players.length > 0 && (
            <CommandGroup heading="Players online">
              {players.map((u) => (
                <CommandItem key={`p-${u.pid}-${u.fc}`} value={`player ${u.name} ${u.fc ?? ""} ${u.pid ?? ""}`} onSelect={() => run(() => openPlayer({ pid: u.pid, fc: u.fc, name: u.name }))}>
                  <UserRound /> {u.name || u.fc}
                  <span className="text-xs text-muted-foreground">{u.game}</span>
                  <CommandShortcut className="font-mono tracking-normal">{u.fc || u.pid}</CommandShortcut>
                </CommandItem>
              ))}
            </CommandGroup>
          )}
          {dashboard.rooms.length > 0 && (
            <CommandGroup heading="Rooms">
              {dashboard.rooms.slice(0, 100).map((r) => (
                <CommandItem key={`r-${r.id}`} value={`room ${r.id} ${r.game} ${r.type}`} onSelect={() => run(() => openRoom(r.id))}>
                  <DoorOpen /> <span className="font-mono">{r.id}</span>
                  <span className="text-xs text-muted-foreground">
                    {r.game} · {r.type}
                  </span>
                  <CommandShortcut>{Object.keys(r.players || {}).length}</CommandShortcut>
                </CommandItem>
              ))}
            </CommandGroup>
          )}
          <CommandSeparator />
          <CommandGroup heading="Actions">
            {can("ban") && (
              <CommandItem value="action ban a player" onSelect={() => run(() => openAction("ban"))}>
                <Ban /> Ban a player…
              </CommandItem>
            )}
            {can("kick") && (
              <CommandItem value="action kick a player" onSelect={() => run(() => openAction("kick"))}>
                <UserMinus /> Kick a player…
              </CommandItem>
            )}
            {can("message") && (
              <CommandItem value="action send login message" onSelect={() => run(() => openAction("message"))}>
                <MessageSquare /> Send a login message…
              </CommandItem>
            )}
            {can("manage_rooms_create") && (
              <CommandItem value="action create room" onSelect={() => run(() => openCreateRoom())}>
                <DoorOpen /> Create a room…
              </CommandItem>
            )}
            <CommandItem value="action refresh data" onSelect={() => run(() => void refreshDashboard())}>
              <RefreshCw /> Refresh data
            </CommandItem>
          </CommandGroup>
          <CommandSeparator />
          <CommandGroup heading="Go to">
            {sections.map((item) => (
              <CommandItem key={item.id} value={`go ${item.label}`} onSelect={() => run(() => navigate(item.id))}>
                <item.icon /> {item.label}
              </CommandItem>
            ))}
            <CommandItem value="go public status page" onSelect={() => run(() => window.location.assign("/"))}>
              <Home /> Public status page
            </CommandItem>
            <CommandItem value="go account settings" onSelect={() => run(() => window.location.assign("/settings"))}>
              <Settings /> Account settings
            </CommandItem>
          </CommandGroup>
          <CommandSeparator />
          <CommandGroup heading="Preferences">
            <CommandItem value="toggle theme dark light" onSelect={() => run(() => setTheme(resolvedTheme === "dark" ? "light" : "dark"))}>
              {resolvedTheme === "dark" ? <Sun /> : <Moon />} Switch to {resolvedTheme === "dark" ? "light" : "dark"} theme
            </CommandItem>
            <CommandItem value="logout sign out" onSelect={() => run(() => void logout())}>
              <LogOut /> Logout
            </CommandItem>
          </CommandGroup>
        </CommandList>
      </Command>
    </CommandDialog>
  )
}
