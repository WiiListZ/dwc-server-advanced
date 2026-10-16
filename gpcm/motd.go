package gpcm

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/logging"
	"unicode/utf16"
)

const maxMessageOfTheDayUTF16Units = 190

func welcomeMessageForConfig(config common.Config) (string, bool) {
	if !config.ShowWelcomeMessage {
		return "", false
	}
	if config.WelcomeMessage != "" {
		return config.WelcomeMessage, true
	}
	return "Welcome to\n" + config.ServerName + "!", true
}

func truncateMessageOfTheDay(motd string) string {
	if len(utf16.Encode([]rune(motd))) <= maxMessageOfTheDayUTF16Units {
		return motd
	}

	out := []rune{}
	units := 0
	for _, r := range motd {
		runeUnits := len(utf16.Encode([]rune{r}))
		if units+runeUnits > maxMessageOfTheDayUTF16Units-3 {
			break
		}
		out = append(out, r)
		units += runeUnits
	}

	return string(out) + "..."
}

func EncodeMessageOfTheDay(motd string) string {
	motd = truncateMessageOfTheDay(motd)
	motdUTF16 := utf16.Encode([]rune(motd))
	motdByteArray := common.UTF16ToByteArray(motdUTF16)
	return common.Base64DwcEncoding.EncodeToString(motdByteArray)
}

func GetProfileMessageOfTheDay(profileID uint32) (string, bool, error) {
	loginMessage, exists, err := db.TakePendingLoginMessage(profileID)
	if err != nil || !exists {
		return "", false, err
	}

	logging.Event("login_message_delivered", map[string]any{
		"profile_id": profileID,
		"message_id": loginMessage.ID,
		"moderator":  loginMessage.Moderator,
	})

	return loginMessage.Message, true, nil
}

func DeliverPendingLoginMessage(profileID uint32) (LoginMessageTargetInfo, bool, error) {
	// The payload reads wl:motd from the real login response and
	// opens the popup from the Wi-Fi menu connection flow. A second lc\2
	// packet after login can consume the pending DB row without displaying
	// anything in-game, so leave the row queued for the next login.
	return GetLoginMessageTargetInfo(profileID), false, nil
}
