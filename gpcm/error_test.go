package gpcm

import (
	"dwc-server-advanced/common"
	"encoding/binary"
	"strings"
	"testing"
)

func decodeWWFCErrorMessage(t *testing.T, msg string) string {
	t.Helper()

	commands, err := common.ParseGameSpyMessage(msg)
	if err != nil {
		t.Fatalf("failed to parse GameSpy message: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}

	encoded := commands[0].OtherValues["wl:errmsg"]
	if encoded == "" {
		t.Fatal("expected wl:errmsg to be present")
	}

	decoded, err := common.Base64DwcEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to decode wl:errmsg: %v", err)
	}

	return common.UTF16Decode(decoded, binary.BigEndian)
}

func TestGetMessageTranslateFallsBackToEnglish(t *testing.T) {
	gpErr := GPError{
		ErrorCode:   ErrConnectionClosed.ErrorCode,
		ErrorString: "restricted",
		Fatal:       true,
		WWFCMessage: WWFCMsgProfileRestrictedCustom,
		Reason:      "Test reason",
	}

	msg, wwfcErrorCode := gpErr.GetMessageTranslate("mariokartwii", 0, LangItalian, 0, 0x12345678)
	if wwfcErrorCode != WWFCMsgProfileRestrictedCustom.ErrorCode {
		t.Fatalf("unexpected WWFC error code: %d", wwfcErrorCode)
	}

	decoded := decodeWWFCErrorMessage(t, msg)
	if !strings.Contains(decoded, "Reason: Test reason") {
		t.Fatalf("expected English fallback message with reason, got %q", decoded)
	}
}

func TestGetMessageTranslateCapsCustomReason(t *testing.T) {
	gpErr := GPError{
		ErrorCode:   ErrConnectionClosed.ErrorCode,
		ErrorString: "restricted",
		Fatal:       true,
		WWFCMessage: WWFCMsgProfileRestrictedCustom,
		Reason:      strings.Repeat("A", 300),
	}

	msg, _ := gpErr.GetMessageTranslate("mariokartwii", 0, LangEnglish, 0, 0x12345678)
	commands, err := common.ParseGameSpyMessage(msg)
	if err != nil {
		t.Fatalf("failed to parse GameSpy message: %v", err)
	}

	encoded := commands[0].OtherValues["wl:errmsg"]
	if len(encoded) > 511 {
		t.Fatalf("encoded wl:errmsg is too large for payload buffer: %d", len(encoded))
	}

	decoded := decodeWWFCErrorMessage(t, msg)
	if !strings.Contains(decoded, "Reason: ") || !strings.Contains(decoded, "...") {
		t.Fatalf("expected truncated custom reason, got %q", decoded)
	}
	if strings.Contains(decoded, "Support Info") {
		t.Fatalf("support info should be omitted, got %q", decoded)
	}
}
