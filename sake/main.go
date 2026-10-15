package sake

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
	db.RegisterEvents(config, []string{
		"mkw_ghost_downloaded",
		"mkw_ghost_uploaded",
		"mkw_invalid_ghost_upload",
	})
}

func Shutdown() {
	db.Close()
}

func RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("POST /SakeStorageServer/StorageServer.asmx", handleStorageRequest)
	mux.HandleFunc("GET /SakeFileServer/download.aspx", handleFileDownloadRequest)
	mux.HandleFunc("POST /SakeFileServer/upload.aspx", handleFileUploadRequest)
	mux.HandleFunc("POST /SakeFileServer/ghostupload.aspx", handleFileUploadRequest)
	mux.HandleFunc("GET /SakeFileServer/ghostdownload.aspx", handleMarioKartWiiGhostDownloadRequest)
}
