import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands"
import { json } from "@codemirror/lang-json"
import { markdown } from "@codemirror/lang-markdown"
import { yaml } from "@codemirror/lang-yaml"
import { HighlightStyle, StreamLanguage, bracketMatching, defaultHighlightStyle, foldGutter, indentOnInput, syntaxHighlighting } from "@codemirror/language"
import { properties } from "@codemirror/legacy-modes/mode/properties"
import { EditorState, type Extension } from "@codemirror/state"
import { oneDark } from "@codemirror/theme-one-dark"
import {
  Decoration,
  EditorView,
  MatchDecorator,
  ViewPlugin,
  drawSelection,
  highlightActiveLine,
  highlightActiveLineGutter,
  keymap,
  lineNumbers,
  type DecorationSet,
  type ViewUpdate,
} from "@codemirror/view"
import { Tag, tags } from "@lezer/highlight"
import { search, searchKeymap } from "@codemirror/search"
import { useTheme } from "next-themes"
import { useEffect, useRef } from "react"

// Server log lines look like: 2026/10/15 19:12:50 N[NAS]: message
const logTags = {
  time: Tag.define(),
  notice: Tag.define(),
  error: Tag.define(),
  warn: Tag.define(),
  info: Tag.define(),
}

const logLanguage = StreamLanguage.define<{ start: boolean }>({
  name: "dwc-log",
  startState: () => ({ start: true }),
  token(stream, state) {
    if (stream.sol()) state.start = true
    if (stream.match(/^\x1b\[[0-9;]*m/)) return null
    if (state.start && stream.match(/^\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}:\d{2}/)) return "time"
    const level = stream.match(/^([NEWI])\[[^\]]*\]:?/)
    if (level && Array.isArray(level)) {
      state.start = false
      return { N: "notice", E: "error", W: "warn", I: "info" }[level[1] as "N" | "E" | "W" | "I"]
    }
    if (stream.match(/^(panic|fatal error|goroutine \d+)/i)) return "error"
    state.start = false
    stream.next()
    return null
  },
  tokenTable: { time: logTags.time, notice: logTags.notice, error: logTags.error, warn: logTags.warn, info: logTags.info },
})

const logHighlight = HighlightStyle.define([
  { tag: logTags.time, color: "var(--muted-foreground)" },
  { tag: logTags.notice, color: "#16a34a", fontWeight: "600" },
  { tag: logTags.error, color: "#dc2626", fontWeight: "600" },
  { tag: logTags.warn, color: "#d97706", fontWeight: "600" },
  { tag: logTags.info, color: "#0891b2", fontWeight: "600" },
  { tag: tags.comment, color: "var(--muted-foreground)" },
])

// The server colours its log lines with ANSI codes; hide them in the editor.
const ansiCodes = new MatchDecorator({ regexp: /\x1b\[[0-9;]*m/g, decoration: () => Decoration.replace({}) })
const hideAnsi = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet
    constructor(view: EditorView) {
      this.decorations = ansiCodes.createDeco(view)
    }
    update(update: ViewUpdate) {
      this.decorations = ansiCodes.updateDeco(update, this.decorations)
    }
  },
  { decorations: (v) => v.decorations, provide: (plugin) => EditorView.atomicRanges.of((view) => view.plugin(plugin)?.decorations ?? Decoration.none) },
)

export function languageFor(fileName: string): Extension[] {
  const name = fileName.toLowerCase()
  if (name.endsWith(".yml") || name.endsWith(".yaml")) return [yaml()]
  if (name.endsWith(".json")) return [json()]
  if (name.endsWith(".md")) return [markdown()]
  if (name.endsWith(".log")) return [logLanguage, syntaxHighlighting(logHighlight), hideAnsi]
  if (name.endsWith(".env") || name.endsWith(".ini") || name.endsWith(".conf") || name.endsWith(".cfg")) return [StreamLanguage.define(properties)]
  return []
}

const baseTheme = EditorView.theme({
  "&": { height: "100%", fontSize: "13px", backgroundColor: "transparent" },
  ".cm-scroller": { fontFamily: "var(--font-mono, ui-monospace, monospace)", lineHeight: "1.55" },
  ".cm-gutters": { backgroundColor: "transparent", borderRight: "1px solid var(--border)" },
  "&.cm-focused": { outline: "none" },
})

/** CodeMirror editor for the server's text files. */
export default function CodeEditor({
  fileName,
  value,
  onChange,
  onSave,
  readOnly = false,
  scrollToEnd = false,
}: {
  fileName: string
  value: string
  onChange: (value: string) => void
  onSave?: () => void
  readOnly?: boolean
  scrollToEnd?: boolean
}) {
  const host = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView | null>(null)
  const callbacks = useRef({ onChange, onSave })
  callbacks.current = { onChange, onSave }
  const { resolvedTheme } = useTheme()

  useEffect(() => {
    if (!host.current) return
    const dark = resolvedTheme !== "light"
    const editor = new EditorView({
      parent: host.current,
      state: EditorState.create({
        doc: value,
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          foldGutter(),
          history(),
          drawSelection(),
          indentOnInput(),
          bracketMatching(),
          highlightActiveLine(),
          search({ top: true }),
          keymap.of([
            { key: "Mod-s", preventDefault: true, run: () => (callbacks.current.onSave?.(), true) },
            indentWithTab,
            ...defaultKeymap,
            ...historyKeymap,
            ...searchKeymap,
          ]),
          baseTheme,
          dark ? oneDark : syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
          ...languageFor(fileName),
          EditorState.readOnly.of(readOnly),
          EditorView.lineWrapping,
          EditorView.updateListener.of((update) => {
            if (update.docChanged) callbacks.current.onChange(update.state.doc.toString())
          }),
        ],
      }),
    })
    view.current = editor
    if (scrollToEnd) {
      editor.dispatch({ effects: EditorView.scrollIntoView(editor.state.doc.length, { y: "end" }) })
    }
    return () => {
      editor.destroy()
      view.current = null
    }
    // The editor is rebuilt only for a new file, theme or mode; edits flow
    // through onChange.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fileName, resolvedTheme, readOnly])

  // Replace the content when the file is reloaded from the server.
  useEffect(() => {
    const editor = view.current
    if (editor && editor.state.doc.toString() !== value) {
      editor.dispatch({ changes: { from: 0, to: editor.state.doc.length, insert: value } })
    }
  }, [value])

  return <div ref={host} className="h-full min-h-0 overflow-hidden" />
}
