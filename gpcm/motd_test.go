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
