import { ChevronRight, Download, File, FileText, Folder, FolderOpen, FolderPlus, HardDrive, Pencil, RefreshCw, Trash2, Upload } from "lucide-react"
import { useCallback, useEffect, useRef, useState } from "react"
import { toast } from "sonner"

import { useConfirm } from "@/components/confirm"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Empty, EmptyDescription, EmptyHeader, EmptyTitle } from "@/components/ui/empty"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { ApiError, api, errorMessage, list } from "@/lib/api"
import { relativeTime } from "@/lib/format"
import { cn } from "@/lib/utils"
import { FileEditorDialog } from "@/pages/panel/sections/file-editor-dialog"

type Entry = { name: string; dir: boolean; size: number; modified: string; text?: boolean }
type Listing = { root: string; path: string; entries: Entry[]; canOpenLocally: boolean }

const join = (dir: string, name: string) => (dir ? `${dir}/${name}` : name)

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  const units = ["KB", "MB", "GB"]
  let value = bytes / 1024
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  return `${value.toFixed(value < 10 ? 1 : 0)} ${units[unit]}`
}

async function uploadFiles(dir: string, files: File[], overwrite: boolean) {
  const form = new FormData()
  for (const file of files) form.append("files", file, file.name)
  const params = new URLSearchParams({ path: dir })
  if (overwrite) params.set("overwrite", "1")
  const response = await fetch(`/api/panel/server-files/upload?${params}`, { method: "POST", credentials: "same-origin", body: form })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new ApiError(data?.error || `HTTP ${response.status}`, response.status)
  return data as { saved: string[] }
}

/** File manager for the server folder (certificates, payload, game list, …). */
export function ServerFilesCard() {
  const { confirm, prompt } = useConfirm()
  const [path, setPath] = useState("")
  const [listing, setListing] = useState<Listing | null>(null)
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [dragging, setDragging] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const [editing, setEditing] = useState<string | null>(null)

  const load = useCallback(async (target: string) => {
    setLoading(true)
    try {
      const result = await api<Listing>(`/api/panel/server-files?path=${encodeURIComponent(target)}`)
      setListing({ ...result, entries: list(result.entries) })
      setPath(result.path)
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load("")
  }, [load])

  async function upload(files: File[]) {
    if (!files.length) return
    setUploading(true)
    try {
      let result
      try {
        result = await uploadFiles(path, files, false)
      } catch (error) {
        if (!(error instanceof ApiError) || error.status !== 409) throw error
        const ok = await confirm({ title: "Replace existing files?", description: `${error.message}. Uploading replaces the files with the same name in this folder.`, confirmLabel: "Replace", destructive: true })
        if (!ok) return
        result = await uploadFiles(path, files, true)
      }
      toast.success(`Uploaded ${result.saved.length} file${result.saved.length === 1 ? "" : "s"}`)
      void load(path)
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setUploading(false)
      if (input.current) input.current.value = ""
    }
  }

  async function createFolder() {
    const name = await prompt({ title: "New folder", label: "Folder name", placeholder: "payload", confirmLabel: "Create", maxLength: 120 })
    if (!name?.trim()) return
    try {
      await api("/api/panel/server-files/mkdir", { method: "POST", body: { path: join(path, name.trim()) } })
      void load(path)
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  async function remove(entry: Entry) {
    const ok = await confirm({
      title: `Delete ${entry.name}?`,
      description: entry.dir ? "Only empty folders can be deleted." : "The file is deleted from the server. This cannot be undone.",
      confirmLabel: "Delete",
      destructive: true,
    })
    if (!ok) return
    try {
      await api(`/api/panel/server-files?path=${encodeURIComponent(join(path, entry.name))}`, { method: "DELETE" })
      toast.success(`${entry.name} deleted`)
      void load(path)
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  async function openLocally() {
    try {
      await api("/api/panel/server-files/open", { method: "POST", body: { path } })
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }

  const crumbs = path ? path.split("/") : []

  return (
    <Card
      onDragOver={(e) => {
        // Drags inside the editor dialog bubble up here through React.
        if (editing || !e.dataTransfer.types.includes("Files")) return
        e.preventDefault()
        setDragging(true)
      }}
      onDragLeave={() => setDragging(false)}
      onDrop={(e) => {
        if (editing || !e.dataTransfer.files.length) return
        e.preventDefault()
        setDragging(false)
        void upload(Array.from(e.dataTransfer.files))
      }}
      className={cn(dragging && "ring-2 ring-ring")}
    >
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <HardDrive className="size-4" /> Server folder
        </CardTitle>
        <CardDescription>
          Files of the server{listing ? <>, in <code className="font-mono break-all">{listing.root}</code></> : null}. Put certificates, the payload/ build and game_list.tsv here. Drop files on this card to upload them.
        </CardDescription>
        <CardAction className="flex flex-wrap justify-end gap-2">
          {listing?.canOpenLocally && (
            <Button variant="outline" size="sm" onClick={() => void openLocally()}>
              <FolderOpen data-icon="inline-start" /> Open in file explorer
            </Button>
          )}
          <Button variant="ghost" size="icon-sm" aria-label="Refresh" onClick={() => void load(path)}>
            <RefreshCw />
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className="grid gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <nav className="flex min-w-0 flex-1 flex-wrap items-center gap-0.5 text-sm" aria-label="Folder">
            <Button variant="ghost" size="xs" onClick={() => void load("")}>
              Server folder
            </Button>
            {crumbs.map((crumb, i) => (
              <span key={i} className="flex items-center gap-0.5">
                <ChevronRight className="size-3.5 text-muted-foreground" />
                <Button variant="ghost" size="xs" onClick={() => void load(crumbs.slice(0, i + 1).join("/"))}>
                  {crumb}
                </Button>
              </span>
            ))}
          </nav>
          <input ref={input} type="file" multiple className="hidden" onChange={(e) => void upload(Array.from(e.target.files ?? []))} />
          <Button variant="outline" size="sm" onClick={() => void createFolder()}>
            <FolderPlus data-icon="inline-start" /> New folder
          </Button>
          <Button size="sm" onClick={() => input.current?.click()} disabled={uploading}>
            {uploading ? <Spinner data-icon="inline-start" /> : <Upload data-icon="inline-start" />} Upload files
          </Button>
        </div>

        {loading && !listing ? (
          <Skeleton className="h-48" />
        ) : !listing?.entries.length ? (
          <Empty className="border">
            <EmptyHeader>
              <EmptyTitle>Empty folder</EmptyTitle>
              <EmptyDescription>Upload files or drop them here.</EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <div className="overflow-hidden rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead className="hidden w-28 text-right sm:table-cell">Size</TableHead>
                  <TableHead className="hidden w-40 md:table-cell">Modified</TableHead>
                  <TableHead className="w-32" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {listing.entries.map((entry) => (
                  <TableRow key={entry.name}>
                    <TableCell className="max-w-0">
                      {entry.dir ? (
                        <button type="button" className="flex min-w-0 items-center gap-2 font-medium hover:underline" onClick={() => void load(join(path, entry.name))}>
                          <Folder className="size-4 shrink-0 text-muted-foreground" />
                          <span className="truncate">{entry.name}</span>
                        </button>
                      ) : entry.text ? (
                        <button type="button" className="flex min-w-0 items-center gap-2 hover:underline" onClick={() => setEditing(join(path, entry.name))}>
                          <FileText className="size-4 shrink-0 text-muted-foreground" />
                          <span className="truncate">{entry.name}</span>
                        </button>
                      ) : (
                        <span className="flex min-w-0 items-center gap-2">
                          <File className="size-4 shrink-0 text-muted-foreground" />
                          <span className="truncate">{entry.name}</span>
                        </span>
                      )}
                    </TableCell>
                    <TableCell className="hidden text-right text-muted-foreground tabular-nums sm:table-cell">{entry.dir ? "" : formatSize(entry.size)}</TableCell>
                    <TableCell className="hidden text-muted-foreground md:table-cell">{relativeTime(entry.modified)}</TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        {entry.text && (
                          <Button variant="ghost" size="icon-sm" aria-label={`Edit ${entry.name}`} onClick={() => setEditing(join(path, entry.name))}>
                            <Pencil />
                          </Button>
                        )}
                        {!entry.dir && (
                          <Button asChild variant="ghost" size="icon-sm" aria-label={`Download ${entry.name}`}>
                            <a href={`/api/panel/server-files/download?path=${encodeURIComponent(join(path, entry.name))}`}>
                              <Download />
                            </a>
                          </Button>
                        )}
                        <Button variant="ghost" size="icon-sm" aria-label={`Delete ${entry.name}`} onClick={() => void remove(entry)}>
                          <Trash2 />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
      <FileEditorDialog path={editing} onClose={() => setEditing(null)} onSaved={() => void load(path)} />
    </Card>
  )
}
