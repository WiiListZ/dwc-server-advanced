package api

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/database"
	"embed"
	"html"
	"io/fs"
	"net/http"
	"strings"
	"unicode"
)

// dist holds the web panel built from web/ (npm run build). The build output
// is committed so `go build` works without Node.js.
//
//go:embed all:dist
var dist embed.FS

var (
	publicHTML   = mustReadPage("index.html")
	loginHTML    = mustReadPage("login.html")
	panelHTML    = mustReadPage("panel.html")
	settingsHTML = mustReadPage("settings.html")
	errorsHTML   = mustReadPage("errors.html")
	setupHTML    = mustReadPage("setup.html")
	assetServer  = newAssetServer()
)

func mustReadPage(name string) string {
	data, err := dist.ReadFile("dist/" + name)
	if err != nil {
		panic("web panel page missing from api/dist (run npm run build in web/): " + name)
	}
	return string(data)
}

func newAssetServer() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

// HandleAssets serves the hashed JS, CSS and font files of the web panel.
func HandleAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/") {
		http.NotFound(w, r)
		return
	}
	// File names contain a content hash, so they never change once published.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	assetServer.ServeHTTP(w, r)
}

func HandleRoot(w http.ResponseWriter, r *http.Request) {
	serveHTMLPage(w, r, "/", publicHTML)
}

// HandlePanel serves /panel/<section>. Sections the account cannot open, and
// unknown sections, answer with a plain 404 as if the page did not exist.
func HandlePanel(w http.ResponseWriter, r *http.Request) {
	account, ok := currentPanelAccount(r)
	if !ok || account.Role == "player" {
		http.NotFound(w, r)
		return
	}

	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "/panel" {
		http.Redirect(w, r, "/panel/dashboard", http.StatusFound)
		return
	}

	section := strings.TrimPrefix(path, "/panel/")
	if section == path || strings.Contains(section, "/") || !panelSectionAllowed(account, section) {
		http.NotFound(w, r)
		return
	}

	writeHTMLPage(w, r, panelHTML)
}

// panelSectionAllowed mirrors sectionAllowed in web/src/pages/panel/panel-context.tsx.
func panelSectionAllowed(account database.PanelAccount, section string) bool {
	switch section {
	case "dashboard":
		return true
	case "live":
		return panelAccountHasRight(account, panelRightViewUsers) || panelAccountHasAnyRoomRight(account)
	case "reports":
		return panelAccountHasRight(account, panelRightViewReports)
	case "bans":
		return panelAccountHasRight(account, panelRightViewBans)
	case "activity":
		return panelAccountHasRight(account, panelRightViewLogs)
	case "ghosts":
		return panelAccountHasRight(account, panelRightManageGhosts)
	case "games":
		return panelAccountHasRight(account, panelRightManageGames)
	case "team", "server":
		return account.Role == "owner"
	default:
		return false
	}
}

func HandleSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := currentPanelAccount(r); !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	serveHTMLPage(w, r, "/settings", settingsHTML)
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	serveHTMLPage(w, r, "/login", loginHTML)
}

func serveHTMLPage(w http.ResponseWriter, r *http.Request, basePath string, html string) {
	path := r.URL.Path
	if basePath == "/" {
		if path != "/" {
			http.NotFound(w, r)
			return
		}
	} else if path != basePath && path != basePath+"/" {
		http.NotFound(w, r)
		return
	}

	writeHTMLPage(w, r, html)
}

func writeHTMLPage(w http.ResponseWriter, r *http.Request, html string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	if r.Method == http.MethodHead {
		return
	}

	_, _ = w.Write([]byte(renderHTMLPage(html)))
}

func renderHTMLPage(pageHTML string) string {
	name := configuredServerName()
	replacements := map[string]string{
		"{{SERVER_NAME}}":     html.EscapeString(name),
		"{{SERVER_INITIALS}}": html.EscapeString(serverInitials(name)),
	}
	for token, value := range replacements {
		pageHTML = strings.ReplaceAll(pageHTML, token, value)
	}
	return pageHTML
}

func configuredServerName() string {
	name := strings.TrimSpace(common.GetConfig().ServerName)
	if name == "" {
		return "WFC Server"
	}
	return name
}

func serverInitials(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(words) > 1 {
		initials := make([]rune, 0, 2)
		for _, word := range words {
			for _, r := range word {
				initials = append(initials, unicode.ToUpper(r))
				break
			}
			if len(initials) == 2 {
				return string(initials)
			}
		}
	}

	initials := make([]rune, 0, 2)
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			initials = append(initials, unicode.ToUpper(r))
			if len(initials) == 2 {
				return string(initials)
			}
		}
	}
	if len(initials) == 0 {
		return "WF"
	}
	return string(initials)
}
