import { Flag, Gamepad2, Gavel, Ghost, LayoutDashboard, Radio, ScrollText, Server, UsersRound, type LucideIcon } from "lucide-react"

import type { SectionId } from "@/pages/panel/panel-context"

export type NavItem = { id: SectionId; label: string; icon: LucideIcon; description: string }

export const NAV_ITEMS: NavItem[] = [
  { id: "dashboard", label: "Dashboard", icon: LayoutDashboard, description: "Network health and what needs your attention." },
  { id: "live", label: "Live", icon: Radio, description: "Rooms and players connected right now. Click anything to inspect it." },
  { id: "reports", label: "Reports", icon: Flag, description: "Triage player reports one by one." },
  { id: "bans", label: "Bans", icon: Gavel, description: "Bans and kicks. Click a row to open the player." },
  { id: "activity", label: "Activity", icon: ScrollText, description: "Every moderation and network event of the last 14 days." },
  { id: "ghosts", label: "Ghosts", icon: Ghost, description: "Mario Kart Wii time trials and ghost data." },
  { id: "games", label: "Games", icon: Gamepad2, description: "Block games from the service with custom error codes." },
  { id: "team", label: "Team", icon: UsersRound, description: "Site accounts and what each moderator can do." },
  { id: "server", label: "Server", icon: Server, description: "Version, updates and every server setting." },
]

export const navItem = (id: SectionId) => NAV_ITEMS.find((item) => item.id === id) ?? NAV_ITEMS[0]
