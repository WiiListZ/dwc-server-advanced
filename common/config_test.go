package common

import "testing"

func TestParseXMLConfigAppliesDefaults(t *testing.T) {
	cfg, err := parseConfig([]byte(`
<Config>
	<address>127.0.0.1</address>
	<enableHttpsExploitWii>false</enableHttpsExploitWii>
	<logLevel>2</logLevel>
</Config>
`), "config.xml")
	if err != nil {
		t.Fatalf("parse XML config: %v", err)
	}

	if cfg.GameSpyAddress == nil || *cfg.GameSpyAddress != "127.0.0.1" {
		t.Fatalf("GameSpyAddress default = %v, want 127.0.0.1", cfg.GameSpyAddress)
	}
	if cfg.NASAddress == nil || *cfg.NASAddress != "127.0.0.1" {
		t.Fatalf("NASAddress default = %v, want 127.0.0.1", cfg.NASAddress)
	}
	if cfg.NASAddressHTTPS == nil || *cfg.NASAddressHTTPS != "127.0.0.1" {
		t.Fatalf("NASAddressHTTPS default = %v, want 127.0.0.1", cfg.NASAddressHTTPS)
	}
	if cfg.EnableHTTPSExploitWii == nil || *cfg.EnableHTTPSExploitWii {
		t.Fatalf("EnableHTTPSExploitWii = %v, want false", cfg.EnableHTTPSExploitWii)
	}
	if cfg.EnableHTTPSExploitDS == nil || !*cfg.EnableHTTPSExploitDS {
		t.Fatalf("EnableHTTPSExploitDS default = %v, want true", cfg.EnableHTTPSExploitDS)
	}
	if cfg.LogOutput != "StdOutAndFile" {
		t.Fatalf("LogOutput default = %q, want StdOutAndFile", cfg.LogOutput)
	}
}

func TestParseYAMLConfigLoadsDiscordWebhooks(t *testing.T) {
	cfg, err := parseConfig([]byte(`
gsAddress: 127.0.0.1
eventReporting:
  logToDatabase: true
  discord:
    webhooks:
      - enabled: true
        url: https://example.com/webhook
        author:
          name: event reporter
          url: https://example.com
        eventTypes:
          - backend_started
          - logged_in
      - enabled: false
        url: https://example.com/second
        author: compact author
        eventTypes:
          - all
`), "config.yml")
	if err != nil {
		t.Fatalf("parse YAML config: %v", err)
	}

	if !cfg.EventReporting.LogToDatabase {
		t.Fatal("EventReporting.LogToDatabase = false, want true")
	}
	if len(cfg.EventReporting.Webhooks) != 2 {
		t.Fatalf("webhook count = %d, want 2", len(cfg.EventReporting.Webhooks))
	}

	first := cfg.EventReporting.Webhooks[0]
	if !first.Enabled || first.URL != "https://example.com/webhook" {
		t.Fatalf("first webhook = %+v", first)
	}
	if first.Author.Name != "event reporter" || first.Author.URL != "https://example.com" {
		t.Fatalf("first author = %+v", first.Author)
	}
	if len(first.EventTypes) != 2 || first.EventTypes[0] != "backend_started" || first.EventTypes[1] != "logged_in" {
		t.Fatalf("first event types = %#v", first.EventTypes)
	}

	second := cfg.EventReporting.Webhooks[1]
	if second.Author.Name != "compact author" || second.Author.URL != "" {
		t.Fatalf("second author = %+v", second.Author)
	}
}
