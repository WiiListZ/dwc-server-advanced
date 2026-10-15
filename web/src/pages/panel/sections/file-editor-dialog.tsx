import { Download, RefreshCw, Save } from "lucide-react"
import { Suspense, lazy, useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { useConfirm } from "@/components/confirm"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { ApiError, api, errorMessage } from "@/lib/api"
import { relativeTime } from "@/lib/format"

const CodeEditor = lazy(() => import("@/components/code-editor"))

type TextFile = { path: string; content: string; size: number; modified: string }

const fileUrl = (path: string) => `/api/panel/server-files/content?path=${encodeURIComponent(path)}`

/** Opens a text file of the server folder in an editor with highlighting. */
export function FileEditorDialog({ path, onClose, onSaved }: { path: string | null; onClose: () => void; onSaved: () => void }) {
  const { confirm } = useConfirm()
  const [file, setFile] = useState<TextFile | null>(null)
  const [draft, setDraft] = useState("")
  const [error, setError] = useState("")
  const [saving, setSaving] = useState(false)
  const name = path?.split("/").pop() ?? ""
  const dirty = !!file && draft !== file.content
  const isLog = name.toLowerCase().endsWith(".log")

  const load = useCallback(async (target: string) => {
    setError("")
    setFile(null)
    try {
      const loaded = await api<TextFile>(fileUrl(target))
      setFile(loaded)
      setDraft(loaded.content)
    } catch (err) {
      setError(errorMessage(err))
    }
  }, [])

  useEffect(() => {
    if (path) void load(path)
  }, [path, load])

  async function save(force = false) {
    if (!file || !path || saving) return
    setSaving(true)
    try {
      const saved = await api<TextFile>(fileUrl(path), { method: "PUT", body: { content: draft, modified: file.modified, force } })
      setFile(saved)
      toast.success(`${name} saved`, { description: name === "config.yml" ? "Restart the server to apply the new configuration." : undefined })
      onSaved()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409 && !force) {
        const ok = await confirm({
          title: "The file changed on the server",
          description: `${name} was modified after you opened it. Saving replaces those changes with yours.`,
          confirmLabel: "Save anyway",
          destructive: true,
        })
        if (ok) {
          setSaving(false)
          await save(true)
          return
        }
      } else {
        toast.error(errorMessage(err))
      }
    } finally {
      setSaving(false)
    }
  }

  async function close() {
    if (dirty) {
      const ok = await confirm({ title: "Discard your changes?", description: `${name} has unsaved changes.`, confirmLabel: "Discard", destructive: true })
      if (!ok) return
    }
    onClose()
  }

  return (
    <Dialog open={!!path} onOpenChange={(open) => !open && void close()}>
      <DialogContent className="flex h-[85vh] max-w-[calc(100%-2rem)] flex-col gap-3 sm:max-w-5xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 font-mono text-base">
            {path}
            {dirty && <Badge variant="secondary">Unsaved</Badge>}
          </DialogTitle>
          <DialogDescription>
            {file ? `${file.size.toLocaleString()} bytes · modified ${relativeTime(file.modified)}` : "Loading…"}
            {name === "config.yml" && " · Changes apply after a restart; settings saved from the panel rewrite this file."}
          </DialogDescription>
        </DialogHeader>
        <div className="min-h-0 flex-1 overflow-hidden rounded-lg border">
          {error ? (
            <p className="p-4 text-sm text-destructive">{error}</p>
          ) : !file ? (
            <Skeleton className="h-full w-full rounded-none" />
          ) : (
            <Suspense fallback={<Skeleton className="h-full w-full rounded-none" />}>
              <CodeEditor fileName={name} value={file.content} onChange={setDraft} onSave={() => void save()} scrollToEnd={isLog} />
            </Suspense>
          )}
        </div>
        <DialogFooter className="flex-wrap gap-2 sm:justify-between">
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={() => path && void load(path)} disabled={saving}>
              <RefreshCw data-icon="inline-start" /> Reload
            </Button>
            {path && (
              <Button asChild variant="ghost" size="sm">
                <a href={`/api/panel/server-files/download?path=${encodeURIComponent(path)}`}>
                  <Download data-icon="inline-start" /> Download
                </a>
              </Button>
            )}
          </div>
          <div className="flex gap-2">
            <Button variant="ghost" onClick={() => void close()}>
              Close
            </Button>
            <Button onClick={() => void save()} disabled={!dirty || saving}>
              {saving ? <Spinner data-icon="inline-start" /> : <Save data-icon="inline-start" />} Save
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
