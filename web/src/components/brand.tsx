import { serverName } from "@/lib/hooks"
import { cn } from "@/lib/utils"

// LogoIcon is a light blue ring around a blue disc, with Wi-Fi waves pointing right.
export function LogoIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={cn("size-8 shrink-0", className)} aria-hidden="true">
      <circle cx="16" cy="16" r="16" fill="#A6E0F8" />
      <circle cx="16" cy="16" r="13" fill="#00A0E6" />
      <g fill="none" stroke="#fff" strokeWidth="2.2" strokeLinecap="round">
        <path d="M13.83 13.17A4 4 0 0 1 13.83 18.83" />
        <path d="M16.09 10.91A7.2 7.2 0 0 1 16.09 21.09" />
        <path d="M18.35 8.65A10.4 10.4 0 0 1 18.35 23.35" />
      </g>
      <circle cx="11" cy="16" r="1.9" fill="#fff" />
    </svg>
  )
}

export function BrandMark({ className }: { className?: string }) {
  return <LogoIcon className={className} />
}

export function Brand({ subtitle }: { subtitle: string }) {
  return (
    <a href="/" className="flex min-w-0 items-center gap-2.5">
      <BrandMark />
      <div className="grid min-w-0 leading-tight">
        <span className="truncate text-sm font-semibold">{serverName()}</span>
        <span className="truncate text-xs text-muted-foreground">{subtitle}</span>
      </div>
    </a>
  )
}
