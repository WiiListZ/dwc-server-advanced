import type { ColumnDef } from "@tanstack/react-table"
import { useEffect, useState } from "react"
import { toast } from "sonner"

import { DataTable } from "@/components/data-table"
import { SearchInput } from "@/components/filters"
import { PageHeading, SiteHeader } from "@/components/site-header"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { api, errorMessage, list, type ErrorCodeEntry } from "@/lib/api"
import { useSession } from "@/lib/hooks"

function matchesCode(codeValue: string, query: string) {
  const code = String(codeValue || "").toLowerCase().replace(/[^a-z0-9]/g, "")
  if (code.includes(query)) return true
  if (!/^\d+$/.test(query)) return false
  const range = String(codeValue || "").match(/(\d+)\D+(\d+)/)
  if (!range) return false
  const n = Number(query)
  return n >= Number(range[1]) && n <= Number(range[2])
}

function matchesError(row: ErrorCodeEntry, raw: string) {
  const query = raw.toLowerCase().replace(/[^a-z0-9]/g, "")
  if (!query) return true
  if (matchesCode(row.code, query)) return true
  return [row.title, row.description, row.action].some((v) => String(v || "").toLowerCase().includes(raw.toLowerCase()))
}

const columns = (actionLabel: string): ColumnDef<ErrorCodeEntry>[] => [
  { accessorKey: "code", header: "Code", cell: ({ row }) => <span className="font-mono font-medium">{row.original.code}</span> },
  { accessorKey: "title", header: "Meaning", cell: ({ row }) => row.original.title || "Error" },
  {
    id: "action",
    header: actionLabel,
    accessorFn: (r) => r.action || r.description,
    cell: ({ row }) => <span className="text-muted-foreground">{row.original.action || row.original.description || "-"}</span>,
  },
]

export function ErrorsPage() {
  const { session } = useSession()
  const [query, setQuery] = useState("")
  const [data, setData] = useState<{ custom_errors: ErrorCodeEntry[]; default_errors: ErrorCodeEntry[] }>({ custom_errors: [], default_errors: [] })
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api<typeof data>("/api/public/errors")
      .then((body) => setData({ custom_errors: list(body?.custom_errors), default_errors: list(body?.default_errors) }))
      .catch((error) => toast.error(errorMessage(error)))
      .finally(() => setLoading(false))
  }, [])

  const sections = [
    {
      title: "In-game custom errors",
      description: "Messages the game may show when this service blocks, disconnects or redirects a player.",
      rows: data.custom_errors,
      action: "What to do",
    },
    {
      title: "Game default error codes",
      description: "Common default codes shown by the game or the Wii online system.",
      rows: data.default_errors,
      action: "What to try",
    },
  ]

  return (
    <div className="min-h-svh bg-background">
      <SiteHeader session={session} page="errors" subtitle="Error code lookup" />
      <main className="mx-auto grid max-w-6xl gap-6 px-4 py-8">
        <PageHeading
          eyebrow="Help"
          title="Error codes"
          description="Search the code shown by the game: what it means and what to try next. Try an exact code like 20100, 23404 or 22002, or a word."
          actions={<SearchInput value={query} onChange={setQuery} placeholder="Error code or word" className="sm:w-80" />}
        />
        {sections.map((section) => (
          <Card key={section.title}>
            <CardHeader>
              <CardTitle>{section.title}</CardTitle>
              <CardDescription>{section.description}</CardDescription>
            </CardHeader>
            <CardContent>
              <DataTable
                columns={columns(section.action)}
                data={section.rows}
                loading={loading}
                globalFilter={query}
                filterFn={(row) => matchesError(row, query)}
                pageSize={25}
                emptyTitle="No error codes"
              />
            </CardContent>
          </Card>
        ))}
      </main>
    </div>
  )
}
