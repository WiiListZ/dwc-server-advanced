import { useEffect, useState } from "react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { errorMessage, post } from "@/lib/api"
import { usePanel } from "@/pages/panel/panel-context"

export type ForceJoin = { identifier: string; roomId: string; expires: string }

export function JoinRoomDialog({ onStarted }: { onStarted: (join: ForceJoin) => void }) {
  const { joinRoomId, closeJoinDialog, account, refreshDashboard } = usePanel()
  const [identifier, setIdentifier] = useState("")
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (joinRoomId !== null) setIdentifier(account.fc || (account.pid ? String(account.pid) : ""))
  }, [joinRoomId, account])

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    if (!joinRoomId || !identifier.trim()) return
    setBusy(true)
    try {
      const response = await post<{ room_id?: string; expires: string }>("/api/panel/join-room", { room_id: joinRoomId, identifier: identifier.trim() })
      onStarted({ identifier: identifier.trim(), roomId: response.room_id || joinRoomId, expires: response.expires })
      toast.success("Force join started", { description: `${identifier.trim()} → ${response.room_id || joinRoomId}` })
      closeJoinDialog()
      void refreshDashboard()
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={joinRoomId !== null} onOpenChange={(open) => !open && closeJoinDialog()}>
      <DialogContent>
        <form onSubmit={submit} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>Join room</DialogTitle>
            <DialogDescription>For the next 2 minutes, the profile below is pulled into this room when it searches for a match.</DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="join-room">Room ID</FieldLabel>
              <Input id="join-room" readOnly value={joinRoomId ?? ""} className="font-mono" />
            </Field>
            <Field>
              <FieldLabel htmlFor="join-identifier">Friend code or profile ID</FieldLabel>
              <Input id="join-identifier" autoFocus required value={identifier} onChange={(e) => setIdentifier(e.target.value)} />
              {(account.fc || account.pid) && <FieldDescription>Prefilled with your linked profile.</FieldDescription>}
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={closeJoinDialog}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy || !identifier.trim()}>
              {busy && <Spinner />} Start 2-minute join
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
