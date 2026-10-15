package race

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/database"
	"net/http"
)

var (
	db database.Connection
)

func StartServer(reload bool) {
	// Get config
	config := common.GetConfig()

	common.ReadGameList()

	// Start SQL
	db = database.Start(config)
}

func Shutdown() {
	db.Close()
}

func RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("POST /RaceService/NintendoRacingService.asmx", handleNintendoRacingServiceRequest)
}
