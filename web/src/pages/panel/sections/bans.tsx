import type { ColumnDef } from "@tanstack/react-table"
import { Gavel, Undo2 } from "lucide-react"
import { useMemo, useState } from "react"
import { toast } from "sonner"

import { banKindColumn, banPeriodColumn, banReasonColumn, banRemainingColumn, banTargetColumn } from "@/components/ban-columns"
import { useConfirm } from "@/components/confirm"
import { DataTable } from "@/components/data-table"
import { BanFilterToggle, SearchInput } from "@/components/filters"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { errorMessage, post, type BanRecord } from "@/lib/api"
import { banStatus, filterBans, isKickRecord, type BanFilter } from "@/lib/format"
import { usePanel } from "@/pages/panel/panel-context"

export function useUnban() {
  const { confirm } = useConfirm()
  const { refreshDashboard } = usePanel()
  return async (identifier: string) => {
    const ok = await confirm({ title: `Unban ${identifier}?`, description: "The console and every profile on it can connect again.", confirmLabel: "Unban" })
    if (!ok) return false
    try {
      await post("/api/panel/unban", { identifier })
      toast.success(`Unbanned ${identifier}`)
      void refreshDashboard()
      return true
    } catch (error) {
      toast.error(errorMessage(error))
      return false
    }
  }
}

export function BansSection() {
  const { dashboard, dashboardLoading, account, can, openPlayer, openAction } = usePanel()
  const unban = useUnban()
  const [filter, setFilter] = useState<BanFilter>("all")
  const [query, setQuery] = useState("")
  const rows = useMemo(() => filterBans(dashboard.bans, filter, query, account), [dashboard.bans, filter, query, account])

  const columns = useMemo<ColumnDef<BanRecord>[]>(
    () => [
      banKindColumn(),
      banTargetColumn(true),
      banReasonColumn(),
      {
        accessorKey: "reason_hidden",
        header: "Moderator notes",
        cell: ({ row }) => <span className="line-clamp-3 max-w-60 text-xs text-muted-foreground">{row.original.reason_hidden || "-"}</span>,
      },
      { accessorKey: "moderator", header: "Moderator", cell: ({ row }) => row.original.moderator || "-" },
      banPeriodColumn(),
      banRemainingColumn(),
      {
        id: "actions",
        header: "",
        enableSorting: false,
        cell: ({ row }) => {
          const ban = row.original
          if (banStatus(ban) !== "active" || isKickRecord(ban) || !can("unban") || !ban.pid) return null
          return (
            <Button size="sm" variant="outline" onClick={() => void unban(String(ban.pid))}>
              <Undo2 data-icon="inline-start" /> Unban
            </Button>
          )
        },
      },
    ],
    [can, unban],
  )

  return (
    <Card>
      <CardHeader>
        <CardTitle>Ban list</CardTitle>
        <CardDescription>Console bans, moderator kicks and private room kicks. Click a row to open the player.</CardDescription>
        {can("ban") && (
          <CardAction>
            <Button size="sm" variant="destructive" onClick={() => openAction("ban")}>
              <Gavel data-icon="inline-start" /> Ban a player
            </Button>
          </CardAction>
        )}
      </CardHeader>
      <CardContent>
        <DataTable
          columns={columns}
          data={rows}
          loading={dashboardLoading}
          emptyTitle="No bans found"
          onRowClick={(ban) => openPlayer({ pid: ban.pid, fc: ban.fc, name: ban.name })}
          toolbar={
            <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <SearchInput value={query} onChange={setQuery} placeholder="Name, friend code, reason, moderator" className="lg:w-80" />
              <BanFilterToggle value={filter} onChange={setFilter} showMine={!!(account.pid || account.fc)} />
            </div>
          }
        />
      </CardContent>
    </Card>
  )
}
