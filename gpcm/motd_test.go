package gpcm

import (
	"dwc-server-advanced/common"
	"encoding/binary"
	"strings"
	"testing"
)

func TestEncodeMessageOfTheDayFitsPayloadBuffer(t *testing.T) {
	encoded := EncodeMessageOfTheDay(strings.Repeat("A", 500))
	if len(encoded) > 511 {
		t.Fatalf("encoded MOTD is too large for payload buffer: %d", len(encoded))
	}

	decoded, err := common.Base64DwcEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to decode MOTD: %v", err)
	}

	text := common.UTF16Decode(decoded, binary.BigEndian)
	if !strings.HasSuffix(text, "...") {
		t.Fatalf("expected truncated MOTD to end with ellipsis, got %q", text)
	}
}

func TestWelcomeMessageSettings(t *testing.T) {
	config := common.DefaultConfig()
	config.ServerName = "My server"
	if text, show := welcomeMessageForConfig(config); !show || text != "Welcome to\nMy server!" {
		t.Fatalf("unexpected default welcome: %q, %v", text, show)
	}
	config.WelcomeMessage = "Custom\nmessage"
	if text, show := welcomeMessageForConfig(config); !show || text != "Custom\nmessage" {
		t.Fatalf("unexpected custom welcome: %q, %v", text, show)
	}
	config.ShowWelcomeMessage = false
	if text, show := welcomeMessageForConfig(config); show || text != "" {
		t.Fatalf("disabled welcome is visible: %q, %v", text, show)
	}
}
