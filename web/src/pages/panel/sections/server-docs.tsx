import { BookOpen, ExternalLink, Image as ImageIcon } from "lucide-react"
import { Children, isValidElement, useCallback, useEffect, useRef, useState, type ReactNode } from "react"
import ReactMarkdown, { type Components } from "react-markdown"
import remarkGfm from "remark-gfm"
import { toast } from "sonner"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { api, errorMessage, list } from "@/lib/api"
import { cn } from "@/lib/utils"

type DocInfo = { id: string; title: string; path: string }
type Doc = DocInfo & { markdown: string }

const REPOSITORY = "https://github.com/WiiListZ/dwc-server-advanced/blob/main/"

function textOf(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node)
  if (Array.isArray(node)) return node.map(textOf).join("")
  if (isValidElement<{ children?: ReactNode }>(node)) return textOf(node.props.children)
  return ""
}

// slug matches GitHub's heading anchors, which the documents link to.
function slug(text: string) {
  return text
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s_-]/gu, "")
    .replace(/\s/g, "-")
}

function resolvePath(base: string, target: string) {
  const parts = base.split("/").slice(0, -1)
  for (const part of target.split("/")) {
    if (part === "..") parts.pop()
    else if (part && part !== ".") parts.push(part)
  }
  return parts.join("/")
}

/** The README and docs/ of the server, readable in the panel. */
export function ServerDocsCard() {
  const [docs, setDocs] = useState<DocInfo[] | null>(null)
  const [doc, setDoc] = useState<Doc | null>(null)
  const [pendingAnchor, setPendingAnchor] = useState("")
  const content = useRef<HTMLDivElement>(null)

  const open = useCallback(async (id: string, anchor = "") => {
    try {
      const loaded = await api<Doc>(`/api/panel/docs?id=${encodeURIComponent(id)}`)
      setDoc(loaded)
      setPendingAnchor(anchor)
    } catch (error) {
      toast.error(errorMessage(error))
    }
  }, [])

  useEffect(() => {
    api<DocInfo[]>("/api/panel/docs")
      .then((list_) => {
        const all = list(list_)
        setDocs(all)
        if (all.length) void open(all[0].id)
      })
      .catch((error) => toast.error(errorMessage(error)))
  }, [open])

  useEffect(() => {
    const el = content.current
    if (!el || !doc) return
    if (!pendingAnchor) {
      el.scrollTop = 0
      return
    }
    const target = el.querySelector(`#${CSS.escape(pendingAnchor)}`)
    if (target instanceof HTMLElement) el.scrollTop = target.offsetTop - el.offsetTop - 8
  }, [doc, pendingAnchor])

  function followLink(href: string) {
    if (!doc) return
    if (href.startsWith("#")) {
      setPendingAnchor("")
      window.requestAnimationFrame(() => setPendingAnchor(decodeURIComponent(href.slice(1))))
      return
    }
    const [target, anchor = ""] = href.split("#")
    const path = resolvePath(doc.path, target)
    const linked = docs?.find((d) => d.path === path)
    if (linked) {
      void open(linked.id, decodeURIComponent(anchor))
      return
    }
    window.open(REPOSITORY + path + (anchor ? `#${anchor}` : ""), "_blank", "noopener")
  }

  const heading =
    (Tag: "h1" | "h2" | "h3" | "h4", className: string) =>
    ({ children }: { children?: ReactNode }) => (
      <Tag id={slug(textOf(children))} className={className}>
        {children}
      </Tag>
    )

  const components: Components = {
    h1: heading("h1", "mt-2 mb-4 text-2xl font-semibold tracking-tight"),
    h2: heading("h2", "mt-8 mb-3 border-b pb-2 text-xl font-semibold tracking-tight"),
    h3: heading("h3", "mt-6 mb-2 text-lg font-semibold"),
    h4: heading("h4", "mt-4 mb-2 font-semibold"),
    p: ({ children }) => <p className="my-3 leading-7">{children}</p>,
    ul: ({ children }) => <ul className="my-3 ml-6 list-disc space-y-1">{children}</ul>,
    ol: ({ children }) => <ol className="my-3 ml-6 list-decimal space-y-1">{children}</ol>,
    li: ({ children }) => <li className="leading-7">{children}</li>,
    blockquote: ({ children }) => <blockquote className="my-4 border-l-2 pl-4 text-muted-foreground">{children}</blockquote>,
    hr: () => <hr className="my-6" />,
    table: ({ children }) => (
      <div className="my-4 overflow-x-auto rounded-lg border">
        <table className="w-full text-sm">{children}</table>
      </div>
    ),
    th: ({ children }) => <th className="border-b bg-muted/40 px-3 py-2 text-left font-medium">{children}</th>,
    td: ({ children }) => <td className="border-b px-3 py-2 align-top">{children}</td>,
    pre: ({ children }) => <pre className="my-4 overflow-x-auto rounded-lg border bg-muted/40 p-3 font-mono text-xs leading-relaxed">{children}</pre>,
    code: ({ className, children }) =>
      className ? (
        <code className={className}>{children}</code>
      ) : Children.toArray(children).some((c) => typeof c === "string" && c.includes("\n")) ? (
        <code>{children}</code>
      ) : (
        <code className="rounded bg-muted px-1 py-0.5 font-mono text-[0.85em]">{children}</code>
      ),
    a: ({ href = "", children }) => {
      const external = /^[a-z]+:/i.test(href)
      return (
        <a
          href={external ? href : `#${href.replace(/^#/, "")}`}
          target={external ? "_blank" : undefined}
          rel={external ? "noopener noreferrer" : undefined}
          className="font-medium underline underline-offset-4"
          onClick={(e) => {
            if (external) return
            e.preventDefault()
            followLink(href)
          }}
        >
          {children}
          {external && <ExternalLink className="ml-0.5 inline size-3" />}
        </a>
      )
    },
    // Screenshots are not built into the server; link to them on GitHub.
    img: ({ src = "", alt = "" }) => {
      const url = typeof src === "string" && /^[a-z]+:/i.test(src) ? src : REPOSITORY + resolvePath(doc?.path ?? "", String(src))
      return (
        <a href={url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-xs text-muted-foreground underline underline-offset-4">
          <ImageIcon className="size-3" /> {alt || "Image"}
        </a>
      )
    },
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <BookOpen className="size-4" /> Documentation
        </CardTitle>
        <CardDescription>Guides for setting up and running the server, from the version that is running now.</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-6 md:grid-cols-[220px_1fr]">
        <nav className="flex gap-1 overflow-x-auto md:flex-col" aria-label="Documents">
          {!docs ? (
            <Skeleton className="h-40" />
          ) : (
            docs.map((d) => (
              <button
                key={d.id}
                type="button"
                onClick={() => void open(d.id)}
                className={cn(
                  "rounded-lg px-3 py-2 text-left text-sm whitespace-nowrap text-muted-foreground transition-colors hover:bg-muted/60 md:whitespace-normal",
                  doc?.id === d.id && "bg-muted font-medium text-foreground",
                )}
              >
                {d.title}
                <span className="block font-mono text-[11px] font-normal text-muted-foreground">{d.path}</span>
              </button>
            ))
          )}
        </nav>
        <div ref={content} className="relative max-h-[75vh] min-w-0 overflow-y-auto pr-2 text-sm">
          {!doc ? (
            <Skeleton className="h-96" />
          ) : (
            <article>
              <ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml components={components}>
                {doc.markdown}
              </ReactMarkdown>
            </article>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
