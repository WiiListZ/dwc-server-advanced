package gpcm

import (
	"dwc-server-advanced/common"
	"testing"
)

func TestMatchesDisabledGame(t *testing.T) {
	tests := []struct {
		name     string
		gameName string
		gameCode string
		game     common.DisabledGame
		want     bool
	}{
		{"by name", "mariokartwii", "RMCP", common.DisabledGame{GameName: "mariokartwii"}, true},
		{"by prefix", "mariokartwii", "RMCE", common.DisabledGame{GameCodePrefix: "RMC"}, true},
		{"by full code", "mariokartwii", "RMCJ", common.DisabledGame{GameCodePrefix: "RMCJ"}, true},
		{"other region code", "mariokartwii", "RMCP", common.DisabledGame{GameCodePrefix: "RMCJ"}, false},
		{"other game", "smashbrosxwii", "RSBE", common.DisabledGame{GameName: "mariokartwii", GameCodePrefix: "RMC"}, false},
		{"empty entry matches nothing", "mariokartwii", "RMCP", common.DisabledGame{}, false},
		{"empty session code", "", "", common.DisabledGame{GameCodePrefix: "RMC"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesDisabledGame(tt.gameName, tt.gameCode, tt.game); got != tt.want {
				t.Fatalf("matchesDisabledGame(%q, %q, %+v) = %v, want %v", tt.gameName, tt.gameCode, tt.game, got, tt.want)
			}
		})
	}
}
