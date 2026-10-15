package api

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/logging"
	"dwc-server-advanced/updater"
	"net/http"
	"os"
	"strings"
	"time"
)

const updateScheduleKey = "update_schedule"

// updateStore keeps the scheduled update in panel_settings.
type updateStore struct{}

func (updateStore) LoadSchedule() (*updater.Schedule, error) {
	var schedule updater.Schedule
	found, err := db.GetSetting(updateScheduleKey, &schedule)
	if err != nil || !found {
		return nil, err
	}
	return &schedule, nil
}

func (updateStore) SaveSchedule(schedule *updater.Schedule) error {
	if schedule == nil {
		return db.SetSetting(updateScheduleKey, nil)
	}
	return db.SetSetting(updateScheduleKey, schedule)
}

func startUpdater() {
	if err := db.UpdateSettingsTable(); err != nil {
		panic(err)
	}
	recordVersionChange()
	updater.Start(updateStore{})
}

// recordVersionChange logs an event the first time a new version starts.
func recordVersionChange() {
	const path = "state/version.txt"
	previous, _ := os.ReadFile(path)
	if prev := strings.TrimSpace(string(previous)); prev != "" && prev != common.Version {
		logging.Event("server_updated", map[string]any{"from": prev, "to": common.Version})
		logging.Notice("API", "Server updated from", prev, "to", common.Version)
	}
	_ = os.WriteFile(path, []byte(common.Version), 0o644)
}

type updateInstallRequest struct {
	// When is "now" or an RFC 3339 time.
	When string `json:"when"`
}

// HandlePanelUpdate is the owner-only update API:
//
//	GET    status
//	POST   ?action=check | body {"when": "now" | "<RFC 3339 time>"}
//	DELETE cancels the scheduled install
func HandlePanelUpdate(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}
	if account.Role != "owner" {
		writePanelError(w, http.StatusForbidden, "owner account required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writePanelJSON(w, http.StatusOK, updater.GetStatus())

	case http.MethodPost:
		if r.URL.Query().Get("action") == "check" {
			status, err := updater.Check()
			if err != nil {
				writePanelError(w, http.StatusBadGateway, "release check failed: "+err.Error())
				return
			}
			writePanelJSON(w, http.StatusOK, status)
			return
		}

		req := updateInstallRequest{}
		if !readPanelJSON(w, r, &req) {
			return
		}
		when := strings.TrimSpace(req.When)
		if when == "" || when == "now" {
			if err := updater.Install(); err != nil {
				writePanelError(w, http.StatusConflict, err.Error())
				return
			}
			logging.Notice("API", "Update installed by", account.Username)
			writePanelJSON(w, http.StatusAccepted, updater.GetStatus())
			return
		}

		at, err := time.Parse(time.RFC3339, when)
		if err != nil {
			writePanelError(w, http.StatusBadRequest, "when must be \"now\" or an RFC 3339 time")
			return
		}
		if at.Before(time.Now().Add(-time.Minute)) {
			writePanelError(w, http.StatusBadRequest, "the scheduled time is in the past")
			return
		}
		schedule, err := updater.ScheduleInstall(at, account.Username)
		if err != nil {
			writePanelError(w, http.StatusConflict, err.Error())
			return
		}
		logging.Event("server_update_scheduled", map[string]any{"version": schedule.Version, "at": schedule.At.Format(time.RFC3339), "moderator": account.Username})
		writePanelJSON(w, http.StatusOK, updater.GetStatus())

	case http.MethodDelete:
		if err := updater.CancelSchedule(); err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to cancel the scheduled update")
			return
		}
		writePanelJSON(w, http.StatusOK, updater.GetStatus())

	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
