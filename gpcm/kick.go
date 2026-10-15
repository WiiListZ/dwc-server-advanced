package gpcm

import (
	"dwc-server-advanced/common"
	"strings"
)

// KickedPlayer describes a session closed by KickDisabledGamePlayers.
type KickedPlayer struct {
	ProfileID uint32 `json:"pid"`
	Name      string `json:"name,omitempty"`
	GameName  string `json:"game"`
}

// matchesDisabledGame reports whether a session playing gameName with the
// given NAS game code is covered by a disabled game entry.
func matchesDisabledGame(gameName string, gameCode string, game common.DisabledGame) bool {
	matchesName := game.GameName != "" && gameName == game.GameName
	matchesCode := game.GameCodePrefix != "" && strings.HasPrefix(gameCode, game.GameCodePrefix)
	return matchesName || matchesCode
}

// KickDisabledGamePlayers disconnects every logged in session that matches the
// disabled game, by GameSpy game name or by game code prefix, showing the same
// message players get when they try to log in to a disabled game.
func KickDisabledGamePlayers(game common.DisabledGame) []KickedPlayer {
	mutex.Lock()
	defer mutex.Unlock()

	message := disabledGameMessage(game)
	kicked := []KickedPlayer{}
	for profileID, session := range sessions {
		if !session.LoggedIn {
			continue
		}

		if !matchesDisabledGame(session.GameName, session.GameCode, game) {
			continue
		}

		name := session.InGameName
		if name == "" {
			name = session.User.LastInGameSn
		}
		kicked = append(kicked, KickedPlayer{ProfileID: profileID, Name: name, GameName: session.GameName})

		session.replyError(GPError{
			ErrorCode:   ErrConnectionClosed.ErrorCode,
			ErrorString: "The game was disabled on this server.",
			Fatal:       true,
			WWFCMessage: message,
			Reason:      "game_disabled",
		})
	}

	return kicked
}

func kickPlayer(profileID uint32, reason string) {
	if session, exists := sessions[profileID]; exists {
		errorMessage := WWFCMsgKickedGeneric

		switch reason {
		case "banned":
			errorMessage = WWFCMsgProfileBannedTOSNow

		case "restricted":
			errorMessage = WWFCMsgProfileRestrictedNow

		case "restricted_join":
			errorMessage = WWFCMsgProfileRestricted

		case "moderator_kick":
			errorMessage = WWFCMsgKickedModerator

		case "room_kick":
			errorMessage = WWFCMsgKickedRoomHost

		case "invalid_elo":
			errorMessage = WWFCMsgInvalidELO

		case "network_error":
			// No error message
			common.ShouldNotError(common.CloseConnection(ServerName, session.ConnIndex))
			return
		}

		session.replyError(GPError{
			ErrorCode:   ErrConnectionClosed.ErrorCode,
			ErrorString: "The player was kicked from the server. Reason: " + reason,
			Fatal:       true,
			WWFCMessage: errorMessage,
		})
	}
}

func KickPlayer(profileID uint32, reason string) {
	mutex.Lock()
	defer mutex.Unlock()

	kickPlayer(profileID, reason)
}

func KickPlayerCustomMessage(profileID uint32, reason string, message WWFCErrorMessage) {
	mutex.Lock()
	defer mutex.Unlock()

	if session, exists := sessions[profileID]; exists {
		session.replyError(GPError{
			ErrorCode:   ErrConnectionClosed.ErrorCode,
			ErrorString: "The player was kicked from the server. Reason: " + reason,
			Fatal:       true,
			WWFCMessage: message,
			Reason:      reason,
		})
	}
}
