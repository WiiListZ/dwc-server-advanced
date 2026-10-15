package common

import (
	"dwc-server-advanced/logging"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/linkdata/deadlock"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Username        string `xml:"username" yaml:"username"`
	Password        string `xml:"password" yaml:"password"`
	DatabaseAddress string `xml:"databaseAddress" yaml:"databaseAddress"`
	DatabaseName    string `xml:"databaseName" yaml:"databaseName"`

	DefaultAddress       string  `xml:"address" yaml:"address"`
	GameSpyAddress       *string `xml:"gsAddress,omitempty" yaml:"gsAddress,omitempty"`
	NASAddress           *string `xml:"nasAddress,omitempty" yaml:"nasAddress,omitempty"`
	NASPort              string  `xml:"nasPort" yaml:"nasPort"`
	NASAddressHTTPS      *string `xml:"nasAddressHttps,omitempty" yaml:"nasAddressHttps,omitempty"`
	NASPortHTTPS         string  `xml:"nasPortHttps" yaml:"nasPortHttps"`
	PayloadServerAddress string  `xml:"payloadServerAddress" yaml:"payloadServerAddress"`

	FrontendAddress        string `xml:"frontendAddress" yaml:"frontendAddress"`
	FrontendBackendAddress string `xml:"frontendBackendAddress" yaml:"frontendBackendAddress"`
	BackendAddress         string `xml:"backendAddress" yaml:"backendAddress"`
	BackendFrontendAddress string `xml:"backendFrontendAddress" yaml:"backendFrontendAddress"`

	EnableHTTPS           bool  `xml:"enableHttps" yaml:"enableHttps"`
	EnableHTTPSExploitWii *bool `xml:"enableHttpsExploitWii,omitempty" yaml:"enableHttpsExploitWii,omitempty"`
	EnableHTTPSExploitDS  *bool `xml:"enableHttpsExploitDS,omitempty" yaml:"enableHttpsExploitDS,omitempty"`

	LogLevel  *int   `xml:"logLevel" yaml:"logLevel"`
	LogOutput string `xml:"logOutput" yaml:"logOutput"`

	CertPath      string `xml:"certPath" yaml:"certPath"`
	KeyPath       string `xml:"keyPath" yaml:"keyPath"`
	CertPathWii   string `xml:"certDerPathWii" yaml:"certDerPathWii"`
	KeyPathWii    string `xml:"keyPathWii" yaml:"keyPathWii"`
	CertPathDS    string `xml:"certDerPathDS" yaml:"certDerPathDS"`
	WiiCertPathDS string `xml:"wiiCertDerPathDS" yaml:"wiiCertDerPathDS"`
	KeyPathDS     string `xml:"keyPathDS" yaml:"keyPathDS"`

	APISecret string `xml:"apiSecret" yaml:"apiSecret"`

	AllowDefaultDolphinKeys     bool   `xml:"allowDefaultDolphinKeys" yaml:"allowDefaultDolphinKeys"`
	AllowMultipleDeviceIDs      string `xml:"allowMultipleDeviceIDs" yaml:"allowMultipleDeviceIDs"`
	AllowConnectWithoutDeviceID bool   `xml:"allowConnectWithoutDeviceID" yaml:"allowConnectWithoutDeviceID"`
	AllowNoSSLCheatClients      bool   `xml:"allowNoSSLCheatClients" yaml:"allowNoSSLCheatClients"`

	ServerName string `xml:"serverName,omitempty" yaml:"serverName,omitempty"`

	// KeepRevokedBans keeps lifted bans in the history marked as revoked.
	// When false, unbanning deletes the lifted ban from the database.
	KeepRevokedBans bool `xml:"keepRevokedBans" yaml:"keepRevokedBans"`
	// BanHistoryDays is how long ended bans, revoked bans and kicks stay
	// visible in the ban lists.
	BanHistoryDays int `xml:"banHistoryDays" yaml:"banHistoryDays"`

	EventReporting EventReportingConfig `xml:"eventReporting" yaml:"eventReporting"`

	Updates UpdatesConfig `xml:"updates" yaml:"updates"`

	Profiles ProfilesConfig `xml:"profiles" yaml:"profiles"`
}

// ProfilesConfig controls the public site profiles (avatar, bio, stats).
type ProfilesConfig struct {
	Enabled bool `xml:"enabled" yaml:"enabled"`
}

// UpdatesConfig controls the release checker and the self-updater.
type UpdatesConfig struct {
	// Enabled turns the release checker on.
	Enabled bool `xml:"enabled" yaml:"enabled"`
	// Repository is the GitHub "owner/name" whose releases are installed.
	Repository string `xml:"repository" yaml:"repository"`
	// CheckIntervalMinutes is how often the latest release is fetched.
	CheckIntervalMinutes int `xml:"checkIntervalMinutes" yaml:"checkIntervalMinutes"`
	// APIBaseURL is the GitHub API root; change it only for mirrors or tests.
	APIBaseURL string `xml:"apiBaseUrl" yaml:"apiBaseUrl"`
}

type EventReportingConfig struct {
	LogToDatabase bool                    `xml:"logToDatabase" yaml:"logToDatabase"`
	Webhooks      []logging.WebhookConfig `xml:"discord>webhook" yaml:"webhooks"`
}

var (
	config       Config
	configLoaded bool
	cmutex       = deadlock.Mutex{}
)

func GetConfig() Config {
	cmutex.Lock()
	defer cmutex.Unlock()

	if configLoaded {
		return config
	}

	data, path, err := readConfigFile()
	if err != nil {
		panic(err)
	}

	parsed, err := parseConfig(data, path)
	if err != nil {
		panic(err)
	}

	config = parsed
	configLoaded = true

	return config
}

func readConfigFile() ([]byte, string, error) {
	if path := strings.TrimSpace(os.Getenv("WWFC_CONFIG")); path != "" {
		data, err := os.ReadFile(path)
		return data, path, err
	}

	for _, path := range []string{"config.yml", "config.yaml", "config.xml"} {
		data, err := os.ReadFile(path)
		if err == nil {
			if path == "config.xml" {
				logging.Warn("CONFIG", "Loading legacy config.xml; config.yml is the default format, copy config_example.yml to config.yml")
			}
			return data, path, nil
		}
		if !os.IsNotExist(err) {
			return nil, path, err
		}
	}

	return nil, "", fmt.Errorf("no config file found; copy config_example.yml to config.yml")
}

func parseConfig(data []byte, path string) (Config, error) {
	config := Config{
		AllowDefaultDolphinKeys:     true,
		AllowMultipleDeviceIDs:      "never",
		AllowConnectWithoutDeviceID: false,
		ServerName:                  "WFC Server",
		KeepRevokedBans:             true,
		BanHistoryDays:              30,
		Profiles:                    ProfilesConfig{Enabled: true},
		Updates: UpdatesConfig{
			Enabled:              true,
			Repository:           "WiiListZ/dwc-server-advanced",
			CheckIntervalMinutes: 60,
			APIBaseURL:           "https://api.github.com",
		},
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".xml":
		if err := xml.Unmarshal(data, &config); err != nil {
			return Config{}, err
		}
	case ".yml", ".yaml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return Config{}, err
		}
	default:
		return Config{}, fmt.Errorf("unsupported config file extension %q", filepath.Ext(path))
	}

	if config.GameSpyAddress == nil {
		config.GameSpyAddress = &config.DefaultAddress
	}

	if config.NASAddress == nil {
		config.NASAddress = &config.DefaultAddress
	}

	if config.NASAddressHTTPS == nil {
		if config.NASAddress != nil {
			config.NASAddressHTTPS = config.NASAddress
		} else {
			config.NASAddressHTTPS = &config.DefaultAddress
		}
	}

	if config.EnableHTTPSExploitWii == nil {
		enable := true
		config.EnableHTTPSExploitWii = &enable
	}

	if config.EnableHTTPSExploitDS == nil {
		enable := true
		config.EnableHTTPSExploitDS = &enable
	}

	if config.LogLevel == nil {
		level := 4
		config.LogLevel = &level
	}

	if config.Updates.Repository == "" {
		config.Updates.Repository = "WiiListZ/dwc-server-advanced"
	}
	if config.Updates.CheckIntervalMinutes <= 0 {
		config.Updates.CheckIntervalMinutes = 60
	}
	if config.Updates.APIBaseURL == "" {
		config.Updates.APIBaseURL = "https://api.github.com"
	}

	if config.BanHistoryDays <= 0 {
		config.BanHistoryDays = 30
	}

	if config.LogOutput == "" {
		config.LogOutput = "StdOutAndFile"
	}

	if config.FrontendAddress == "" {
		config.FrontendAddress = "127.0.0.1:29998"
	}

	if config.BackendAddress == "" {
		config.BackendAddress = "127.0.0.1:29999"
	}

	if config.FrontendBackendAddress == "" {
		config.FrontendBackendAddress = config.BackendAddress
	}

	if config.BackendFrontendAddress == "" {
		config.BackendFrontendAddress = config.FrontendAddress
	}

	if config.AllowMultipleDeviceIDs == "true" || config.AllowMultipleDeviceIDs == "yes" {
		config.AllowMultipleDeviceIDs = "always"
	} else if config.AllowMultipleDeviceIDs != "SameIPAddress" {
		config.AllowMultipleDeviceIDs = "never"
	}

	return config, nil
}

func (c *EventReportingConfig) UnmarshalYAML(value *yaml.Node) error {
	var decoded struct {
		LogToDatabase bool                    `yaml:"logToDatabase"`
		Webhooks      []logging.WebhookConfig `yaml:"webhooks"`
		Discord       struct {
			Webhooks []logging.WebhookConfig `yaml:"webhooks"`
			Webhook  []logging.WebhookConfig `yaml:"webhook"`
		} `yaml:"discord"`
	}

	if err := value.Decode(&decoded); err != nil {
		return err
	}

	c.LogToDatabase = decoded.LogToDatabase
	c.Webhooks = append(decoded.Webhooks, decoded.Discord.Webhooks...)
	c.Webhooks = append(c.Webhooks, decoded.Discord.Webhook...)

	return nil
}

func (c Config) RegisterWebhooks() {
	for _, webhook := range c.EventReporting.Webhooks {
		webhook.RegisterWebhook()
	}
}

// ResetConfigForTest forgets the loaded config so the next GetConfig reads
// the file again. Only tests use it.
func ResetConfigForTest() {
	cmutex.Lock()
	defer cmutex.Unlock()
	configLoaded = false
}
