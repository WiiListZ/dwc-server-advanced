package api

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/logging"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// serverSettings is the editable view of config.yml used by the setup wizard
// and by the panel's server settings. Secrets are never sent back: the
// database password and API secret are write-only.
type serverSettings struct {
	ServerName string `json:"serverName"`

	GSAddress            string `json:"gsAddress"`
	NASAddress           string `json:"nasAddress"`
	NASPort              string `json:"nasPort"`
	NASAddressHTTPS      string `json:"nasAddressHttps"`
	NASPortHTTPS         string `json:"nasPortHttps"`
	PayloadServerAddress string `json:"payloadServerAddress"`

	EnableHTTPS           bool   `json:"enableHttps"`
	CertPath              string `json:"certPath"`
	KeyPath               string `json:"keyPath"`
	EnableHTTPSExploitWii bool   `json:"enableHttpsExploitWii"`
	CertDerPathWii        string `json:"certDerPathWii"`
	KeyPathWii            string `json:"keyPathWii"`
	EnableHTTPSExploitDS  bool   `json:"enableHttpsExploitDS"`
	CertDerPathDS         string `json:"certDerPathDS"`
	WiiCertDerPathDS      string `json:"wiiCertDerPathDS"`
	KeyPathDS             string `json:"keyPathDS"`

	FrontendAddress        string `json:"frontendAddress"`
	FrontendBackendAddress string `json:"frontendBackendAddress"`
	BackendAddress         string `json:"backendAddress"`
	BackendFrontendAddress string `json:"backendFrontendAddress"`

	DatabaseAddress string `json:"databaseAddress"`
	DatabaseName    string `json:"databaseName"`
	Username        string `json:"username"`
	Password        string `json:"password,omitempty"`
	HasPassword     bool   `json:"hasPassword"`

	AllowDefaultDolphinKeys     bool   `json:"allowDefaultDolphinKeys"`
	AllowNoSSLCheatClients      bool   `json:"allowNoSSLCheatClients"`
	AllowConnectWithoutDeviceID bool   `json:"allowConnectWithoutDeviceID"`
	AllowMultipleDeviceIDs      string `json:"allowMultipleDeviceIDs"`

	LogLevel  int    `json:"logLevel"`
	LogOutput string `json:"logOutput"`

	KeepRevokedBans bool `json:"keepRevokedBans"`
	BanHistoryDays  int  `json:"banHistoryDays"`

	ProfilesEnabled bool `json:"profilesEnabled"`

	UpdatesEnabled              bool   `json:"updatesEnabled"`
	UpdatesRepository           string `json:"updatesRepository"`
	UpdatesCheckIntervalMinutes int    `json:"updatesCheckIntervalMinutes"`

	LogEventsToDatabase bool              `json:"logEventsToDatabase"`
	Webhooks            []webhookSettings `json:"webhooks"`

	HasAPISecret        bool `json:"hasApiSecret"`
	RegenerateAPISecret bool `json:"regenerateApiSecret,omitempty"`
}

type webhookSettings struct {
	Enabled    bool     `json:"enabled"`
	URL        string   `json:"url"`
	Author     string   `json:"author"`
	EventTypes []string `json:"eventTypes"`
}

func settingsFromConfig(c common.Config) serverSettings {
	deref := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	derefBool := func(b *bool) bool { return b != nil && *b }

	s := serverSettings{
		ServerName:                  c.ServerName,
		GSAddress:                   deref(c.GameSpyAddress),
		NASAddress:                  deref(c.NASAddress),
		NASPort:                     c.NASPort,
		NASAddressHTTPS:             deref(c.NASAddressHTTPS),
		NASPortHTTPS:                c.NASPortHTTPS,
		PayloadServerAddress:        c.PayloadServerAddress,
		EnableHTTPS:                 c.EnableHTTPS,
		CertPath:                    c.CertPath,
		KeyPath:                     c.KeyPath,
		EnableHTTPSExploitWii:       derefBool(c.EnableHTTPSExploitWii),
		CertDerPathWii:              c.CertPathWii,
		KeyPathWii:                  c.KeyPathWii,
		EnableHTTPSExploitDS:        derefBool(c.EnableHTTPSExploitDS),
		CertDerPathDS:               c.CertPathDS,
		WiiCertDerPathDS:            c.WiiCertPathDS,
		KeyPathDS:                   c.KeyPathDS,
		FrontendAddress:             c.FrontendAddress,
		FrontendBackendAddress:      c.FrontendBackendAddress,
		BackendAddress:              c.BackendAddress,
		BackendFrontendAddress:      c.BackendFrontendAddress,
		DatabaseAddress:             c.DatabaseAddress,
		DatabaseName:                c.DatabaseName,
		Username:                    c.Username,
		HasPassword:                 c.Password != "",
		AllowDefaultDolphinKeys:     c.AllowDefaultDolphinKeys,
		AllowNoSSLCheatClients:      c.AllowNoSSLCheatClients,
		AllowConnectWithoutDeviceID: c.AllowConnectWithoutDeviceID,
		AllowMultipleDeviceIDs:      c.AllowMultipleDeviceIDs,
		LogOutput:                   c.LogOutput,
		KeepRevokedBans:             c.KeepRevokedBans,
		BanHistoryDays:              c.BanHistoryDays,
		ProfilesEnabled:             c.Profiles.Enabled,
		UpdatesEnabled:              c.Updates.Enabled,
		UpdatesRepository:           c.Updates.Repository,
		UpdatesCheckIntervalMinutes: c.Updates.CheckIntervalMinutes,
		LogEventsToDatabase:         c.EventReporting.LogToDatabase,
		Webhooks:                    []webhookSettings{},
		HasAPISecret:                c.APISecret != "",
	}
	if c.LogLevel != nil {
		s.LogLevel = *c.LogLevel
	}
	for _, w := range c.EventReporting.Webhooks {
		s.Webhooks = append(s.Webhooks, webhookSettings{Enabled: w.Enabled, URL: w.URL, Author: w.Author.Name, EventTypes: append([]string{}, w.EventTypes...)})
	}
	return s
}

var (
	repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	deviceIDModes     = []string{"never", "SameIPAddress", "always"}
	logOutputs        = []string{"None", "StdOut", "StdOutAndFile"}
)

// applySettings returns base updated with s, or a user-facing error.
func applySettings(base common.Config, s serverSettings) (common.Config, error) {
	c := base
	trim := strings.TrimSpace
	str := func(v string) *string { v = trim(v); return &v }
	boolean := func(v bool) *bool { return &v }

	c.ServerName = trim(s.ServerName)
	if c.ServerName == "" {
		return c, fmt.Errorf("server name required")
	}

	c.GameSpyAddress = str(s.GSAddress)
	c.NASAddress = str(s.NASAddress)
	c.NASAddressHTTPS = str(s.NASAddressHTTPS)
	c.NASPort = trim(s.NASPort)
	c.NASPortHTTPS = trim(s.NASPortHTTPS)
	c.PayloadServerAddress = trim(s.PayloadServerAddress)
	for name, port := range map[string]string{"NAS port": c.NASPort, "NAS HTTPS port": c.NASPortHTTPS} {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return c, fmt.Errorf("%s must be a number between 1 and 65535", name)
		}
	}
	for name, addr := range map[string]string{"GameSpy address": *c.GameSpyAddress, "NAS address": *c.NASAddress, "NAS HTTPS address": *c.NASAddressHTTPS} {
		if addr != "" && net.ParseIP(addr) == nil {
			return c, fmt.Errorf("%s must be an IP address such as 0.0.0.0 or 127.0.0.1", name)
		}
	}

	c.EnableHTTPS = s.EnableHTTPS
	c.CertPath, c.KeyPath = trim(s.CertPath), trim(s.KeyPath)
	c.EnableHTTPSExploitWii = boolean(s.EnableHTTPSExploitWii)
	c.CertPathWii, c.KeyPathWii = trim(s.CertDerPathWii), trim(s.KeyPathWii)
	c.EnableHTTPSExploitDS = boolean(s.EnableHTTPSExploitDS)
	c.CertPathDS, c.WiiCertPathDS, c.KeyPathDS = trim(s.CertDerPathDS), trim(s.WiiCertDerPathDS), trim(s.KeyPathDS)
	if c.EnableHTTPS && (c.CertPath == "" || c.KeyPath == "") {
		return c, fmt.Errorf("HTTPS needs a certificate and a key path")
	}

	c.FrontendAddress = trim(s.FrontendAddress)
	c.FrontendBackendAddress = trim(s.FrontendBackendAddress)
	c.BackendAddress = trim(s.BackendAddress)
	c.BackendFrontendAddress = trim(s.BackendFrontendAddress)
	for name, addr := range map[string]string{"Frontend RPC address": c.FrontendAddress, "Frontend to backend address": c.FrontendBackendAddress, "Backend RPC address": c.BackendAddress, "Backend to frontend address": c.BackendFrontendAddress} {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			return c, fmt.Errorf("%s must look like 127.0.0.1:29998", name)
		}
	}

	c.DatabaseAddress, c.DatabaseName, c.Username = trim(s.DatabaseAddress), trim(s.DatabaseName), trim(s.Username)
	if s.Password != "" {
		c.Password = s.Password
	}
	if c.DatabaseAddress == "" || c.DatabaseName == "" || c.Username == "" {
		return c, fmt.Errorf("database address, name and user are required")
	}

	c.AllowDefaultDolphinKeys = s.AllowDefaultDolphinKeys
	c.AllowNoSSLCheatClients = s.AllowNoSSLCheatClients
	c.AllowConnectWithoutDeviceID = s.AllowConnectWithoutDeviceID
	if !contains(deviceIDModes, s.AllowMultipleDeviceIDs) {
		return c, fmt.Errorf("multiple device IDs must be one of %s", strings.Join(deviceIDModes, ", "))
	}
	c.AllowMultipleDeviceIDs = s.AllowMultipleDeviceIDs

	if s.LogLevel < 0 || s.LogLevel > 4 {
		return c, fmt.Errorf("log level must be between 0 and 4")
	}
	level := s.LogLevel
	c.LogLevel = &level
	if !contains(logOutputs, s.LogOutput) {
		return c, fmt.Errorf("log output must be one of %s", strings.Join(logOutputs, ", "))
	}
	c.LogOutput = s.LogOutput

	c.KeepRevokedBans = s.KeepRevokedBans
	if s.BanHistoryDays < 1 || s.BanHistoryDays > 3650 {
		return c, fmt.Errorf("ban history must be between 1 and 3650 days")
	}
	c.BanHistoryDays = s.BanHistoryDays

	c.Profiles.Enabled = s.ProfilesEnabled

	c.Updates.Enabled = s.UpdatesEnabled
	c.Updates.Repository = trim(s.UpdatesRepository)
	if !repositoryPattern.MatchString(c.Updates.Repository) {
		return c, fmt.Errorf("update repository must look like owner/name")
	}
	if s.UpdatesCheckIntervalMinutes < 5 || s.UpdatesCheckIntervalMinutes > 10080 {
		return c, fmt.Errorf("update check interval must be between 5 minutes and 7 days")
	}
	c.Updates.CheckIntervalMinutes = s.UpdatesCheckIntervalMinutes

	c.EventReporting.LogToDatabase = s.LogEventsToDatabase
	c.EventReporting.Webhooks = []logging.WebhookConfig{}
	for i, w := range s.Webhooks {
		u, err := url.Parse(trim(w.URL))
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return c, fmt.Errorf("webhook %d needs an https:// URL", i+1)
		}
		types := []string{}
		for _, t := range w.EventTypes {
			if t = trim(t); t != "" {
				types = append(types, t)
			}
		}
		if len(types) == 0 {
			types = []string{"all"}
		}
		hook := logging.WebhookConfig{Enabled: w.Enabled, URL: u.String(), EventTypes: types}
		hook.Author.Name = trim(w.Author)
		c.EventReporting.Webhooks = append(c.EventReporting.Webhooks, hook)
	}

	if s.RegenerateAPISecret || c.APISecret == "" {
		c.APISecret = common.RandomString(32)
	}
	return c, nil
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// liveSettings take effect without a restart because they are read from the
// config on every use.
var liveSettings = map[string]bool{
	"ServerName": true, "KeepRevokedBans": true, "BanHistoryDays": true, "Profiles": true,
	"AllowDefaultDolphinKeys": true, "AllowMultipleDeviceIDs": true, "AllowConnectWithoutDeviceID": true,
}

func restartRequired(before, after common.Config) bool {
	// An empty and a missing webhook list mean the same thing.
	if len(before.EventReporting.Webhooks) == 0 && len(after.EventReporting.Webhooks) == 0 {
		before.EventReporting.Webhooks, after.EventReporting.Webhooks = nil, nil
	}
	b, a := reflect.ValueOf(before), reflect.ValueOf(after)
	for i := 0; i < b.NumField(); i++ {
		name := b.Type().Field(i).Name
		if liveSettings[name] {
			continue
		}
		if name == "Updates" {
			// The repository is read on every check; enabling or the
			// interval need the checker to start again.
			if before.Updates.Enabled != after.Updates.Enabled || before.Updates.CheckIntervalMinutes != after.Updates.CheckIntervalMinutes {
				return true
			}
			continue
		}
		if !reflect.DeepEqual(b.Field(i).Interface(), a.Field(i).Interface()) {
			return true
		}
	}
	return false
}

// HandlePanelServerSettings lets the owner read and change config.yml.
func HandlePanelServerSettings(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}
	if account.Role != "owner" {
		writePanelError(w, http.StatusForbidden, "owner account required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writePanelJSON(w, http.StatusOK, map[string]any{"settings": settingsFromConfig(common.GetConfig()), "path": common.ConfigFilePath()})

	case http.MethodPut:
		req := serverSettings{}
		if !readPanelJSON(w, r, &req) {
			return
		}
		before := common.GetConfig()
		next, err := applySettings(before, req)
		if err != nil {
			writePanelError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := common.SaveConfig(next); err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to write the config file: "+err.Error())
			return
		}
		restart := restartRequired(before, next)
		logging.Event("server_settings_changed", map[string]any{"moderator": account.Username, "restart_required": restart})
		writePanelJSON(w, http.StatusOK, map[string]any{"settings": settingsFromConfig(common.GetConfig()), "restartRequired": restart})

	default:
		w.Header().Set("Allow", "GET, PUT")
		writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// HandlePanelServerRestart restarts the whole server (owner only).
func HandlePanelServerRestart(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}
	if account.Role != "owner" {
		writePanelError(w, http.StatusForbidden, "owner account required")
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	logging.Notice("API", "Server restart requested by", account.Username)
	logging.Event("server_restart_requested", map[string]any{"moderator": account.Username})
	writePanelJSON(w, http.StatusAccepted, map[string]bool{"success": true})
	go func() {
		if err := common.RequestRestart(); err != nil {
			logging.Error("API", "Restart request failed:", err)
		}
	}()
}
