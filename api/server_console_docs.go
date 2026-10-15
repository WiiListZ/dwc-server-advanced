package api

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/logging"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
)

// HandlePanelServerConsole returns the console output of the server, the
// same lines the terminal shows. ?since=<seq> returns only newer lines.
func HandlePanelServerConsole(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireServerOwner(w, r); !ok || !allowMethod(w, r, http.MethodGet) {
		return
	}
	since, _ := strconv.ParseUint(r.URL.Query().Get("since"), 10, 64)

	// The frontend process holds the whole console, backend output included.
	// Without it, show at least what this process printed.
	source := "server"
	snapshot, err := common.ConsoleLines(since)
	if err != nil {
		source = "backend"
		snapshot = logging.ConsoleSince(since, 2000)
	}
	writePanelJSON(w, http.StatusOK, map[string]any{
		"source":  source,
		"lines":   snapshot.Lines,
		"next":    snapshot.Next,
		"dropped": snapshot.Dropped,
	})
}

// The documentation (README.md and docs/*.md) is built into the binary so
// the owner can read it in Panel → Server.

var documentation fs.FS

type documentInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// SetDocumentation provides the embedded README.md and docs/*.md files.
func SetDocumentation(files fs.FS) {
	documentation = files
}

func documentTitle(markdown, fallback string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
		// The README starts with an HTML title.
		if strings.HasPrefix(line, "<h1") {
			if start := strings.Index(line, ">"); start >= 0 {
				if end := strings.Index(line, "</h1>"); end > start {
					return strings.TrimSpace(line[start+1 : end])
				}
			}
		}
	}
	return fallback
}

func listDocuments() []documentInfo {
	docs := []documentInfo{}
	if documentation == nil {
		return docs
	}
	add := func(file, id string) {
		data, err := fs.ReadFile(documentation, file)
		if err != nil {
			return
		}
		docs = append(docs, documentInfo{ID: id, Title: documentTitle(string(data), file), Path: file})
	}
	add("README.md", "readme")
	if entries, err := fs.ReadDir(documentation, "docs"); err == nil {
		names := []string{}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				names = append(names, entry.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			add(path.Join("docs", name), strings.ToLower(strings.TrimSuffix(name, ".md")))
		}
	}
	return docs
}

// HandlePanelDocs lists the documents, or returns one with ?id=<id>.
func HandlePanelDocs(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireServerOwner(w, r); !ok || !allowMethod(w, r, http.MethodGet) {
		return
	}
	docs := listDocuments()
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writePanelJSON(w, http.StatusOK, docs)
		return
	}
	for _, doc := range docs {
		if doc.ID != id {
			continue
		}
		data, err := fs.ReadFile(documentation, doc.Path)
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "cannot read the document")
			return
		}
		writePanelJSON(w, http.StatusOK, map[string]any{"id": doc.ID, "title": doc.Title, "path": doc.Path, "markdown": string(data)})
		return
	}
	writePanelError(w, http.StatusNotFound, "document not found")
}
