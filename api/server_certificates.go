package api

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/logging"
	"net/http"
)

// Registered by main to avoid a dependency cycle: nas also serves API routes.
var generateHTTPSCertificates func(common.Config) error

func SetHTTPSCertificateGenerator(generator func(common.Config) error) {
	generateHTTPSCertificates = generator
}

// HandlePanelHTTPSCertificates creates missing files using the saved config.
// The owner can restart from the same settings screen to load new certificates.
func HandlePanelHTTPSCertificates(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}
	if account.Role != "owner" {
		writePanelError(w, http.StatusForbidden, "owner account required")
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	if generateHTTPSCertificates == nil {
		writePanelError(w, http.StatusServiceUnavailable, "certificate generator unavailable")
		return
	}
	config := common.GetConfig()
	if !config.EnableHTTPS {
		writePanelError(w, http.StatusBadRequest, "enable HTTPS and save the settings first")
		return
	}
	if err := generateHTTPSCertificates(config); err != nil {
		logging.Error("API", "HTTPS certificate generation incomplete:", err)
		writePanelError(w, http.StatusInternalServerError, "certificate generation incomplete: "+err.Error())
		return
	}
	logging.Event("https_certificates_generated", map[string]any{"moderator": account.Username})
	writePanelJSON(w, http.StatusOK, map[string]any{"success": true, "restartRequired": true})
}
