package api

import (
	"bytes"
	"dwc-server-advanced/database"
	"dwc-server-advanced/logging"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/logrusorgru/aurora/v3"
	"gopkg.in/yaml.v3"
)

// The owner manages the files in the server folder (certificates, payload,
// game list, …) from Panel → Server. Every operation goes through os.Root,
// so paths cannot leave the server folder, not even through symlinks.

const serverFilesMaxUpload = 512 << 20

type serverFileEntry struct {
	Name     string    `json:"name"`
	Dir      bool      `json:"dir"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	// Text is true for files the panel can open in its editor.
	Text bool `json:"text"`
}

type serverFilesListing struct {
	Root           string            `json:"root"`
	Path           string            `json:"path"`
	Entries        []serverFileEntry `json:"entries"`
	CanOpenLocally bool              `json:"canOpenLocally"`
}

func requireServerOwner(w http.ResponseWriter, r *http.Request) (database.PanelAccount, bool) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return account, false
	}
	if account.Role != "owner" {
		writePanelError(w, http.StatusForbidden, "owner account required")
		return account, false
	}
	return account, true
}

func serverFolder() (string, error) {
	return os.Getwd()
}

// cleanServerPath turns a slash path from the panel into a path relative to
// the server folder. "" and "/" are the folder itself.
func cleanServerPath(p string) (string, bool) {
	p = strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
	cleaned := strings.TrimPrefix(path.Clean("/"+p), "/")
	if cleaned == "" {
		return ".", true
	}
	for _, part := range strings.Split(cleaned, "/") {
		if part == ".." || strings.ContainsAny(part, ":") {
			return "", false
		}
	}
	return filepath.FromSlash(cleaned), true
}

func displayServerPath(rel string) string {
	if rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

func openServerRoot(w http.ResponseWriter) (*os.Root, string, bool) {
	dir, err := serverFolder()
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "cannot find the server folder")
		return nil, "", false
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "cannot open the server folder")
		return nil, "", false
	}
	return root, dir, true
}

func serverPathParam(w http.ResponseWriter, value string) (string, bool) {
	rel, ok := cleanServerPath(value)
	if !ok {
		writePanelError(w, http.StatusBadRequest, "invalid path")
	}
	return rel, ok
}

// canOpenLocally is true when "Open in file explorer" can work: the browser
// runs on the server machine itself (not through a proxy), and the server
// runs in a desktop session that can show a file manager.
func canOpenLocally(r *http.Request) bool {
	return requestFromThisMachine(r) && desktopAvailable()
}

// requestFromThisMachine is true when the client address is one of this
// machine's own addresses. Requests through a reverse proxy carry forwarding
// headers and never count, because the proxy is local but the user is not.
func requestFromThisMachine(r *http.Request) bool {
	for _, header := range []string{"X-Forwarded-For", "X-Real-Ip", "Forwarded", "Cf-Connecting-Ip"} {
		if r.Header.Get(header) != "" {
			return false
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.Equal(ip) {
			return true
		}
	}
	return false
}

// desktopAvailable is true when the server process can open a file manager
// window: not in a container, not a Windows service, and on Linux only with
// a graphical session.
func desktopAvailable() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return false
	}
	opener := ""
	switch runtime.GOOS {
	case "windows":
		// Services run without an interactive session and have no
		// SESSIONNAME; a window opened from there would never be seen.
		if os.Getenv("SESSIONNAME") == "" {
			return false
		}
		opener = "explorer.exe"
	case "darwin":
		opener = "open"
	case "linux", "freebsd":
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return false
		}
		opener = "xdg-open"
	default:
		return false
	}
	_, err := exec.LookPath(opener)
	return err == nil
}

func fileErrorStatus(err error) int {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, fs.ErrExist):
		return http.StatusConflict
	case errors.Is(err, fs.ErrPermission):
		return http.StatusForbidden
	}
	return http.StatusBadRequest
}

// HandlePanelServerFiles lists a folder (GET), or deletes a file or an empty
// folder (DELETE).
func HandlePanelServerFiles(w http.ResponseWriter, r *http.Request) {
	account, ok := requireServerOwner(w, r)
	if !ok {
		return
	}
	rel, ok := serverPathParam(w, r.URL.Query().Get("path"))
	if !ok {
		return
	}
	root, dir, ok := openServerRoot(w)
	if !ok {
		return
	}
	defer root.Close()

	switch r.Method {
	case http.MethodGet:
		folder, err := root.Open(rel)
		if err != nil {
			writePanelError(w, fileErrorStatus(err), "cannot open the folder: "+err.Error())
			return
		}
		defer folder.Close()
		items, err := folder.ReadDir(-1)
		if err != nil {
			writePanelError(w, http.StatusBadRequest, "not a folder")
			return
		}
		entries := make([]serverFileEntry, 0, len(items))
		for _, item := range items {
			info, err := item.Info()
			if err != nil {
				continue
			}
			entries = append(entries, serverFileEntry{
				Name:     item.Name(),
				Dir:      item.IsDir(),
				Size:     info.Size(),
				Modified: info.ModTime(),
				Text:     !item.IsDir() && isServerTextFile(item.Name()),
			})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Dir != entries[j].Dir {
				return entries[i].Dir
			}
			return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		})
		writePanelJSON(w, http.StatusOK, serverFilesListing{
			Root:           dir,
			Path:           displayServerPath(rel),
			Entries:        entries,
			CanOpenLocally: canOpenLocally(r),
		})

	case http.MethodDelete:
		if rel == "." {
			writePanelError(w, http.StatusBadRequest, "the server folder itself cannot be deleted")
			return
		}
		if err := root.Remove(rel); err != nil {
			message := "cannot delete: " + err.Error()
			if info, statErr := root.Lstat(rel); statErr == nil && info.IsDir() {
				message = "only empty folders can be deleted"
			}
			writePanelError(w, fileErrorStatus(err), message)
			return
		}
		logging.Notice("API", "Owner", aurora.Cyan(account.Username), "deleted", aurora.Cyan(displayServerPath(rel)), "from the server folder")
		writePanelJSON(w, http.StatusOK, map[string]bool{"ok": true})

	default:
		w.Header().Set("Allow", "GET, DELETE")
		writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// HandlePanelServerFileDownload sends one file of the server folder.
func HandlePanelServerFileDownload(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireServerOwner(w, r); !ok || !allowMethod(w, r, http.MethodGet) {
		return
	}
	rel, ok := serverPathParam(w, r.URL.Query().Get("path"))
	if !ok {
		return
	}
	root, _, ok := openServerRoot(w)
	if !ok {
		return
	}
	defer root.Close()

	file, err := root.Open(rel)
	if err != nil {
		writePanelError(w, fileErrorStatus(err), "cannot open the file")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		writePanelError(w, http.StatusBadRequest, "not a file")
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": info.Name()}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", info.ModTime(), file)
}

// HandlePanelServerFileUpload stores the files of a multipart upload in a
// folder of the server folder. Existing files are replaced only with
// overwrite=1.
func HandlePanelServerFileUpload(w http.ResponseWriter, r *http.Request) {
	account, ok := requireServerOwner(w, r)
	if !ok || !allowMethod(w, r, http.MethodPost) {
		return
	}
	rel, ok := serverPathParam(w, r.URL.Query().Get("path"))
	if !ok {
		return
	}
	overwrite := r.URL.Query().Get("overwrite") == "1"
	root, _, ok := openServerRoot(w)
	if !ok {
		return
	}
	defer root.Close()

	if info, err := root.Stat(rel); err != nil || !info.IsDir() {
		writePanelError(w, http.StatusBadRequest, "the target folder does not exist")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, serverFilesMaxUpload)
	multipartReader, err := r.MultipartReader()
	if err != nil {
		writePanelError(w, http.StatusBadRequest, "multipart upload required")
		return
	}

	saved := []string{}
	for {
		part, err := multipartReader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writePanelError(w, http.StatusRequestEntityTooLarge, "upload failed or larger than 512 MB")
			return
		}
		name := filepath.Base(filepath.FromSlash(strings.ReplaceAll(part.FileName(), "\\", "/")))
		if part.FormName() != "files" || name == "" || name == "." || name == ".." || strings.ContainsAny(name, ":") {
			part.Close()
			continue
		}
		target := filepath.Join(rel, name)

		flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		if !overwrite {
			flags |= os.O_EXCL
		}
		file, err := root.OpenFile(target, flags, 0o644)
		if err != nil {
			part.Close()
			if errors.Is(err, fs.ErrExist) {
				writePanelError(w, http.StatusConflict, name+" already exists")
			} else {
				writePanelError(w, fileErrorStatus(err), "cannot write "+name+": "+err.Error())
			}
			return
		}
		_, copyErr := io.Copy(file, part)
		closeErr := file.Close()
		part.Close()
		if copyErr != nil || closeErr != nil {
			_ = root.Remove(target)
			writePanelError(w, http.StatusRequestEntityTooLarge, "upload of "+name+" failed or is larger than 512 MB")
			return
		}
		saved = append(saved, displayServerPath(target))
	}

	if len(saved) == 0 {
		writePanelError(w, http.StatusBadRequest, "no files in the upload")
		return
	}
	logging.Notice("API", "Owner", aurora.Cyan(account.Username), "uploaded", aurora.Cyan(strings.Join(saved, ", ")), "to the server folder")
	writePanelJSON(w, http.StatusOK, map[string]any{"saved": saved})
}

// HandlePanelServerFolderCreate creates a folder in the server folder.
func HandlePanelServerFolderCreate(w http.ResponseWriter, r *http.Request) {
	account, ok := requireServerOwner(w, r)
	if !ok || !allowMethod(w, r, http.MethodPost) {
		return
	}
	req := struct {
		Path string `json:"path"`
	}{}
	if !readPanelJSON(w, r, &req) {
		return
	}
	rel, ok := serverPathParam(w, req.Path)
	if !ok {
		return
	}
	if rel == "." {
		writePanelError(w, http.StatusBadRequest, "folder name required")
		return
	}
	root, _, ok := openServerRoot(w)
	if !ok {
		return
	}
	defer root.Close()

	if err := root.Mkdir(rel, 0o755); err != nil {
		writePanelError(w, fileErrorStatus(err), "cannot create the folder: "+err.Error())
		return
	}
	logging.Notice("API", "Owner", aurora.Cyan(account.Username), "created folder", aurora.Cyan(displayServerPath(rel)), "in the server folder")
	writePanelJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// HandlePanelServerFolderOpen shows a folder of the server folder in the file
// manager of the server machine. Only allowed when the browser runs on it.
func HandlePanelServerFolderOpen(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireServerOwner(w, r); !ok || !allowMethod(w, r, http.MethodPost) {
		return
	}
	if !canOpenLocally(r) {
		writePanelError(w, http.StatusForbidden, "the folder can only be opened from a browser on the server machine")
		return
	}
	req := struct {
		Path string `json:"path"`
	}{}
	if !readPanelJSON(w, r, &req) {
		return
	}
	rel, ok := serverPathParam(w, req.Path)
	if !ok {
		return
	}
	root, dir, ok := openServerRoot(w)
	if !ok {
		return
	}
	info, err := root.Stat(rel)
	root.Close()
	if err != nil || !info.IsDir() {
		writePanelError(w, http.StatusNotFound, "folder not found")
		return
	}
	target := filepath.Join(dir, rel)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	// explorer.exe exits with code 1 even on success, so only a failed start
	// is an error.
	if err := cmd.Start(); err != nil {
		writePanelError(w, http.StatusInternalServerError, "cannot open the folder: "+err.Error())
		return
	}
	go func() { _ = cmd.Wait() }()
	writePanelJSON(w, http.StatusOK, map[string]string{"path": target})
}

const serverTextMaxSize = 2 << 20

// serverTextExtensions are the files the panel opens in its text editor.
var serverTextExtensions = map[string]bool{
	".txt": true, ".log": true, ".yml": true, ".yaml": true, ".json": true, ".md": true,
	".tsv": true, ".csv": true, ".env": true, ".ini": true, ".conf": true, ".cfg": true,
}

func isServerTextFile(name string) bool {
	lower := strings.ToLower(name)
	return serverTextExtensions[filepath.Ext(lower)] || lower == ".env"
}

type serverTextFile struct {
	Path     string    `json:"path"`
	Content  string    `json:"content"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// HandlePanelServerFileContent reads (GET) or saves (PUT) a text file of the
// server folder for the panel's editor.
func HandlePanelServerFileContent(w http.ResponseWriter, r *http.Request) {
	account, ok := requireServerOwner(w, r)
	if !ok {
		return
	}
	rel, ok := serverPathParam(w, r.URL.Query().Get("path"))
	if !ok {
		return
	}
	if rel == "." || !isServerTextFile(filepath.Base(rel)) {
		writePanelError(w, http.StatusBadRequest, "only text files (.txt, .log, .yml, .json, .md, .tsv, …) can be opened in the editor")
		return
	}
	root, _, ok := openServerRoot(w)
	if !ok {
		return
	}
	defer root.Close()

	switch r.Method {
	case http.MethodGet:
		info, err := root.Stat(rel)
		if err != nil {
			writePanelError(w, fileErrorStatus(err), "cannot open the file")
			return
		}
		if info.IsDir() {
			writePanelError(w, http.StatusBadRequest, "not a file")
			return
		}
		if info.Size() > serverTextMaxSize {
			writePanelError(w, http.StatusRequestEntityTooLarge, "the file is larger than 2 MB; download it instead")
			return
		}
		file, err := root.Open(rel)
		if err != nil {
			writePanelError(w, fileErrorStatus(err), "cannot open the file")
			return
		}
		data, err := io.ReadAll(io.LimitReader(file, serverTextMaxSize+1))
		file.Close()
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "cannot read the file")
			return
		}
		if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
			writePanelError(w, http.StatusUnsupportedMediaType, "this is not a text file; download it instead")
			return
		}
		writePanelJSON(w, http.StatusOK, serverTextFile{Path: displayServerPath(rel), Content: string(data), Size: info.Size(), Modified: info.ModTime()})

	case http.MethodPut:
		req := struct {
			Content string `json:"content"`
			// Modified is the time the editor loaded the file; a newer file
			// on disk means someone else changed it.
			Modified *time.Time `json:"modified"`
			Force    bool       `json:"force"`
		}{}
		r.Body = http.MaxBytesReader(w, r.Body, serverTextMaxSize*2)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writePanelError(w, http.StatusBadRequest, "invalid request or file larger than 2 MB")
			return
		}
		if len(req.Content) > serverTextMaxSize {
			writePanelError(w, http.StatusRequestEntityTooLarge, "the file must be 2 MB or smaller")
			return
		}
		if info, err := root.Stat(rel); err == nil {
			if info.IsDir() {
				writePanelError(w, http.StatusBadRequest, "not a file")
				return
			}
			if !req.Force && req.Modified != nil && info.ModTime().After(req.Modified.Add(time.Second)) {
				writePanelError(w, http.StatusConflict, "the file changed on the server since it was opened")
				return
			}
		}
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".yml" || ext == ".yaml" {
			var node yaml.Node
			if err := yaml.Unmarshal([]byte(req.Content), &node); err != nil {
				writePanelError(w, http.StatusBadRequest, "invalid YAML: "+err.Error())
				return
			}
		}
		if ext == ".json" && strings.TrimSpace(req.Content) != "" && !json.Valid([]byte(req.Content)) {
			writePanelError(w, http.StatusBadRequest, "invalid JSON")
			return
		}

		mode := fs.FileMode(0o644)
		if info, err := root.Stat(rel); err == nil {
			mode = info.Mode().Perm()
		}
		file, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			writePanelError(w, fileErrorStatus(err), "cannot write the file: "+err.Error())
			return
		}
		_, writeErr := io.WriteString(file, req.Content)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			writePanelError(w, http.StatusInternalServerError, "cannot write the file")
			return
		}
		info, _ := root.Stat(rel)
		logging.Notice("API", "Owner", aurora.Cyan(account.Username), "edited", aurora.Cyan(displayServerPath(rel)), "in the server folder")
		result := serverTextFile{Path: displayServerPath(rel), Content: req.Content, Size: int64(len(req.Content))}
		if info != nil {
			result.Modified = info.ModTime()
		}
		writePanelJSON(w, http.StatusOK, result)

	default:
		w.Header().Set("Allow", "GET, PUT")
		writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
