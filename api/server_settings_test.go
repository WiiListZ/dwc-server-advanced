package api

import (
	"dwc-server-advanced/common"
	"testing"
)

func TestSettingsRoundTripNeedsNoRestart(t *testing.T) {
	base := common.DefaultConfig()
	base.DatabaseAddress, base.DatabaseName, base.Username, base.Password = "127.0.0.1", "wwfc", "dwc", "secret"
	base.APISecret = "existing"
	s := SetupDefaults()
	s.Username = "dwc"
	s.Password = "secret"
	cfg, err := applySettings(base, s)
	if err != nil {
		t.Fatal(err)
	}

	again, err := applySettings(cfg, settingsFromConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if restartRequired(cfg, again) {
		t.Fatal("saving unchanged settings asks for a restart")
	}
	if again.Password != "secret" {
		t.Fatal("empty password field must keep the stored password")
	}
	if again.APISecret != "existing" {
		t.Fatal("API secret changed without being asked")
	}

	renamed := settingsFromConfig(cfg)
	renamed.ServerName = "New name"
	next, err := applySettings(cfg, renamed)
	if err != nil {
		t.Fatal(err)
	}
	if restartRequired(cfg, next) {
		t.Fatal("renaming the server must apply without a restart")
	}

	welcome := settingsFromConfig(cfg)
	welcome.ShowWelcomeMessage = false
	welcome.WelcomeMessage = "Custom welcome"
	welcome.RequirePayloadFeatures = true
	next, err = applySettings(cfg, welcome)
	if err != nil {
		t.Fatal(err)
	}
	if next.ShowWelcomeMessage || next.WelcomeMessage != "Custom welcome" || !next.RequirePayloadFeatures {
		t.Fatal("welcome or payload policy did not round-trip")
	}
	if restartRequired(cfg, next) {
		t.Fatal("live client settings must apply without a restart")
	}

	moved := settingsFromConfig(cfg)
	moved.NASPort = "8080"
	next, err = applySettings(cfg, moved)
	if err != nil {
		t.Fatal(err)
	}
	if !restartRequired(cfg, next) {
		t.Fatal("changing the NAS port must ask for a restart")
	}

	bad := settingsFromConfig(cfg)
	bad.Webhooks = []webhookSettings{{Enabled: true, URL: "http://insecure.example"}}
	if _, err := applySettings(cfg, bad); err == nil {
		t.Fatal("non-https webhook accepted")
	}
}
