import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"

type ConfirmOptions = {
  title: string
  description?: ReactNode
  confirmLabel?: string
  destructive?: boolean
}

type PromptOptions = {
  title: string
  description?: ReactNode
  label: string
  placeholder?: string
  confirmLabel?: string
  maxLength?: number
}

type ConfirmContextValue = {
  confirm: (options: ConfirmOptions) => Promise<boolean>
  prompt: (options: PromptOptions) => Promise<string | null>
}

const ConfirmContext = createContext<ConfirmContextValue | null>(null)

// ConfirmProvider replaces window.confirm/window.prompt with shadcn dialogs.
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [confirmState, setConfirmState] = useState<ConfirmOptions | null>(null)
  const [promptState, setPromptState] = useState<PromptOptions | null>(null)
  const [promptValue, setPromptValue] = useState("")
  const resolver = useRef<((value: boolean) => void) | null>(null)
  const promptResolver = useRef<((value: string | null) => void) | null>(null)

  const confirm = useCallback((options: ConfirmOptions) => {
    setConfirmState(options)
    return new Promise<boolean>((resolve) => {
      resolver.current = resolve
    })
  }, [])

  const prompt = useCallback((options: PromptOptions) => {
    setPromptValue("")
    setPromptState(options)
    return new Promise<string | null>((resolve) => {
      promptResolver.current = resolve
    })
  }, [])

  const closeConfirm = (value: boolean) => {
    resolver.current?.(value)
    resolver.current = null
    setConfirmState(null)
  }

  const closePrompt = (value: string | null) => {
    promptResolver.current?.(value)
    promptResolver.current = null
    setPromptState(null)
  }

  return (
    <ConfirmContext.Provider value={{ confirm, prompt }}>
      {children}
      <AlertDialog open={!!confirmState} onOpenChange={(open) => !open && closeConfirm(false)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirmState?.title}</AlertDialogTitle>
            {confirmState?.description && <AlertDialogDescription>{confirmState.description}</AlertDialogDescription>}
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => closeConfirm(false)}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant={confirmState?.destructive ? "destructive" : "default"}
              onClick={() => closeConfirm(true)}
            >
              {confirmState?.confirmLabel ?? "Continue"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      <Dialog open={!!promptState} onOpenChange={(open) => !open && closePrompt(null)}>
        <DialogContent>
          <form
            className="grid gap-4"
            onSubmit={(event) => {
              event.preventDefault()
              if (promptValue.trim()) closePrompt(promptValue.trim())
            }}
          >
            <DialogHeader>
              <DialogTitle>{promptState?.title}</DialogTitle>
              {promptState?.description && <DialogDescription>{promptState.description}</DialogDescription>}
            </DialogHeader>
            <Field>
              <FieldLabel htmlFor="prompt-value">{promptState?.label}</FieldLabel>
              <Textarea
                id="prompt-value"
                autoFocus
                value={promptValue}
                maxLength={promptState?.maxLength}
                placeholder={promptState?.placeholder}
                onChange={(event) => setPromptValue(event.target.value)}
              />
              {promptState?.maxLength && (
                <FieldDescription>
                  {promptValue.length}/{promptState.maxLength}
                </FieldDescription>
              )}
            </Field>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => closePrompt(null)}>
                Cancel
              </Button>
              <Button type="submit" disabled={!promptValue.trim()}>
                {promptState?.confirmLabel ?? "Submit"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </ConfirmContext.Provider>
  )
}

export function useConfirm() {
  const ctx = useContext(ConfirmContext)
  if (!ctx) throw new Error("useConfirm must be used inside ConfirmProvider")
  return ctx
}
