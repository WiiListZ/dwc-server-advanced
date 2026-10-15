import type { ColumnDef } from "@tanstack/react-table"
import { Monitor } from "lucide-react"

import { StatusBadge, remainingTone } from "@/components/status-badge"
import { Badge } from "@/components/ui/badge"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import type { BanRecord } from "@/lib/api"
import { banDurationText, fmtDate, fmtOptionalDate, isKickRecord, moderationKind, remainingBanTime } from "@/lib/format"

export function banKindColumn(): ColumnDef<BanRecord> {
  return {
    id: "kind",
    header: "Type",
    accessorFn: (ban) => moderationKind(ban),
    cell: ({ row }) => (
      <StatusBadge tone={isKickRecord(row.original) ? "warning" : "danger"}>{moderationKind(row.original)}</StatusBadge>
    ),
  }
}

export function banTargetColumn(showPid: boolean): ColumnDef<BanRecord> {
  return {
    id: "target",
    header: "Player",
    accessorFn: (ban) => ban.name || ban.fc || "",
    cell: ({ row }) => {
      const ban = row.original
      return (
        <div className="grid gap-0.5">
          <span className="flex items-center gap-1.5 font-medium">
            {ban.name || "-"}
            {ban.console_id ? (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Monitor className="size-3.5 text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>Whole console banned</TooltipContent>
              </Tooltip>
            ) : null}
          </span>
          <span className="font-mono text-xs text-muted-foreground">
            {[ban.fc && `FC ${ban.fc}`, showPid && ban.pid && `PID ${ban.pid}`].filter(Boolean).join(" · ") || "-"}
          </span>
        </div>
      )
    },
  }
}

export function banReasonColumn(): ColumnDef<BanRecord> {
  return {
    accessorKey: "reason",
    header: "Reason",
    cell: ({ row }) => <span className="line-clamp-3 max-w-72 text-sm">{row.original.reason || "-"}</span>,
  }
}

export function banPeriodColumn(): ColumnDef<BanRecord> {
  return {
    id: "period",
    header: "Period",
    accessorFn: (ban) => ban.issued || "",
    cell: ({ row }) => {
      const ban = row.original
      return (
        <div className="grid gap-0.5 text-xs whitespace-nowrap">
          <span>{fmtOptionalDate(ban.issued)}</span>
          <span className="text-muted-foreground">{isKickRecord(ban) ? banDurationText(ban) : ban.expires ? `until ${fmtDate(ban.expires)}` : "No end date"}</span>
        </div>
      )
    },
  }
}

export function banRemainingColumn(): ColumnDef<BanRecord> {
  return {
    id: "remaining",
    header: "Remaining",
    accessorFn: (ban) => remainingBanTime(ban),
    cell: ({ row }) => {
      const label = remainingBanTime(row.original)
      return label === "Revoked" || label === "Expired" ? (
        <Badge variant="outline" className="text-muted-foreground">
          {label}
        </Badge>
      ) : (
        <StatusBadge tone={remainingTone(label)} className="whitespace-nowrap">{label}</StatusBadge>
      )
    },
  }
}
