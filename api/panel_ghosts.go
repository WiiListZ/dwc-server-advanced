package api

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/database"
	"dwc-server-advanced/logging"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type panelMKWRanking struct {
	ID         int     `json:"id"`
	RegionID   int     `json:"regionid"`
	CourseID   int     `json:"courseid"`
	Score      int     `json:"score"`
	PID        uint32  `json:"pid"`
	FriendCode string  `json:"fc"`
	PlayerInfo string  `json:"playerinfo"`
	HasGhost   bool    `json:"has_ghost"`
	GhostSize  int     `json:"ghost_size"`
	UploadTime *string `json:"upload_time,omitempty"`
}

type panelMKWClearProfileRequest struct {
	ProfileID  uint32 `json:"pid"`
	Identifier string `json:"identifier"`
	FriendCode string `json:"fc"`
}

func toPanelMKWRanking(r database.MarioKartWiiPanelRanking) panelMKWRanking {
	out := panelMKWRanking{
		ID:         r.ID,
		RegionID:   r.RegionID,
		CourseID:   r.CourseID,
		Score:      r.Score,
		PID:        r.PID,
		PlayerInfo: r.PlayerInfo,
		HasGhost:   r.HasGhost,
		GhostSize:  r.GhostSize,
	}
	if len(r.Gsbrcd) >= 4 {
		out.FriendCode = common.CalcFriendCodeString(r.PID, r.Gsbrcd[:4])
	}
	if r.UploadTime != nil {
		s := r.UploadTime.UTC().Format("2006-01-02T15:04:05Z")
		out.UploadTime = &s
	}
	return out
}

func HandlePanelMKWTop10(w http.ResponseWriter, r *http.Request) {
	if _, ok := requirePanelRight(w, r, panelRightManageGhosts, "manage ghosts"); !ok {
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	regionId, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("region")))
	if err != nil || regionId < 0 || regionId > 7 {
		writePanelError(w, http.StatusBadRequest, "region must be 0..7 (0 = worldwide)")
		return
	}

	courseId, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("course")))
	if err != nil || courseId < 0 || courseId > 0x7FFF {
		writePanelError(w, http.StatusBadRequest, "course required")
		return
	}

	rankings, err := db.GetMarioKartWiiPanelTop10(regionId, courseId)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to load rankings")
		return
	}

	out := make([]panelMKWRanking, 0, len(rankings))
	for _, rr := range rankings {
		out = append(out, toPanelMKWRanking(rr))
	}

	writePanelJSON(w, http.StatusOK, map[string]any{
		"region":   regionId,
		"course":   courseId,
		"rankings": out,
	})
}

func HandlePanelMKWSearch(w http.ResponseWriter, r *http.Request) {
	if _, ok := requirePanelRight(w, r, panelRightManageGhosts, "manage ghosts"); !ok {
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	query := r.URL.Query()
	identifier := strings.TrimSpace(query.Get("identifier"))
	if identifier == "" {
		identifier = strings.TrimSpace(query.Get("pid"))
	}
	if identifier == "" {
		identifier = strings.TrimSpace(query.Get("fc"))
	}
	if identifier == "" {
		writePanelError(w, http.StatusBadRequest, "identifier, pid, or fc required")
		return
	}

	profileID, exists, err := db.ResolveProfileIdentifier(identifier)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to resolve profile")
		return
	}
	if !exists {
		writePanelError(w, http.StatusNotFound, "profile not found")
		return
	}

	rankings, err := db.GetMarioKartWiiPanelRankingsByProfile(profileID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to load rankings")
		return
	}

	out := make([]panelMKWRanking, 0, len(rankings))
	for _, rr := range rankings {
		out = append(out, toPanelMKWRanking(rr))
	}

	writePanelJSON(w, http.StatusOK, map[string]any{
		"pid":      profileID,
		"rankings": out,
	})
}

func HandlePanelMKWGhostDownload(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightManageGhosts, "manage ghosts")
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	id, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("id")))
	if err != nil || id <= 0 {
		writePanelError(w, http.StatusBadRequest, "id required")
		return
	}

	blob, exists, err := db.GetMarioKartWiiPanelGhostByID(id)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to read ghost")
		return
	}
	if !exists || len(blob.Ghost) == 0 {
		writePanelError(w, http.StatusNotFound, "ghost not found")
		return
	}

	filename := fmt.Sprintf("mkw-ghost-%d-pid%d-course%d.rkg", id, blob.PID, blob.CourseID)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(blob.Ghost)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(blob.Ghost)

	logging.Event("panel_mkw_ghost_downloaded", map[string]any{
		"id":         id,
		"profile_id": blob.PID,
		"course_id":  blob.CourseID,
		"region_id":  blob.RegionID,
		"score":      blob.Score,
		"ghost_size": len(blob.Ghost),
		"moderator":  account.Username,
	})
}

func HandlePanelMKWDeleteRanking(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightManageGhosts, "manage ghosts")
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	var req struct {
		ID int `json:"id"`
	}
	body, err := readBodyBytes(w, r)
	if err != nil {
		return
	}
	if len(body) == 0 {
		if idStr := strings.TrimSpace(r.URL.Query().Get("id")); idStr != "" {
			if id, convErr := strconv.Atoi(idStr); convErr == nil {
				req.ID = id
			}
		}
	} else if err := json.Unmarshal(body, &req); err != nil {
		writePanelError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.ID <= 0 {
		writePanelError(w, http.StatusBadRequest, "id required")
		return
	}

	rows, err := db.DeleteMarioKartWiiPanelRanking(req.ID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to delete ranking")
		return
	}
	if rows == 0 {
		writePanelError(w, http.StatusNotFound, "ranking not found")
		return
	}

	logging.Event("panel_mkw_ranking_deleted", map[string]any{
		"id":        req.ID,
		"moderator": account.Username,
	})

	writePanelJSON(w, http.StatusOK, map[string]any{"success": true, "deleted": rows})
}

func HandlePanelMKWClearProfile(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightManageGhosts, "manage ghosts")
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelMKWClearProfileRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	profileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
	if !ok {
		return
	}

	rows, err := db.DeleteMarioKartWiiPanelRankingsByProfile(profileID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to clear rankings")
		return
	}

	logging.Event("panel_mkw_profile_cleared", map[string]any{
		"profile_id": profileID,
		"deleted":    rows,
		"moderator":  account.Username,
	})

	writePanelJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"profile_id": profileID,
		"deleted":    rows,
	})
}

func readBodyBytes(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	defer func() {
		_ = r.Body.Close()
	}()
	buf := make([]byte, 0, 1024)
	var chunk [512]byte
	for {
		n, err := r.Body.Read(chunk[:])
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			if len(buf) > 1<<20 {
				writePanelError(w, http.StatusBadRequest, "body too large")
				return nil, fmt.Errorf("body too large")
			}
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}
