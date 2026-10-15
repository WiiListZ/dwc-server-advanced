import { Area, AreaChart, Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts"

import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from "@/components/ui/chart"
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import type { Stats } from "@/lib/api"

const perGameConfig = {
  online: { label: "Online", color: "var(--chart-1)" },
  active: { label: "In a match", color: "var(--chart-2)" },
} satisfies ChartConfig

export function PlayersByGameChart({ stats }: { stats: Stats }) {
  const data = Object.entries(stats)
    .filter(([game]) => game !== "global")
    .map(([game, value]) => ({ game, online: value.online, active: value.active }))
    .sort((a, b) => b.online - a.online)
    .slice(0, 8)

  if (!data.length) {
    return (
      <Empty className="h-[220px]">
        <EmptyHeader>
          <EmptyTitle>No players online</EmptyTitle>
          <EmptyDescription>Per-game totals show up once someone connects.</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <ChartContainer config={perGameConfig} className="aspect-auto h-[220px] w-full">
      <BarChart data={data} layout="vertical" margin={{ left: 8, right: 8 }}>
        <CartesianGrid horizontal={false} />
        <XAxis type="number" allowDecimals={false} tickLine={false} axisLine={false} />
        <YAxis type="category" dataKey="game" width={110} tickLine={false} axisLine={false} tick={{ fontSize: 12 }} />
        <ChartTooltip cursor={false} content={<ChartTooltipContent />} />
        <Bar dataKey="online" fill="var(--color-online)" radius={4} />
        <Bar dataKey="active" fill="var(--color-active)" radius={4} />
      </BarChart>
    </ChartContainer>
  )
}

export type OnlineSample = { time: number; online: number; active: number; rooms: number }

const historyConfig = {
  online: { label: "Online", color: "var(--chart-1)" },
  active: { label: "In a match", color: "var(--chart-2)" },
  rooms: { label: "Rooms", color: "var(--chart-3)" },
} satisfies ChartConfig

export function OnlineHistoryChart({ samples }: { samples: OnlineSample[] }) {
  const data = samples.map((s) => ({ ...s, label: new Date(s.time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }) }))
  return (
    <ChartContainer config={historyConfig} className="aspect-auto h-[220px] w-full">
      <AreaChart data={data} margin={{ left: 0, right: 8, top: 8 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="label" tickLine={false} axisLine={false} minTickGap={40} tick={{ fontSize: 11 }} />
        <YAxis allowDecimals={false} tickLine={false} axisLine={false} width={28} />
        <ChartTooltip cursor={false} content={<ChartTooltipContent indicator="dot" />} />
        <Area dataKey="rooms" type="monotone" fill="var(--color-rooms)" fillOpacity={0.15} stroke="var(--color-rooms)" strokeWidth={2} />
        <Area dataKey="active" type="monotone" fill="var(--color-active)" fillOpacity={0.15} stroke="var(--color-active)" strokeWidth={2} />
        <Area dataKey="online" type="monotone" fill="var(--color-online)" fillOpacity={0.15} stroke="var(--color-online)" strokeWidth={2} />
      </AreaChart>
    </ChartContainer>
  )
}

const matchesConfig = { matches: { label: "Matches", color: "var(--chart-2)" } } satisfies ChartConfig

export function MatchesPerHourChart({ buckets }: { buckets: { hour: string; matches: number }[] }) {
  return (
    <ChartContainer config={matchesConfig} className="aspect-auto h-[220px] w-full">
      <BarChart data={buckets} margin={{ left: 0, right: 8, top: 8 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="hour" tickLine={false} axisLine={false} minTickGap={16} tick={{ fontSize: 11 }} />
        <YAxis allowDecimals={false} tickLine={false} axisLine={false} width={28} />
        <ChartTooltip cursor={false} content={<ChartTooltipContent />} />
        <Bar dataKey="matches" fill="var(--color-matches)" radius={4} />
      </BarChart>
    </ChartContainer>
  )
}
