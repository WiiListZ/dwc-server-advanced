import { useEffect, useState } from "react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldContent, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Spinner } from "@/components/ui/spinner"
import { Textarea } from "@/components/ui/textarea"
import { errorMessage, post, type Room, type RoomPlayer } from "@/lib/api"
import { playerIdentifier, playerName, roomType } from "@/lib/format"

export type KickTarget = { room: Room; player: RoomPlayer }

export function PrivateKickDialog({ target, onClose, onDone }: { target: KickTarget | null; onClose: () => void; onDone: () => void }) {
  const [reason, setReason] = useState("")
  const [blacklist, setBlacklist] = useState(false)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setReason("")
    setBlacklist(false)
  }, [target])

  const isPrivate = target ? roomType(target.room) === "private" : false

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    if (!target || !reason.trim()) return
    setBusy(true)
    try {
      const response = await post<{ open_host_block?: boolean }>("/api/panel/my/kick", {
        room_id: target.room.id,
        identifier: playerIdentifier(target.player),
        reason: reason.trim(),
        blacklist_open_host: isPrivate && blacklist,
      })
      toast.success(`Kick sent to ${playerName(target.player)}`, {
        description: response?.open_host_block ? "Open Host blacklist updated." : undefined,
      })
      onDone()
      onClose()
    } catch (error) {
      toast.error(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={!!target} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <form onSubmit={submit} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>Kick {target ? playerName(target.player) : "player"}</DialogTitle>
            <DialogDescription>
              Room <span className="font-mono">{target?.room.id}</span> · {target ? roomType(target.room) : ""}
            </DialogDescription>
          </DialogHeader>
          <Field>
            <FieldLabel htmlFor="kick-reason">Kick reason</FieldLabel>
            <Textarea id="kick-reason" required autoFocus maxLength={200} value={reason} onChange={(e) => setReason(e.target.value)} />
            <FieldDescription>{reason.length}/200</FieldDescription>
          </Field>
          {isPrivate && (
            <Field orientation="horizontal">
              <Checkbox id="kick-blacklist" checked={blacklist} onCheckedChange={(v) => setBlacklist(v === true)} />
              <FieldContent>
                <FieldLabel htmlFor="kick-blacklist">Blacklist in Open Host</FieldLabel>
                <FieldDescription>Switches your Open Host mode to Blacklist and adds this player to the block list.</FieldDescription>
              </FieldContent>
            </Field>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" variant="destructive" disabled={busy || !reason.trim()}>
              {busy && <Spinner />} Kick player
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
