package api

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/database"
	"dwc-server-advanced/gpcm"
	"dwc-server-advanced/logging"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	disabledGameDefaultReturnCode = "110"
	disabledGameMessageMaxLength  = 500
)

var (
	disabledGameCodePrefixPattern = regexp.MustCompile(`^[A-Z0-9]{1,4}$`)
	disabledGameReturnCodePattern = regexp.MustCompile(`^[0-9]{3}$`)
)

type disabledGameResponse struct {
	database.PanelDisabledGame
	Kicked []gpcm.KickedPlayer `json:"kicked"`
}

func loadDisabledGames() {
	if err := db.UpdateDisabledGamesTable(); err != nil {
		panic(err)
	}

	imported, err := db.ImportLegacyDisabledGames()
	if err != nil {
		logging.Error("API", "Failed to import disabled_games.json:", err)
	} else if imported > 0 {
		logging.Notice("API", "Imported", imported, "disabled games from disabled_games.json")
	}

	if err := db.ReloadDisabledGamesCache(); err != nil {
		panic(err)
	}
}

func HandlePanelDisabledGames(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightManageGames, "manage games")
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		games, err := db.ListDisabledGames()
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to list disabled games")
			return
		}
		writePanelJSON(w, http.StatusOK, games)

	case http.MethodPost, http.MethodPatch:
		game := common.DisabledGame{}
		if !readPanelJSON(w, r, &game) {
			return
		}
		if r.Method == http.MethodPatch && game.ID <= 0 {
			writePanelError(w, http.StatusBadRequest, "id required")
			return
		}
		if message := normalizeDisabledGame(&game); message != "" {
			writePanelError(w, http.StatusBadRequest, message)
			return
		}

		var saved database.PanelDisabledGame
		var err error
		eventType := "disabled_game_added"
		status := http.StatusCreated
		if r.Method == http.MethodPost {
			saved, err = db.CreateDisabledGame(game, account.Username)
		} else {
			var found bool
			saved, found, err = db.UpdateDisabledGame(game, account.Username)
			if err == nil && !found {
				writePanelError(w, http.StatusNotFound, "disabled game not found")
				return
			}
			eventType = "disabled_game_updated"
			status = http.StatusOK
		}
		if errors.Is(err, database.ErrDuplicateDisabledGame) {
			writePanelError(w, http.StatusConflict, "this game code prefix or game name is already disabled")
			return
		}
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to save disabled game")
			return
		}

		refreshDisabledGamesCache()

		// Players already online with the game are disconnected right away,
		// with the same message they get when logging in to a disabled game.
		kicked := gpcm.KickDisabledGamePlayers(saved.DisabledGame)
		if len(kicked) > 0 {
			logging.Notice("API", "Kicked", len(kicked), "players of disabled game", saved.GameName+saved.GameCodePrefix)
		}

		eventData := disabledGameEventData(saved.DisabledGame, account.Username)
		eventData["kicked_players"] = len(kicked)
		logging.Event(eventType, eventData)
		writePanelJSON(w, status, disabledGameResponse{PanelDisabledGame: saved, Kicked: kicked})

	case http.MethodDelete:
		id, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("id")))
		if id <= 0 {
			req := struct {
				ID int `json:"id"`
			}{}
			if !readPanelJSON(w, r, &req) {
				return
			}
			id = req.ID
		}
		if id <= 0 {
			writePanelError(w, http.StatusBadRequest, "id required")
			return
		}

		deleted, found, err := db.DeleteDisabledGame(id)
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to delete disabled game")
			return
		}
		if !found {
			writePanelError(w, http.StatusNotFound, "disabled game not found")
			return
		}

		refreshDisabledGamesCache()
		logging.Event("disabled_game_removed", disabledGameEventData(deleted.DisabledGame, account.Username))
		writePanelJSON(w, http.StatusOK, map[string]bool{"success": true})

	default:
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE")
		writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// HandlePanelGameList returns the known GameSpy games, for the disable-game picker.
func HandlePanelGameList(w http.ResponseWriter, r *http.Request) {
	if _, ok := requirePanelRight(w, r, panelRightManageGames, "manage games"); !ok {
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	writePanelJSON(w, http.StatusOK, common.ListGames())
}

// normalizeDisabledGame trims and validates a disabled game in place. It
// returns a user-facing error message, or "" when the entry is valid.
func normalizeDisabledGame(game *common.DisabledGame) string {
	game.GameCodePrefix = strings.ToUpper(strings.TrimSpace(game.GameCodePrefix))
	game.GameName = strings.TrimSpace(game.GameName)
	game.NASReturnCode = strings.TrimSpace(game.NASReturnCode)
	game.Message = strings.TrimSpace(strings.ReplaceAll(game.Message, "\r\n", "\n"))

	if game.GameCodePrefix == "" && game.GameName == "" {
		return "game code prefix or game name required"
	}
	if game.GameCodePrefix != "" && !disabledGameCodePrefixPattern.MatchString(game.GameCodePrefix) {
		return "game code prefix must be 1-4 letters or digits (e.g. RMC)"
	}
	if game.GameName != "" && common.GetGameInfoByName(game.GameName) == nil {
		return "unknown game name (see game_list.tsv)"
	}

	if game.NASReturnCode == "" {
		game.NASReturnCode = disabledGameDefaultReturnCode
	}
	if !disabledGameReturnCodePattern.MatchString(game.NASReturnCode) {
		return "NAS return code must be 3 digits (e.g. 110)"
	}

	if game.ErrorCode == 0 {
		code, _ := strconv.Atoi(game.NASReturnCode)
		game.ErrorCode = 20000 + code
	}
	if game.ErrorCode < 0 || game.ErrorCode > 99999 {
		return "error code must be between 0 and 99999"
	}

	if utf8.RuneCountInString(game.Message) > disabledGameMessageMaxLength {
		return "message too long (max " + strconv.Itoa(disabledGameMessageMaxLength) + " characters)"
	}

	return ""
}

func refreshDisabledGamesCache() {
	if err := db.ReloadDisabledGamesCache(); err != nil {
		logging.Error("API", "Failed to reload disabled games:", err)
	}
}

func disabledGameEventData(game common.DisabledGame, moderator string) map[string]any {
	return map[string]any{
		"id":               game.ID,
		"game_code_prefix": game.GameCodePrefix,
		"game_name":        game.GameName,
		"nas_return_code":  game.NASReturnCode,
		"error_code":       game.ErrorCode,
		"moderator":        moderator,
	}
}
