import { Search } from "lucide-react"

import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import type { BanFilter } from "@/lib/format"
import { cn } from "@/lib/utils"

export function SearchInput({
  value,
  onChange,
  placeholder,
  className,
}: {
  value: string
  onChange: (value: string) => void
  placeholder: string
  className?: string
}) {
  return (
    <InputGroup className={cn("w-full sm:w-72", className)}>
      <InputGroupAddon>
        <Search />
      </InputGroupAddon>
      <InputGroupInput type="search" value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} />
    </InputGroup>
  )
}

export function Segmented<T extends string>({
  value,
  onChange,
  options,
  className,
}: {
  value: T
  onChange: (value: T) => void
  options: { value: T; label: string; hidden?: boolean }[]
  className?: string
}) {
  return (
    <ToggleGroup
      type="single"
      variant="outline"
      size="sm"
      value={value}
      onValueChange={(next) => next && onChange(next as T)}
      className={cn("flex-wrap", className)}
    >
      {options
        .filter((option) => !option.hidden)
        .map((option) => (
          <ToggleGroupItem key={option.value} value={option.value} className="px-3">
            {option.label}
          </ToggleGroupItem>
        ))}
    </ToggleGroup>
  )
}

export function BanFilterToggle({ value, onChange, showMine }: { value: BanFilter; onChange: (v: BanFilter) => void; showMine: boolean }) {
  return (
    <Segmented
      value={value}
      onChange={onChange}
      options={[
        { value: "all", label: "All" },
        { value: "active", label: "Active" },
        { value: "expired", label: "Expired" },
        { value: "revoked", label: "Revoked" },
        { value: "kick", label: "Room kicks" },
        { value: "mine", label: "Mine", hidden: !showMine },
      ]}
    />
  )
}

export type RoomFilter = "all" | "public" | "private"

export function RoomFilterToggle({ value, onChange }: { value: RoomFilter; onChange: (v: RoomFilter) => void }) {
  return (
    <Segmented
      value={value}
      onChange={onChange}
      options={[
        { value: "all", label: "All" },
        { value: "public", label: "Public" },
        { value: "private", label: "Private" },
      ]}
    />
  )
}
