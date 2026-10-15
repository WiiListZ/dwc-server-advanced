package api

import (
	"context"
	"crypto/subtle"
	"dwc-server-advanced/common"
	"dwc-server-advanced/database"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Setup mode runs before config.yml exists, so nothing here may call
// common.GetConfig.

type setupDatabaseRequest struct {
	DatabaseAddress string `json:"databaseAddress"`
	DatabaseName    string `json:"databaseName"`
	Username        string `json:"username"`
	Password        string `json:"password"`
}

type setupCompleteRequest struct {
	Settings serverSettings          `json:"settings"`
	Owner    panelCredentialsRequest `json:"owner"`
	DataDir  string                  `json:"dataDir"`
}

// SetupOptions configures the first-run wizard.
type SetupOptions struct {
	Address string
	Token   string
	Schema  string
	// DataDir is the folder the server started in.
	DataDir string
	// DataDirFixed is true when --dir or DWC_HOME chose the folder, so the
	// wizard cannot move it.
	DataDirFixed bool
}

type setupFolderStatus struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Writable  bool   `json:"writable"`
	HasConfig bool   `json:"hasConfig"`
	Entries   int    `json:"entries"`
	Error     string `json:"error,omitempty"`
}

// resolveSetupFolder turns the folder typed in the wizard into an absolute
// path, relative paths being relative to the folder the server started in.
func resolveSetupFolder(opts SetupOptions, path string) string {
	path = strings.TrimSpace(path)
	if path == "" || opts.DataDirFixed {
		return opts.DataDir
	}
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(opts.DataDir, path)
	}
	return filepath.Clean(path)
}

// prepareSetupFolder creates the folder when asked and checks that the server
// can write to it.
func prepareSetupFolder(path string, create bool) setupFolderStatus {
	status := setupFolderStatus{Path: path}
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		status.Error = "this path is a file, not a folder"
		return status
	}
	if err != nil {
		if !create {
			return status
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			status.Error = "cannot create the folder: " + err.Error()
			return status
		}
	}
	status.Exists = true

	probe := filepath.Join(path, ".dwc-write-test")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		status.Error = "the server cannot write to this folder: " + err.Error()
		return status
	}
	_ = os.Remove(probe)
	status.Writable = true

	if entries, err := os.ReadDir(path); err == nil {
		status.Entries = len(entries)
	}
	for _, name := range []string{"config.yml", "config.yaml", "config.xml"} {
		if _, err := os.Stat(filepath.Join(path, name)); err == nil {
			status.HasConfig = true
		}
	}
	return status
}

// SetupDefaults are the values the wizard starts from.
func SetupDefaults() serverSettings {
	s := settingsFromConfig(common.DefaultConfig())
	s.ServerName = "DWC Server Advanced"
	s.GSAddress, s.NASAddress, s.NASAddressHTTPS = "0.0.0.0", "0.0.0.0", "0.0.0.0"
	s.NASPort, s.NASPortHTTPS = "80", "443"
	s.CertPath, s.KeyPath = "fullchain.pem", "privkey.pem"
	s.CertDerPathWii, s.KeyPathWii = "naswii-chain.der", "naswii-signed-key.pem"
	s.CertDerPathDS, s.WiiCertDerPathDS, s.KeyPathDS = "nas-cert.der", "nwc.der", "nas-key.pem"
	s.DatabaseAddress, s.DatabaseName, s.Username = "127.0.0.1", "wwfc", ""
	s.LogOutput = "StdOutAndFile"
	s.LogEventsToDatabase = true
	s.UpdatesEnabled = true
	return s
}

// RunSetup serves the first-run wizard on address until the administrator
// finishes it and config.yml has been written.
func RunSetup(opts SetupOptions) error {
	address, token, schema := opts.Address, opts.Token, opts.Schema
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	var finish sync.Once
	authorized := func(w http.ResponseWriter, r *http.Request) bool {
		got := r.Header.Get("X-Setup-Token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			writePanelError(w, http.StatusUnauthorized, "invalid setup token: open the link printed in the server console")
			return false
		}
		return true
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/assets/", HandleAssets)

	mux.HandleFunc("/api/setup/state", func(w http.ResponseWriter, r *http.Request) {
		if !allowMethod(w, r, http.MethodGet) || !authorized(w, r) {
			return
		}
		writePanelJSON(w, http.StatusOK, map[string]any{
			"version":      common.Version,
			"defaults":     SetupDefaults(),
			"addresses":    localAddresses(),
			"port":         portOf(address),
			"dataDir":      opts.DataDir,
			"dataDirFixed": opts.DataDirFixed,
			"docker":       checkDocker(),
		})
	})

	mux.HandleFunc("/api/setup/folder", func(w http.ResponseWriter, r *http.Request) {
		if !allowMethod(w, r, http.MethodPost) || !authorized(w, r) {
			return
		}
		req := struct {
			Path   string `json:"path"`
			Create bool   `json:"create"`
		}{}
		if !readPanelJSON(w, r, &req) {
			return
		}
		writePanelJSON(w, http.StatusOK, prepareSetupFolder(resolveSetupFolder(opts, req.Path), req.Create))
	})

	mux.HandleFunc("/api/setup/docker", func(w http.ResponseWriter, r *http.Request) {
		if !allowMethod(w, r, http.MethodGet) || !authorized(w, r) {
			return
		}
		writePanelJSON(w, http.StatusOK, checkDocker())
	})

	mux.HandleFunc("/api/setup/docker-database", func(w http.ResponseWriter, r *http.Request) {
		if !allowMethod(w, r, http.MethodPost) || !authorized(w, r) {
			return
		}
		req := struct {
			DataDir string `json:"dataDir"`
		}{}
		if !readPanelJSON(w, r, &req) {
			return
		}
		dir := resolveSetupFolder(opts, req.DataDir)
		if status := prepareSetupFolder(dir, true); status.Error != "" {
			writePanelError(w, http.StatusBadRequest, status.Error)
			return
		}
		result, err := createDockerDatabase(dir)
		if err != nil {
			writePanelError(w, http.StatusBadGateway, err.Error())
			return
		}
		writePanelJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("/api/setup/test-database", func(w http.ResponseWriter, r *http.Request) {
		if !allowMethod(w, r, http.MethodPost) || !authorized(w, r) {
			return
		}
		req := setupDatabaseRequest{}
		if !readPanelJSON(w, r, &req) {
			return
		}
		cfg := common.DefaultConfig()
		cfg.DatabaseAddress, cfg.DatabaseName, cfg.Username, cfg.Password = strings.TrimSpace(req.DatabaseAddress), strings.TrimSpace(req.DatabaseName), strings.TrimSpace(req.Username), req.Password
		conn, err := database.TryConnect(cfg, 5*time.Second)
		if err != nil {
			writePanelJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		defer conn.Close()

		hasSchema, _ := conn.HasSchema()
		hasOwner := false
		if hasSchema {
			if err := conn.UpdatePanelTables(); err == nil {
				hasOwner, _ = conn.HasPanelAccounts()
			}
		}
		writePanelJSON(w, http.StatusOK, map[string]any{"ok": true, "hasSchema": hasSchema, "hasOwner": hasOwner})
	})

	mux.HandleFunc("/api/setup/complete", func(w http.ResponseWriter, r *http.Request) {
		if !allowMethod(w, r, http.MethodPost) || !authorized(w, r) {
			return
		}
		req := setupCompleteRequest{}
		if !readPanelJSON(w, r, &req) {
			return
		}

		cfg, err := applySettings(common.DefaultConfig(), req.Settings)
		if err != nil {
			writePanelError(w, http.StatusBadRequest, err.Error())
			return
		}
		req.Owner.Username = strings.TrimSpace(req.Owner.Username)
		if req.Owner.Username != "" || req.Owner.Password != "" {
			if msg := validateOwner(req.Owner); msg != "" {
				writePanelError(w, http.StatusBadRequest, msg)
				return
			}
		}

		dataDir := resolveSetupFolder(opts, req.DataDir)
		if status := prepareSetupFolder(dataDir, true); status.Error != "" {
			writePanelError(w, http.StatusBadRequest, "server folder: "+status.Error)
			return
		}

		conn, err := database.TryConnect(cfg, 10*time.Second)
		if err != nil {
			writePanelError(w, http.StatusBadRequest, "cannot connect to the database: "+err.Error())
			return
		}
		defer conn.Close()

		if hasSchema, err := conn.HasSchema(); err != nil {
			writePanelError(w, http.StatusInternalServerError, "cannot read the database: "+err.Error())
			return
		} else if !hasSchema {
			if err := conn.ApplySchema(schema); err != nil {
				writePanelError(w, http.StatusInternalServerError, "cannot create the database tables: "+err.Error())
				return
			}
		}
		conn.UpdateTables()
		if err := conn.UpdatePanelTables(); err != nil {
			writePanelError(w, http.StatusInternalServerError, "cannot create the panel tables: "+err.Error())
			return
		}

		ownerCreated := false
		hasOwner, err := conn.HasPanelAccounts()
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "cannot read the panel accounts: "+err.Error())
			return
		}
		if !hasOwner {
			owner := req.Owner
			if msg := validateOwner(owner); msg != "" {
				writePanelError(w, http.StatusBadRequest, msg)
				return
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(owner.Password), bcrypt.DefaultCost)
			if err != nil {
				writePanelError(w, http.StatusInternalServerError, "failed to hash the password")
				return
			}
			if _, err := conn.CreatePanelAccount(owner.Username, hash, "owner"); err != nil {
				writePanelError(w, http.StatusConflict, "cannot create the owner account: "+err.Error())
				return
			}
			ownerCreated = true
		}

		// Everything the server writes from now on (config.yml, logs, state,
		// payload) goes to the chosen folder.
		if err := os.Chdir(dataDir); err != nil {
			writePanelError(w, http.StatusInternalServerError, "cannot use the server folder: "+err.Error())
			return
		}
		_ = os.Setenv("DWC_HOME", dataDir)
		if !opts.DataDirFixed {
			common.WriteHomePointers(dataDir, opts.DataDir, common.ExecutableDir())
		}

		if err := common.SaveConfig(cfg); err != nil {
			writePanelError(w, http.StatusInternalServerError, "cannot write config.yml: "+err.Error())
			return
		}

		writePanelJSON(w, http.StatusOK, map[string]any{"ok": true, "ownerCreated": ownerCreated, "nasPort": cfg.NASPort, "dataDir": dataDir})
		finish.Do(func() {
			// Let the response reach the browser before the port is released.
			time.AfterFunc(500*time.Millisecond, func() { close(done) })
		})
	})

	page := renderSetupPage()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writePanelError(w, http.StatusServiceUnavailable, "the server is waiting for its first-run setup")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, page)
	})

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()

	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func validateOwner(owner panelCredentialsRequest) string {
	if owner.Username == "" || len(owner.Username) > 64 {
		return "owner username required (at most 64 characters)"
	}
	if len(owner.Password) < 8 {
		return "owner password must be at least 8 characters"
	}
	return ""
}

func renderSetupPage() string {
	return strings.NewReplacer("{{SERVER_NAME}}", "DWC Server Advanced", "{{SERVER_INITIALS}}", "DS").Replace(setupHTML)
}

func portOf(address string) string {
	if _, port, err := net.SplitHostPort(address); err == nil {
		return port
	}
	return ""
}

// localAddresses lists this machine's IPv4 addresses for the wizard hints.
func localAddresses() []string {
	out := []string{}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return out
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			out = append(out, ipnet.IP.String())
		}
	}
	return out
}
