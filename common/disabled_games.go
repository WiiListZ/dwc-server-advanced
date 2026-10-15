package common

import (
	"sync"
)

// DisabledGame describes a game blocked from the service. The authoritative
// list lives in the public.disabled_games table; this package only keeps an
// in-memory copy so NAS/GPCM logins never hit the database.
type DisabledGame struct {
	ID             int    `json:"id"`
	GameCodePrefix string `json:"gameCodePrefix"`
	GameName       string `json:"gameName"`
	NASReturnCode  string `json:"nasReturnCode"`
	ErrorCode      int    `json:"errorCode"`
	Message        string `json:"message"`
}

var (
	disabledGames      []DisabledGame
	disabledGamesMutex = sync.RWMutex{}
)

// SetDisabledGames replaces the cached disabled games list. Called by the API
// server at startup and after every panel change.
func SetDisabledGames(games []DisabledGame) {
	copied := append([]DisabledGame(nil), games...)

	disabledGamesMutex.Lock()
	defer disabledGamesMutex.Unlock()

	disabledGames = copied
}

func GetDisabledGameByCode(gamecd []byte) *DisabledGame {
	disabledGamesMutex.RLock()
	defer disabledGamesMutex.RUnlock()

	for i := range disabledGames {
		prefix := disabledGames[i].GameCodePrefix
		if prefix == "" {
			continue
		}
		if len(gamecd) >= len(prefix) && string(gamecd[:len(prefix)]) == prefix {
			game := disabledGames[i]
			return &game
		}
	}
	return nil
}

func GetDisabledGameByName(name string) *DisabledGame {
	disabledGamesMutex.RLock()
	defer disabledGamesMutex.RUnlock()

	for i := range disabledGames {
		if disabledGames[i].GameName != "" && disabledGames[i].GameName == name {
			game := disabledGames[i]
			return &game
		}
	}
	return nil
}
