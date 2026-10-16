package api

import (
	"crypto/rand"
	"crypto/sha256"
	"dwc-server-advanced/common"
	"dwc-server-advanced/database"
	"dwc-server-advanced/gpcm"
	"dwc-server-advanced/logging"
	"dwc-server-advanced/qr2"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	panelCookieName = "wwfc_panel_session"
	panelSessionTTL = 7 * 24 * time.Hour
	profileLinkTTL  = 10 * time.Minute
)

const (
	panelRightViewUsers          = "view_users"
	panelRightViewBans           = "view_bans"
	panelRightBan                = "ban"
	panelRightUnban              = "unban"
	panelRightKick               = "kick"
	panelRightMessage            = "message"
	panelRightManageRoomsCreate  = "manage_rooms_create"
	panelRightManageRoomsJoin    = "manage_rooms_join"
	panelRightManageRoomsDelete  = "manage_rooms_delete"
	panelRightManageRoomsEndGame = "manage_rooms_end_game"
	panelRightViewLogs           = "view_logs"
	panelRightViewReports        = "view_reports"
	panelRightManageGhosts       = "manage_ghosts"
	panelRightManageGames        = "manage_games"
)

type panelCredentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type panelBanRequest struct {
	ProfileID    uint32 `json:"pid"`
	Identifier   string `json:"identifier"`
	FriendCode   string `json:"fc"`
	Duration     string `json:"duration"`
	Days         uint64 `json:"days"`
	Hours        uint64 `json:"hours"`
	Minutes      uint64 `json:"minutes"`
	TOS          bool   `json:"tos"`
	Reason       string `json:"reason"`
	ReasonHidden string `json:"reason_hidden"`
}

type panelKickRequest struct {
	ProfileID  uint32 `json:"pid"`
	Identifier string `json:"identifier"`
	FriendCode string `json:"fc"`
	Reason     string `json:"reason"`
}

type panelLoginMessageRequest struct {
	ProfileID  uint32 `json:"pid"`
	Identifier string `json:"identifier"`
	FriendCode string `json:"fc"`
	Message    string `json:"message"`
}

type panelJoinRoomRequest struct {
	ProfileID  uint32 `json:"pid"`
	Identifier string `json:"identifier"`
	FriendCode string `json:"fc"`
	RoomID     string `json:"room_id"`
}

type panelCreateRoomRequest struct {
	ProfileID  uint32 `json:"pid"`
	Identifier string `json:"identifier"`
	FriendCode string `json:"fc"`
	MatchType  string `json:"type"`
}

type panelDeleteRoomRequest struct {
	RoomID          string `json:"room_id"`
	EndMode         string `json:"end_mode"`
	ForceDisconnect bool   `json:"force_disconnect"`
}

type panelProfileLinkRequest struct {
	ProfileID  uint32 `json:"pid"`
	Identifier string `json:"identifier"`
	FriendCode string `json:"fc"`
}

type panelProfileLinkCompleteRequest struct {
	ProfileID   uint32 `json:"pid"`
	Identifier  string `json:"identifier"`
	FriendCode  string `json:"fc"`
	Code        string `json:"code"`
	Placeholder string `json:"placeholder"`
}

type panelRoomKickRequest struct {
	RoomID            string `json:"room_id"`
	ProfileID         uint32 `json:"pid"`
	Identifier        string `json:"identifier"`
	FriendCode        string `json:"fc"`
	Reason            string `json:"reason"`
	BlacklistOpenHost bool   `json:"blacklist_open_host"`
}

type panelRoomReportRequest struct {
	RoomID     string `json:"room_id"`
	ProfileID  uint32 `json:"pid"`
	Identifier string `json:"identifier"`
	FriendCode string `json:"fc"`
	Reason     string `json:"reason"`
}

type panelAccountRoleRequest struct {
	AccountID int                   `json:"id"`
	Role      string                `json:"role"`
	Rights    *database.PanelRights `json:"rights"`
}

type panelUnbanRequest struct {
	ProfileID  uint32 `json:"pid"`
	Identifier string `json:"identifier"`
	FriendCode string `json:"fc"`
}

func HandlePanelSession(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	account, authenticated := currentPanelAccount(r)
	setupRequired, err := setupRequired()
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to read panel setup state")
		return
	}

	response := map[string]any{
		"authenticated":  authenticated,
		"setup_required": setupRequired,
		"features": map[string]bool{
			"profiles": profilesEnabled(),
		},
	}
	if authenticated {
		response["account"] = account
		if profilesEnabled() {
			if profile, err := db.GetSiteProfileByAccount(account.ID); err == nil {
				response["avatar_url"] = profileAvatarURL(profile)
			}
		}
	}

	writePanelJSON(w, http.StatusOK, response)
}

func HandlePanelSetup(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	required, err := setupRequired()
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to read panel setup state")
		return
	}
	if !required {
		writePanelError(w, http.StatusConflict, "owner account already exists")
		return
	}

	req, ok := readCredentials(w, r)
	if !ok {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	account, err := db.CreatePanelAccount(req.Username, hash, "owner")
	if err != nil {
		writePanelError(w, http.StatusConflict, "failed to create owner account")
		return
	}

	if !startPanelSession(w, account) {
		return
	}

	writePanelJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"account": account,
	})
}

func HandlePanelLogin(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req, ok := readCredentials(w, r)
	if !ok {
		return
	}

	account, passwordHash, exists, err := db.GetPanelAccountByUsername(req.Username)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to read account")
		return
	}

	if !exists || bcrypt.CompareHashAndPassword(passwordHash, []byte(req.Password)) != nil {
		writePanelError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if !startPanelSession(w, account) {
		return
	}

	writePanelJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"account": account,
	})
}

func HandlePanelRegister(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	required, err := setupRequired()
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to read panel setup state")
		return
	}
	if required {
		writePanelError(w, http.StatusConflict, "owner account setup required first")
		return
	}

	req, ok := readCredentials(w, r)
	if !ok {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	account, err := db.CreatePanelAccount(req.Username, hash, "player")
	if err != nil {
		writePanelError(w, http.StatusConflict, "failed to create player account")
		return
	}

	if !startPanelSession(w, account) {
		return
	}

	writePanelJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"account": account,
	})
}

func HandlePanelLogout(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	if cookie, err := r.Cookie(panelCookieName); err == nil {
		_ = db.DeletePanelSession(hashSessionToken(cookie.Value))
	}

	http.SetCookie(w, &http.Cookie{
		Name:     panelCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	writePanelJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func HandlePanelAccounts(w http.ResponseWriter, r *http.Request) {
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
		accounts, err := db.ListPanelAccounts()
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to list accounts")
			return
		}

		writePanelJSON(w, http.StatusOK, accounts)

	case http.MethodPost:
		req, ok := readCredentials(w, r)
		if !ok {
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}

		created, err := db.CreatePanelAccount(req.Username, hash, "moderator")
		if err != nil {
			writePanelError(w, http.StatusConflict, "failed to create moderator account")
			return
		}

		writePanelJSON(w, http.StatusCreated, created)

	case http.MethodPatch:
		req := panelAccountRoleRequest{}
		if !readPanelJSON(w, r, &req) {
			return
		}

		if req.AccountID <= 0 {
			writePanelError(w, http.StatusBadRequest, "account ID required")
			return
		}

		var updated database.PanelAccount
		var changed bool
		var err error
		if req.Rights != nil {
			updated, changed, err = db.UpdatePanelAccountRights(req.AccountID, *req.Rights)
		} else {
			role := strings.TrimSpace(strings.ToLower(req.Role))
			if role == "normal" {
				role = "player"
			}
			if role != "player" && role != "moderator" {
				writePanelError(w, http.StatusBadRequest, "role must be normal or moderator")
				return
			}

			updated, changed, err = db.UpdatePanelAccountRole(req.AccountID, role)
		}
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to update account")
			return
		}
		if !changed {
			writePanelError(w, http.StatusNotFound, "account not found or owner role cannot be changed")
			return
		}

		syncLinkedConsoleOpenHostRight(updated)

		writePanelJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		req := panelAccountRoleRequest{}
		if !readPanelJSON(w, r, &req) {
			return
		}
		if req.AccountID <= 0 {
			writePanelError(w, http.StatusBadRequest, "account ID required")
			return
		}
		if req.AccountID == account.ID {
			writePanelError(w, http.StatusBadRequest, "owner account cannot delete itself")
			return
		}

		deleted, err := db.DeletePanelAccount(req.AccountID)
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to delete account")
			return
		}
		if !deleted {
			writePanelError(w, http.StatusNotFound, "account not found or owner account cannot be deleted")
			return
		}

		logging.Event("panel_account_deleted", map[string]any{
			"account_id": req.AccountID,
			"moderator":  account.Username,
		})
		writePanelJSON(w, http.StatusOK, map[string]bool{"success": true})

	default:
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE")
		writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func HandlePanelUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requirePanelRight(w, r, panelRightViewUsers, "view users"); !ok {
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	writePanelJSON(w, http.StatusOK, gpcm.GetOnlineUsers())
}

func HandlePublicUsers(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	writePanelJSON(w, http.StatusOK, publicOnlineUsers())
}

func HandlePanelBans(w http.ResponseWriter, r *http.Request) {
	if _, ok := requirePanelRight(w, r, panelRightViewBans, "view bans"); !ok {
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	writeBans(w, true)
}

func HandlePublicBans(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	writeBans(w, false)
}

func HandlePanelMyBans(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	bans, err := db.ListBans(false)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to list bans")
		return
	}
	consoles, err := db.ListConsoleSummariesForAccount(account.ID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to list linked consoles")
		return
	}
	profileIDs := map[uint32]bool{}
	friendCodes := map[string]bool{}
	consoleIDs := map[int]bool{}
	for _, id := range panelAccountProfileIDs(account) {
		profileIDs[id] = true
	}
	if account.FriendCode != "" {
		friendCodes[account.FriendCode] = true
	}
	for _, profile := range account.LinkedProfiles {
		if profile.FriendCode != "" {
			friendCodes[profile.FriendCode] = true
		}
	}
	for _, console := range consoles {
		consoleIDs[console.ID] = true
	}

	mine := []database.BanRecord{}
	for _, ban := range bans {
		if !profileIDs[ban.ProfileID] && !friendCodes[ban.FriendCode] && !consoleIDs[ban.ConsoleID] {
			continue
		}
		ban.ProfileID = 0
		ban.TOS = false
		mine = append(mine, ban)
	}
	writePanelJSON(w, http.StatusOK, mine)
}

func HandlePanelRooms(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	if account.Role == "player" {
		if account.ProfileID == 0 {
			writePanelError(w, http.StatusForbidden, "linked profile required")
			return
		}
		writePanelJSON(w, http.StatusOK, ownedRooms(account.ProfileID))
		return
	}
	if !panelAccountHasAnyRoomRight(account) {
		writePanelError(w, http.StatusForbidden, "matchmaking room right required")
		return
	}

	writeRooms(w, true)
}

func HandlePanelMatches(w http.ResponseWriter, r *http.Request) {
	if _, ok := requirePanelRight(w, r, panelRightViewLogs, "view logs"); !ok {
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	limit := 200
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			writePanelError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = parsed
	}

	matches, err := db.ListPanelMatchLogs(limit)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to list match logs")
		return
	}

	writePanelJSON(w, http.StatusOK, matches)
}

func HandlePanelLogs(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightViewLogs, "view logs")
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	var fromTime, toTime time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writePanelError(w, http.StatusBadRequest, "invalid from")
			return
		}
		fromTime = parsed.UTC()
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writePanelError(w, http.StatusBadRequest, "invalid to")
			return
		}
		toTime = parsed.UTC()
	}

	limit := 2000
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			writePanelError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = parsed
	}

	logs, err := db.ListPanelEventLogs(fromTime, toTime, true, limit)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to list logs")
		return
	}

	if account.Role != "owner" && len(account.Rights.LogTypes) > 0 {
		allowed := map[string]bool{}
		for _, t := range account.Rights.LogTypes {
			allowed[t] = true
		}
		filtered := make([]database.PanelEventLog, 0, len(logs))
		for _, log := range logs {
			if allowed[log.Type] {
				filtered = append(filtered, log)
			}
		}
		logs = filtered
	}

	writePanelJSON(w, http.StatusOK, logs)
}

func HandlePublicRooms(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	ownProfileIDs := []uint32{}
	if account, ok := currentPanelAccount(r); ok {
		ownProfileIDs = panelAccountProfileIDs(account)
	}
	writePublicRooms(w, ownProfileIDs)
}

func HandlePanelProfileLinkStart(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelProfileLinkRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	profileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
	if !ok {
		return
	}
	if _, exists := db.GetProfile(profileID); !exists {
		writePanelError(w, http.StatusNotFound, "profile not found")
		return
	}
	linked, err := db.IsProfileLinkedToOtherPanelAccount(profileID, account.ID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to check linked profile")
		return
	}
	if linked {
		writePanelError(w, http.StatusConflict, "profile is already linked to another account")
		return
	}
	if target := gpcm.GetLoginMessageTargetInfo(profileID); target.Online && !target.PayloadFeatures {
		writePanelError(w, http.StatusConflict, "this online player has no payload features; the link code cannot be shown")
		return
	}

	code, err := newProfileLinkCode()
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to create link code")
		return
	}

	expiresAt := time.Now().UTC().Add(profileLinkTTL)
	if err := db.CreatePanelProfileLinkCode(account.ID, profileID, hashProfileLinkCode(account.ID, profileID, code), expiresAt); err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to store link code")
		return
	}

	message := fmt.Sprintf("%s site link code: %s\nExpires in 10 minutes.", configuredServerName(), code)
	loginMessage, err := db.CreatePanelLoginMessage(profileID, message, account.Username)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to create link message")
		return
	}

	targetInfo, delivered, err := gpcm.DeliverPendingLoginMessage(profileID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to deliver link message")
		return
	}

	notice := "Link code queued for the next Mario Kart Wii payload login. It will show once."
	if delivered {
		notice = "Link code sent to the online payload client."
	} else if targetInfo.Online && targetInfo.PayloadFeatures {
		notice = "Link code queued. It will show once on the next payload login."
	}

	logging.Event("profile_link_requested", map[string]any{
		"profile_id":       profileID,
		"account":          account.Username,
		"message_id":       loginMessage.ID,
		"target_online":    targetInfo.Online,
		"payload_features": targetInfo.PayloadFeatures,
		"delivered_now":    delivered,
	})

	writePanelJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"pid":      profileID,
		"expires":  expiresAt,
		"delivery": targetInfo,
		"sent_now": delivered,
		"notice":   notice,
	})
}

func HandlePanelProfileLinkComplete(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelProfileLinkCompleteRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	profileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
	if !ok {
		return
	}

	code := strings.TrimSpace(req.Code)
	if len(code) != 6 {
		writePanelError(w, http.StatusBadRequest, "six digit link code required")
		return
	}
	for _, ch := range code {
		if ch < '0' || ch > '9' {
			writePanelError(w, http.StatusBadRequest, "six digit link code required")
			return
		}
	}

	linked, err := db.IsProfileLinkedToOtherPanelAccount(profileID, account.ID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to check linked profile")
		return
	}
	if linked {
		writePanelError(w, http.StatusConflict, "profile is already linked to another account")
		return
	}

	consumed, err := db.ConsumePanelProfileLinkCode(account.ID, profileID, hashProfileLinkCode(account.ID, profileID, code))
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to verify link code")
		return
	}
	if !consumed {
		writePanelError(w, http.StatusUnauthorized, "invalid or expired link code")
		return
	}

	consoleID, err := db.ResolveConsoleForProfile(profileID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to resolve console")
		return
	}
	if consoleID == 0 {
		writePanelError(w, http.StatusConflict, "no console known for this profile yet, log in once on the game first")
		return
	}

	linkedAccount, ok, err := db.LinkPanelAccountConsole(account.ID, consoleID, profileID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to link console")
		return
	}
	if !ok {
		writePanelError(w, http.StatusConflict, "console is already linked to another account")
		return
	}
	placeholder := strings.TrimSpace(req.Placeholder)
	if len([]rune(placeholder)) > 80 {
		writePanelError(w, http.StatusBadRequest, "console placeholder must be 80 characters or less")
		return
	}
	if placeholder != "" {
		if _, err := db.SetConsoleDisplayName(account.ID, consoleID, placeholder); err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to save console placeholder")
			return
		}
		reloaded, found, err := db.GetPanelAccountByID(account.ID)
		if err != nil || !found {
			writePanelError(w, http.StatusInternalServerError, "failed to reload account")
			return
		}
		linkedAccount = reloaded
	}

	syncLinkedConsoleOpenHostRight(linkedAccount)

	logging.Event("profile_linked", map[string]any{
		"profile_id": profileID,
		"console_id": consoleID,
		"account":    account.Username,
	})

	writePanelJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"account": linkedAccount,
	})
}

func HandlePanelProfileUnlink(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelConsoleRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	if account.ProfileID == 0 && len(account.LinkedProfiles) == 0 {
		writePanelError(w, http.StatusConflict, "no console linked")
		return
	}

	previousProfileID := account.ProfileID
	updated, ok, err := db.UnlinkPanelAccountConsole(account.ID, req.ConsoleID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to unlink console")
		return
	}
	if !ok {
		writePanelError(w, http.StatusNotFound, "account not found")
		return
	}

	logging.Event("profile_unlinked", map[string]any{
		"profile_id": previousProfileID,
		"account":    account.Username,
	})

	writePanelJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"account": updated,
	})
}

type panelConsolePrimaryRequest struct {
	ProfileID  uint32 `json:"pid"`
	Identifier string `json:"identifier"`
	FriendCode string `json:"fc"`
}

type panelConsoleRequest struct {
	ConsoleID   int    `json:"console_id"`
	Placeholder string `json:"placeholder"`
}

func HandlePanelConsoleSummary(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		consoles, err := db.ListConsoleSummariesForAccount(account.ID)
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to load consoles")
			return
		}
		if len(consoles) == 0 {
			writePanelJSON(w, http.StatusOK, map[string]any{"linked": false, "consoles": []database.ConsoleSummary{}})
			return
		}

		writePanelJSON(w, http.StatusOK, map[string]any{
			"linked":   true,
			"console":  consoles[0],
			"consoles": consoles,
		})

	case http.MethodPatch:
		req := panelConsoleRequest{}
		if !readPanelJSON(w, r, &req) {
			return
		}
		if req.ConsoleID <= 0 {
			writePanelError(w, http.StatusBadRequest, "console ID required")
			return
		}
		placeholder := strings.TrimSpace(req.Placeholder)
		if len([]rune(placeholder)) > 80 {
			writePanelError(w, http.StatusBadRequest, "console placeholder must be 80 characters or less")
			return
		}
		updated, err := db.SetConsoleDisplayName(account.ID, req.ConsoleID, placeholder)
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to update console")
			return
		}
		if !updated {
			writePanelError(w, http.StatusNotFound, "console not found")
			return
		}

		consoles, err := db.ListConsoleSummariesForAccount(account.ID)
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to load consoles")
			return
		}
		writePanelJSON(w, http.StatusOK, map[string]any{
			"success":  true,
			"linked":   len(consoles) > 0,
			"console":  firstConsoleSummary(consoles),
			"consoles": consoles,
		})

	default:
		w.Header().Set("Allow", "GET, PATCH")
		writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func firstConsoleSummary(consoles []database.ConsoleSummary) any {
	if len(consoles) == 0 {
		return nil
	}
	return consoles[0]
}

func HandlePanelConsolePrimary(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelConsolePrimaryRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	profileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
	if !ok {
		return
	}

	consoles, err := db.ListConsoleSummariesForAccount(account.ID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to load console")
		return
	}
	if len(consoles) == 0 {
		writePanelError(w, http.StatusConflict, "no console linked to this account")
		return
	}
	belongs := false
	for _, summary := range consoles {
		for _, p := range summary.Profiles {
			if p.ProfileID == profileID {
				belongs = true
				break
			}
		}
		if belongs {
			break
		}
	}
	if !belongs {
		writePanelError(w, http.StatusBadRequest, "profile is not on this console")
		return
	}

	if err := db.SetConsolePrimaryProfile(account.ID, profileID); err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to set primary profile")
		return
	}

	updated, err := db.ListConsoleSummariesForAccount(account.ID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to load console")
		return
	}

	writePanelJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"console":  firstConsoleSummary(updated),
		"consoles": updated,
	})
}

func HandlePanelMyRooms(w http.ResponseWriter, r *http.Request) {
	_, profileID, ok := requireLinkedPlayerAccount(w, r)
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	writePanelJSON(w, http.StatusOK, ownedRooms(profileID))
}

type panelOpenHostRequest struct {
	Enabled         bool     `json:"enabled"`
	ProfileID       uint32   `json:"pid"`
	Mode            string   `json:"mode"`
	List            []uint32 `json:"list"`
	ListDisplay     []string `json:"list_display"`
	ListIdentifiers []string `json:"list_identifiers"`
}

type panelOpenHostProfile struct {
	ProfileID         uint32   `json:"pid"`
	FriendCode        string   `json:"fc,omitempty"`
	Name              string   `json:"name,omitempty"`
	IsPrimary         bool     `json:"primary"`
	Enabled           bool     `json:"enabled"`
	Mode              string   `json:"mode"`
	List              []uint32 `json:"list"`
	ListDisplay       []string `json:"list_display"`
	Online            bool     `json:"online"`
	RightLocked       bool     `json:"right_locked"`
	ConsoleID         int      `json:"console_id,omitempty"`
	ConsoleName       string   `json:"console_name,omitempty"`
	ConsoleType       string   `json:"console_type,omitempty"`
	ConsoleFriendCode string   `json:"console_fc,omitempty"`
}

func requireLinkedAccount(w http.ResponseWriter, r *http.Request) (database.PanelAccount, bool) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return database.PanelAccount{}, false
	}
	if account.ProfileID == 0 && len(account.LinkedProfiles) == 0 {
		writePanelError(w, http.StatusForbidden, "linked console required")
		return database.PanelAccount{}, false
	}
	return account, true
}

func resolveOpenHostTarget(account database.PanelAccount, requested uint32) (uint32, int, string) {
	summaries, err := db.ListConsoleSummariesForAccount(account.ID)
	if err != nil {
		return 0, http.StatusInternalServerError, "failed to load console"
	}
	if len(summaries) == 0 {
		return 0, http.StatusForbidden, "no console linked to this account"
	}

	target := requested
	if target == 0 {
		target = account.ProfileID
		if target == 0 {
			profileIDs := panelAccountProfileIDs(account)
			if len(profileIDs) > 0 {
				target = profileIDs[0]
			}
		}
	}

	for _, summary := range summaries {
		for _, p := range summary.Profiles {
			if p.ProfileID == target {
				return target, 0, ""
			}
		}
	}
	return 0, http.StatusBadRequest, "profile is not on this console"
}

func syncLinkedConsoleOpenHostRight(account database.PanelAccount) {
	if account.ProfileID == 0 && len(account.LinkedProfiles) == 0 {
		return
	}

	profileIDs := []uint32{}
	summaries, err := db.ListConsoleSummariesForAccount(account.ID)
	if err != nil {
		logging.Error("API:OpenHost", "failed to load console while syncing OpenHost right for account", account.ID, "err:", err.Error())
	} else {
		for _, summary := range summaries {
			for _, p := range summary.Profiles {
				profileIDs = append(profileIDs, p.ProfileID)
			}
		}
	}
	if len(profileIDs) == 0 {
		for _, profileID := range panelAccountProfileIDs(account) {
			profileIDs = append(profileIDs, profileID)
		}
	}

	for _, profileID := range profileIDs {
		if _, err := db.SetMiscRight(profileID, database.MiscRightOpenHost, account.Rights.OpenHost); err != nil {
			logging.Error("API:OpenHost", "propagate OpenHost right failed for", profileID, "err:", err.Error())
			continue
		}
		if !account.Rights.OpenHost {
			gpcm.SetOpenHostMode(profileID, database.OpenHostModeDisabled, nil)
			qr2.SetOpenHostMode(profileID, database.OpenHostModeDisabled, nil)
		}
	}
}

func HandlePanelMyOpenHost(w http.ResponseWriter, r *http.Request) {
	account, ok := requireLinkedAccount(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		summaries, err := db.ListConsoleSummariesForAccount(account.ID)
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to load console")
			return
		}
		if len(summaries) == 0 {
			writePanelError(w, http.StatusForbidden, "no console linked to this account")
			return
		}

		profiles := []panelOpenHostProfile{}
		for _, summary := range summaries {
			for _, p := range summary.Profiles {
				mode, list, display, _, found, err := db.GetUserOpenHostModeWithDisplay(p.ProfileID)
				if err != nil || !found {
					mode = database.OpenHostModeDisabled
					list = nil
					display = nil
				}
				profiles = append(profiles, panelOpenHostProfile{
					ProfileID:         p.ProfileID,
					FriendCode:        p.FriendCode,
					Name:              p.InGameName,
					IsPrimary:         p.IsPrimary,
					Enabled:           p.OpenHost,
					Mode:              mode,
					List:              list,
					ListDisplay:       display,
					Online:            gpcm.IsLoggedIn(p.ProfileID),
					RightLocked:       !account.Rights.OpenHost,
					ConsoleID:         summary.ID,
					ConsoleName:       summary.DisplayName,
					ConsoleType:       summary.ConsoleType,
					ConsoleFriendCode: summary.ConsoleFriendCode,
				})
			}
		}

		writePanelJSON(w, http.StatusOK, map[string]any{
			"profiles": profiles,
			"consoles": summaries,
			"modes": []map[string]string{
				{"value": database.OpenHostModeDisabled, "label": "Disabled", "description": "Default. Strangers cannot auto-friend you. Only mutual friend-code adds become authorized; classic Wii FC behavior."},
				{"value": database.OpenHostModeWhitelist, "label": "Whitelist", "description": "Auto-friend ONLY the players you list. Anyone else still needs a mutual friend-code add."},
				{"value": database.OpenHostModeBlacklist, "label": "Blacklist", "description": "Auto-friend everyone EXCEPT the players you list. The listed players are blocked from your hosted matches even if they have your FC."},
			},
		})

	case http.MethodPost:
		req := panelOpenHostRequest{}
		if !readPanelJSON(w, r, &req) {
			return
		}
		if !account.Rights.OpenHost {
			writePanelError(w, http.StatusForbidden, "open host right revoked by owner")
			return
		}

		profileID, status, msg := resolveOpenHostTarget(account, req.ProfileID)
		if profileID == 0 {
			writePanelError(w, status, msg)
			return
		}

		mode := strings.TrimSpace(strings.ToLower(req.Mode))
		if mode == "" {
			if req.Enabled {
				mode = database.OpenHostModeBlacklist
			} else {
				mode = database.OpenHostModeDisabled
			}
		}
		if mode != database.OpenHostModeDisabled && mode != database.OpenHostModeWhitelist && mode != database.OpenHostModeBlacklist {
			writePanelError(w, http.StatusBadRequest, "mode must be disabled, whitelist, or blacklist")
			return
		}

		merged := make([]uint32, 0, len(req.List)+len(req.ListIdentifiers))
		display := make([]string, 0, len(req.List)+len(req.ListIdentifiers))
		for i, id := range req.List {
			if id == 0 {
				continue
			}
			label := ""
			if i < len(req.ListDisplay) {
				label = req.ListDisplay[i]
			}
			merged = append(merged, id)
			display = append(display, canonicalOpenHostListDisplay(id, label))
		}
		for _, ident := range req.ListIdentifiers {
			ident = strings.TrimSpace(ident)
			if ident == "" {
				continue
			}
			resolved, exists, err := db.ResolveProfileIdentifier(ident)
			if err != nil {
				writePanelError(w, http.StatusInternalServerError, "failed to resolve identifier: "+err.Error())
				return
			}
			if !exists || resolved == 0 {
				writePanelError(w, http.StatusBadRequest, "could not resolve identifier: "+ident)
				return
			}
			merged = append(merged, resolved)
			display = append(display, canonicalOpenHostIdentifierDisplay(resolved, ident))
		}
		list, listDisplay := dedupeOpenHostEntries(merged, display)
		if len(list) > 200 {
			writePanelError(w, http.StatusBadRequest, "list may not contain more than 200 entries")
			return
		}
		for _, pid := range list {
			if pid == 0 || pid == profileID {
				writePanelError(w, http.StatusBadRequest, "list contains an invalid profile ID")
				return
			}
		}

		if _, err := db.SetMiscRight(profileID, database.MiscRightOpenHost, true); err != nil {
			logging.Error("API:OpenHost", "failed to sync OpenHost right for", profileID, "err:", err.Error())
			writePanelError(w, http.StatusInternalServerError, "failed to sync open host right")
			return
		}
		if err := db.SetUserOpenHostModeWithDisplay(profileID, mode, list, listDisplay); err != nil {
			if errors.Is(err, database.ErrOpenHostRightRevoked) {
				writePanelError(w, http.StatusForbidden, "open host has been disabled for this profile by a moderator")
				return
			}
			logging.Error("API:OpenHost", "SetUserOpenHostMode failed for", profileID, "mode", mode, "list_len", len(list), "err:", err.Error())
			writePanelError(w, http.StatusInternalServerError, "failed to update open host mode: "+err.Error())
			return
		}

		gpcm.SetOpenHostMode(profileID, mode, list)
		qr2.SetOpenHostMode(profileID, mode, list)

		logging.Event("profile_open_host_changed", map[string]any{
			"profile_id": profileID,
			"mode":       mode,
			"list_count": len(list),
			"account":    account.Username,
		})

		writePanelJSON(w, http.StatusOK, map[string]any{
			"success":      true,
			"pid":          profileID,
			"enabled":      mode != database.OpenHostModeDisabled,
			"mode":         mode,
			"list":         list,
			"list_display": listDisplay,
			"online":       gpcm.IsLoggedIn(profileID),
		})

	default:
		w.Header().Set("Allow", "GET, POST")
		writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type panelMiscRightsRequest struct {
	ProfileID  uint32 `json:"pid"`
	Identifier string `json:"identifier"`
	FriendCode string `json:"fc"`
	Key        string `json:"key"`
	Value      bool   `json:"value"`
}

type panelMiscRightDescriptor struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
}

var panelMiscRightCatalog = []panelMiscRightDescriptor{
	{
		Key:         string(database.MiscRightOpenHost),
		Label:       "Open Host",
		Description: "Lets the profile use Open Host (auto-friend strangers so they can join hosted matches without manual approval). Disabling this forces the profile's Open Host mode to 'disabled' and prevents the user from re-enabling it.",
		Default:     true,
	},
}

func HandlePanelMiscRights(w http.ResponseWriter, r *http.Request) {
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
		query := r.URL.Query()
		profileID, ok := resolvePanelProfileID(w, parseUint32Query(query.Get("pid")), query.Get("identifier"), query.Get("fc"))
		if !ok {
			return
		}

		rights, found, err := db.GetMiscRights(profileID)
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to load misc rights")
			return
		}
		if !found {
			writePanelError(w, http.StatusNotFound, "profile not found")
			return
		}

		writePanelJSON(w, http.StatusOK, map[string]any{
			"pid":     profileID,
			"catalog": panelMiscRightCatalog,
			"rights": map[string]bool{
				string(database.MiscRightOpenHost): rights.OpenHost,
			},
		})

	case http.MethodPost:
		req := panelMiscRightsRequest{}
		if !readPanelJSON(w, r, &req) {
			return
		}

		profileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
		if !ok {
			return
		}

		key := database.MiscRightKey(strings.TrimSpace(strings.ToLower(req.Key)))
		known := false
		for _, k := range database.AllMiscRightKeys() {
			if k == key {
				known = true
				break
			}
		}
		if !known {
			writePanelError(w, http.StatusBadRequest, "unknown misc right key")
			return
		}

		updated, err := db.SetMiscRight(profileID, key, req.Value)
		if err != nil {
			logging.Error("API:MiscRights", "SetMiscRight failed for", profileID, "key", string(key), "value", req.Value, "err:", err.Error())
			writePanelError(w, http.StatusInternalServerError, "failed to update misc right: "+err.Error())
			return
		}
		if !updated {
			writePanelError(w, http.StatusNotFound, "profile not found")
			return
		}

		// Side effects per right.
		if key == database.MiscRightOpenHost && !req.Value {
			gpcm.SetOpenHostMode(profileID, database.OpenHostModeDisabled, nil)
			qr2.SetOpenHostMode(profileID, database.OpenHostModeDisabled, nil)
		}

		logging.Event("profile_misc_right_changed", map[string]any{
			"profile_id": profileID,
			"key":        string(key),
			"value":      req.Value,
			"moderator":  account.Username,
		})

		writePanelJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"pid":     profileID,
			"key":     string(key),
			"value":   req.Value,
		})

	default:
		w.Header().Set("Allow", "GET, POST")
		writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func parseUint32Query(s string) uint32 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(v)
}

func dedupeProfileIDs(in []uint32) []uint32 {
	if len(in) == 0 {
		return []uint32{}
	}
	seen := make(map[uint32]struct{}, len(in))
	out := make([]uint32, 0, len(in))
	for _, v := range in {
		if v == 0 {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func dedupeOpenHostEntries(ids []uint32, displays []string) ([]uint32, []string) {
	if len(ids) == 0 {
		return []uint32{}, []string{}
	}
	seen := make(map[uint32]struct{}, len(ids))
	outIDs := make([]uint32, 0, len(ids))
	outDisplay := make([]string, 0, len(ids))
	for i, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		label := ""
		if i < len(displays) {
			label = strings.TrimSpace(displays[i])
		}
		if label == "" {
			label = openHostPIDDisplay(id)
		}
		outIDs = append(outIDs, id)
		outDisplay = append(outDisplay, label)
	}
	return outIDs, outDisplay
}

func canonicalOpenHostListDisplay(profileID uint32, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return openHostPIDDisplay(profileID)
	}

	if digits := openHostFriendCodeDigits(raw); len(digits) == 12 {
		if resolved, exists, err := db.ResolveProfileIdentifier(digits); err == nil && exists && resolved == profileID {
			return "FC " + formatOpenHostFriendCode(digits)
		}
	}

	candidate := strings.TrimSpace(raw)
	if len(candidate) >= 3 && strings.EqualFold(candidate[:3], "PID") {
		candidate = strings.TrimSpace(candidate[3:])
	}
	if parsed, err := strconv.ParseUint(candidate, 10, 32); err == nil && uint32(parsed) == profileID {
		return openHostPIDDisplay(profileID)
	}

	return openHostPIDDisplay(profileID)
}

func canonicalOpenHostIdentifierDisplay(profileID uint32, identifier string) string {
	if digits := openHostFriendCodeDigits(identifier); len(digits) == 12 {
		return "FC " + formatOpenHostFriendCode(digits)
	}
	return openHostPIDDisplay(profileID)
}

func openHostPIDDisplay(profileID uint32) string {
	return "PID " + strconv.FormatUint(uint64(profileID), 10)
}

func openHostFriendCodeDigits(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, ch := range value {
		if ch >= '0' && ch <= '9' {
			builder.WriteRune(ch)
		}
	}
	return builder.String()
}

func formatOpenHostFriendCode(digits string) string {
	if len(digits) != 12 {
		return digits
	}
	return digits[0:4] + "-" + digits[4:8] + "-" + digits[8:12]
}

func HandlePanelMyRoomKick(w http.ResponseWriter, r *http.Request) {
	account, ownerProfileID, ok := requireLinkedPlayerAccount(w, r)
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	if !account.Rights.PrivateRoomKick {
		writePanelError(w, http.StatusForbidden, "private room kick right required")
		return
	}

	req := panelRoomKickRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	roomID := strings.TrimSpace(req.RoomID)
	if roomID == "" {
		writePanelError(w, http.StatusBadRequest, "room ID required")
		return
	}

	targetProfileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
	if !ok {
		return
	}
	if targetProfileID == ownerProfileID {
		writePanelError(w, http.StatusBadRequest, "cannot kick your own linked profile")
		return
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		writePanelError(w, http.StatusBadRequest, "reason required")
		return
	}
	if len([]rune(reason)) > 200 {
		writePanelError(w, http.StatusBadRequest, "reason must be 200 characters or less")
		return
	}

	room, player, found := ownedRoomPlayer(ownerProfileID, roomID, targetProfileID)
	if !found {
		writePanelError(w, http.StatusForbidden, "target is not in one of your rooms")
		return
	}
	if req.BlacklistOpenHost {
		if room.MatchType != "private" {
			writePanelError(w, http.StatusBadRequest, "open host blacklist can only be changed from a private room kick")
			return
		}
		if !account.Rights.OpenHost {
			writePanelError(w, http.StatusForbidden, "open host right revoked by owner")
			return
		}
		if err := blacklistOpenHostKickTarget(ownerProfileID, targetProfileID, player.FriendCode, account.Username); err != nil {
			if errors.Is(err, database.ErrOpenHostRightRevoked) {
				writePanelError(w, http.StatusForbidden, "open host has been disabled for this profile by a moderator")
				return
			}
			logging.Error("API:OpenHost", "failed to blacklist kicked player", targetProfileID, "for", ownerProfileID, "err:", err.Error())
			writePanelError(w, http.StatusInternalServerError, "failed to update open host blacklist")
			return
		}
	}

	if err := db.RecordRoomKick(room.ID, room.GameName, ownerProfileID, targetProfileID, player.InGameName, player.FriendCode, reason, account.Username); err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to record room kick")
		return
	}

	gpcm.KickPlayerCustomMessage(targetProfileID, reason, gpcm.WWFCMsgKickedRoomHostCustom)

	logging.Event("room_player_kicked", map[string]any{
		"profile_id":       targetProfileID,
		"owner_profile_id": ownerProfileID,
		"room_id":          room.ID,
		"game_name":        room.GameName,
		"reason":           reason,
		"moderator":        account.Username,
		"open_host_block":  req.BlacklistOpenHost,
	})

	writePanelJSON(w, http.StatusOK, map[string]bool{"success": true, "open_host_block": req.BlacklistOpenHost})
}

func blacklistOpenHostKickTarget(ownerProfileID uint32, targetProfileID uint32, targetFriendCode string, account string) error {
	mode, list, display, _, found, err := db.GetUserOpenHostModeWithDisplay(ownerProfileID)
	if err != nil {
		return err
	}
	if !found || mode != database.OpenHostModeBlacklist {
		list = []uint32{}
		display = []string{}
	}

	label := targetFriendCode
	if strings.TrimSpace(label) == "" {
		label = openHostPIDDisplay(targetProfileID)
	}
	list = append(list, targetProfileID)
	display = append(display, canonicalOpenHostIdentifierDisplay(targetProfileID, label))
	list, display = dedupeOpenHostEntries(list, display)

	if err := db.SetUserOpenHostModeWithDisplay(ownerProfileID, database.OpenHostModeBlacklist, list, display); err != nil {
		return err
	}
	gpcm.SetOpenHostMode(ownerProfileID, database.OpenHostModeBlacklist, list)
	qr2.SetOpenHostMode(ownerProfileID, database.OpenHostModeBlacklist, list)

	logging.Event("profile_open_host_changed", map[string]any{
		"profile_id":      ownerProfileID,
		"mode":            database.OpenHostModeBlacklist,
		"list_count":      len(list),
		"blocked_profile": targetProfileID,
		"account":         account,
		"source":          "private_room_kick",
	})
	return nil
}

func HandlePanelMyRoomReport(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	reporterProfileIDs := panelAccountProfileIDs(account)
	if len(reporterProfileIDs) == 0 {
		writePanelError(w, http.StatusForbidden, "linked profile required")
		return
	}

	req := panelRoomReportRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	roomID := strings.TrimSpace(req.RoomID)
	if roomID == "" {
		writePanelError(w, http.StatusBadRequest, "room ID required")
		return
	}

	targetProfileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
	if !ok {
		return
	}
	for _, reporterProfileID := range reporterProfileIDs {
		if targetProfileID == reporterProfileID {
			writePanelError(w, http.StatusBadRequest, "cannot report your own linked profile")
			return
		}
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		writePanelError(w, http.StatusBadRequest, "reason required")
		return
	}
	if len([]rune(reason)) > 500 {
		writePanelError(w, http.StatusBadRequest, "reason must be 500 characters or less")
		return
	}

	room, reporterProfileID, reporterPlayer, targetPlayer, found := roomWithReporterAndTarget(reporterProfileIDs, roomID, targetProfileID)
	if !found {
		writePanelError(w, http.StatusForbidden, "target is not in a room with your linked profile")
		return
	}

	reporterName := account.InGameName
	reporterFC := account.FriendCode
	if reporterPlayer.ProfileID != "" {
		reporterName = reporterPlayer.InGameName
		reporterFC = reporterPlayer.FriendCode
	} else if name, fc := panelAccountProfileDisplay(account, reporterProfileID); name != "" || fc != "" {
		reporterName = name
		reporterFC = fc
	}

	reporterType := panelReportType(account)
	err := db.RecordRoomReport(room.ID, room.GameName, room.MatchType, reporterProfileID, reporterName, reporterFC, reporterType, targetProfileID, targetPlayer.InGameName, targetPlayer.FriendCode, reason)
	if errors.Is(err, database.ErrDuplicateRoomReport) {
		writePanelError(w, http.StatusConflict, "you already reported this player")
		return
	}
	if err != nil {
		logging.Error("API:Reports", "RecordRoomReport failed for reporter", reporterProfileID, "target", targetProfileID, "err:", err.Error())
		writePanelError(w, http.StatusInternalServerError, "failed to record report")
		return
	}

	logging.Event("room_player_reported", map[string]any{
		"profile_id":          targetProfileID,
		"reporter_profile_id": reporterProfileID,
		"room_id":             room.ID,
		"game_name":           room.GameName,
		"room_type":           room.MatchType,
		"reason":              reason,
		"reporter_type":       reporterType,
		"account":             account.Username,
	})

	writePanelJSON(w, http.StatusOK, map[string]bool{"success": true})
}

type panelRoomReportSummaryResponse struct {
	database.PanelRoomReportSummary
	Online   bool   `json:"online"`
	RoomID   string `json:"room_id,omitempty"`
	RoomType string `json:"room_type,omitempty"`
	RoomGame string `json:"room_game,omitempty"`
}

func HandlePanelReports(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightViewReports, "view reports")
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		reports, err := db.ListRoomReportSummaries()
		if err != nil {
			logging.Error("API:Reports", "ListRoomReportSummaries failed:", err.Error())
			writePanelError(w, http.StatusInternalServerError, "failed to load reports")
			return
		}

		out := make([]panelRoomReportSummaryResponse, 0, len(reports))
		for _, report := range reports {
			row := panelRoomReportSummaryResponse{PanelRoomReportSummary: report}
			if room, _, ok := activeRoomForProfile(report.ProfileID); ok {
				row.Online = true
				row.RoomID = room.ID
				row.RoomType = room.MatchType
				row.RoomGame = room.GameName
			}
			out = append(out, row)
		}
		writePanelJSON(w, http.StatusOK, out)

	case http.MethodDelete:
		profileID := parseUint32Query(r.URL.Query().Get("pid"))
		if profileID == 0 {
			req := panelRoomReportRequest{}
			if !readPanelJSON(w, r, &req) {
				return
			}
			var resolved bool
			profileID, resolved = resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
			if !resolved {
				return
			}
		}
		removed, err := db.ClearRoomReportsForProfile(profileID)
		if err != nil {
			writePanelError(w, http.StatusInternalServerError, "failed to remove reports")
			return
		}
		logging.Event("room_player_reports_removed", map[string]any{
			"profile_id": profileID,
			"removed":    removed,
			"moderator":  account.Username,
		})
		writePanelJSON(w, http.StatusOK, map[string]any{"success": true, "removed": removed})

	default:
		w.Header().Set("Allow", "GET, DELETE")
		writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func HandlePanelReportDetails(w http.ResponseWriter, r *http.Request) {
	if _, ok := requirePanelRight(w, r, panelRightViewReports, "view reports"); !ok {
		return
	}
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	profileID := parseUint32Query(r.URL.Query().Get("pid"))
	if profileID == 0 {
		writePanelError(w, http.StatusBadRequest, "profile ID required")
		return
	}

	reports, err := db.ListRoomReportDetails(profileID)
	if err != nil {
		logging.Error("API:Reports", "ListRoomReportDetails failed for", profileID, "err:", err.Error())
		writePanelError(w, http.StatusInternalServerError, "failed to load report details")
		return
	}
	writePanelJSON(w, http.StatusOK, reports)
}

func clearReportsAfterModeratorAction(profileID uint32, moderator string, action string) {
	removed, err := db.ClearRoomReportsForProfile(profileID)
	if err != nil {
		logging.Error("API:Reports", "failed to clear reports after", action, "for", profileID, "err:", err.Error())
		return
	}
	if removed == 0 {
		return
	}
	logging.Event("room_player_reports_reset", map[string]any{
		"profile_id": profileID,
		"removed":    removed,
		"moderator":  moderator,
		"action":     action,
	})
}

func HandlePanelBan(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightBan, "ban users")
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelBanRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	profileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
	if !ok {
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writePanelError(w, http.StatusBadRequest, "reason required")
		return
	}
	if len([]rune(req.Reason)) > 200 {
		writePanelError(w, http.StatusBadRequest, "reason must be 200 characters or less")
		return
	}

	expiresAt, minutes, permanent, ok := panelBanExpiry(w, req)
	if !ok {
		return
	}

	inGameName := onlineNameForProfile(profileID)

	if !db.BanUserUntilWithName(profileID, true, expiresAt, req.Reason, req.ReasonHidden, account.Username, inGameName) {
		writePanelError(w, http.StatusInternalServerError, "failed to ban user")
		return
	}

	gpcm.KickPlayerCustomMessage(profileID, req.Reason, gpcm.WWFCMsgProfileRestrictedCustom)

	logging.Event("profile_banned", map[string]any{
		"profile_id":     profileID,
		"tos_violation":  true,
		"length_minutes": minutes,
		"permanent":      permanent,
		"reason":         req.Reason,
		"reason_hidden":  req.ReasonHidden,
		"moderator":      account.Username,
	})
	clearReportsAfterModeratorAction(profileID, account.Username, "ban")

	writePanelJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func HandlePanelUnban(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightUnban, "unban users")
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelUnbanRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	profileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
	if !ok {
		return
	}

	if !db.UnbanUser(profileID) {
		writePanelError(w, http.StatusInternalServerError, "failed to unban user")
		return
	}

	logging.Event("profile_unbanned", map[string]any{
		"profile_id": profileID,
		"moderator":  account.Username,
	})

	writePanelJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func HandlePanelKick(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightKick, "kick users")
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelKickRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	profileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
	if !ok {
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writePanelError(w, http.StatusBadRequest, "reason required")
		return
	}
	if len([]rune(req.Reason)) > 200 {
		writePanelError(w, http.StatusBadRequest, "reason must be 200 characters or less")
		return
	}

	targetName, targetFriendCode := profileDisplayForPanelAction(profileID)
	if err := db.RecordModeratorKick(profileID, targetName, targetFriendCode, req.Reason, account.Username); err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to record kick")
		return
	}

	gpcm.KickPlayerCustomMessage(profileID, req.Reason, gpcm.WWFCMsgKickedCustom)

	logging.Event("profile_kicked", map[string]any{
		"profile_id": profileID,
		"reason":     req.Reason,
		"moderator":  account.Username,
	})
	clearReportsAfterModeratorAction(profileID, account.Username, "kick")

	writePanelJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func HandlePanelLoginMessage(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightMessage, "message users")
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelLoginMessageRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	profileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
	if !ok {
		return
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		writePanelError(w, http.StatusBadRequest, "message required")
		return
	}
	if len([]rune(message)) > 500 {
		writePanelError(w, http.StatusBadRequest, "message too long")
		return
	}
	if target := gpcm.GetLoginMessageTargetInfo(profileID); target.Online && !target.PayloadFeatures {
		writePanelError(w, http.StatusConflict, "this online player has no payload features; a login message cannot be shown")
		return
	}

	loginMessage, err := db.CreatePanelLoginMessage(profileID, message, account.Username)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to create login message")
		return
	}

	targetInfo, delivered, err := gpcm.DeliverPendingLoginMessage(profileID)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to deliver login message")
		return
	}

	notice := "Queued for the next Mario Kart Wii payload login. It will show once."
	if delivered {
		notice = "Login message sent to the online payload client."
	} else if targetInfo.Online && targetInfo.PayloadFeatures {
		notice = "Queued. It will show once on the next payload login."
	}

	logging.Event("login_message_created", map[string]any{
		"profile_id":       profileID,
		"message_id":       loginMessage.ID,
		"moderator":        account.Username,
		"target_online":    targetInfo.Online,
		"payload_features": targetInfo.PayloadFeatures,
		"delivered_now":    delivered,
	})
	clearReportsAfterModeratorAction(profileID, account.Username, "message")

	writePanelJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  loginMessage,
		"delivery": targetInfo,
		"sent_now": delivered,
		"notice":   notice,
	})
}

func HandlePanelJoinRoom(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightManageRoomsJoin, "join matchmaking rooms")
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelJoinRoomRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	profileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
	if !ok {
		return
	}

	roomID := strings.TrimSpace(req.RoomID)
	if roomID == "" {
		writePanelError(w, http.StatusBadRequest, "room ID required")
		return
	}

	assignment, ok := qr2.ForceJoinRoom(profileID, roomID, 2*time.Minute)
	if !ok {
		writePanelError(w, http.StatusNotFound, "room not found or unavailable")
		return
	}

	logging.Event("profile_forced_room_join", map[string]any{
		"profile_id": profileID,
		"room_id":    assignment.RoomID,
		"game_name":  assignment.GameName,
		"expires":    assignment.ExpiresAt,
		"moderator":  account.Username,
	})

	writePanelJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"pid":     profileID,
		"room_id": assignment.RoomID,
		"game":    assignment.GameName,
		"expires": assignment.ExpiresAt,
	})
}

func HandlePanelCreateRoom(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightManageRoomsCreate, "create matchmaking rooms")
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelCreateRoomRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	profileID, ok := resolvePanelProfileID(w, req.ProfileID, req.Identifier, req.FriendCode)
	if !ok {
		return
	}

	room, ok, message := qr2.CreateMatchmakingRoom(profileID, req.MatchType)
	if !ok {
		writePanelError(w, http.StatusConflict, message)
		return
	}

	logging.Event("matchmaking_room_created", map[string]any{
		"profile_id": profileID,
		"room_id":    room.ID,
		"game_name":  room.GameName,
		"match_type": room.MatchType,
		"status":     room.Status,
		"moderator":  account.Username,
	})

	writePanelJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"room":    room,
	})
}

func HandlePanelDeleteRoom(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightManageRoomsDelete, "delete matchmaking rooms")
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelDeleteRoomRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	roomID := strings.TrimSpace(req.RoomID)
	if roomID == "" {
		writePanelError(w, http.StatusBadRequest, "room ID required")
		return
	}

	deleted, ok := qr2.DeleteMatchmakingRoom(roomID)
	if !ok {
		writePanelError(w, http.StatusNotFound, "room not found")
		return
	}

	reason := "Matchmaking room deleted by moderator"
	for _, profileID := range deleted.PlayerProfileIDs {
		gpcm.KickPlayerCustomMessage(profileID, reason, gpcm.WWFCMsgKickedCustom)
	}

	logging.Event("matchmaking_room_deleted", map[string]any{
		"room_id":    deleted.ID,
		"game_name":  deleted.GameName,
		"players":    deleted.PlayerProfileIDs,
		"moderator":  account.Username,
		"kicked_all": true,
	})

	writePanelJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"room":    deleted,
	})
}

func HandlePanelEndGame(w http.ResponseWriter, r *http.Request) {
	account, ok := requirePanelRight(w, r, panelRightManageRoomsEndGame, "try to end started games")
	if !ok {
		return
	}
	if !allowMethod(w, r, http.MethodPost) {
		return
	}

	req := panelDeleteRoomRequest{}
	if !readPanelJSON(w, r, &req) {
		return
	}

	roomID := strings.TrimSpace(req.RoomID)
	if roomID == "" {
		writePanelError(w, http.StatusBadRequest, "room ID required")
		return
	}

	endMode := strings.TrimSpace(strings.ToLower(req.EndMode))
	forceDisconnect := false
	switch endMode {
	case qr2.MKWRaceEndModeAuto:
	case qr2.MKWRaceEndModeNativeTry:
	case qr2.MKWRaceEndModeForceDisconnect:
		forceDisconnect = true
	case qr2.MKWRaceEndModeTrackingOnly:
	case "":
		if req.ForceDisconnect {
			endMode = qr2.MKWRaceEndModeForceDisconnect
			forceDisconnect = true
		} else {
			endMode = qr2.MKWRaceEndModeAuto
		}
	default:
		writePanelError(w, http.StatusBadRequest, "invalid end mode")
		return
	}

	ended, ok, message := qr2.EndMKWRace(roomID, endMode)
	if !ok {
		status := http.StatusConflict
		if message == "room not found" {
			status = http.StatusNotFound
		}
		writePanelError(w, status, message)
		return
	}

	logging.Event("matchmaking_room_game_end_attempted", map[string]any{
		"room_id":              ended.ID,
		"game_name":            ended.GameName,
		"race_number":          ended.RaceInfo.RaceNumber,
		"course_id":            ended.RaceInfo.CourseID,
		"engine_class_id":      ended.RaceInfo.EngineClassID,
		"in_game_end_attempts": ended.InGameEndAttempts,
		"force_disconnect":     forceDisconnect,
		"native_try":           endMode == qr2.MKWRaceEndModeNativeTry,
		"end_mode":             endMode,
		"server_state_reset":   ended.ServerStateReset,
		"moderator":            account.Username,
	})

	writePanelJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"room":    ended,
	})
}

func setupRequired() (bool, error) {
	hasAccounts, err := db.HasPanelAccounts()
	if err != nil {
		return false, err
	}

	return !hasAccounts, nil
}

func currentPanelAccount(r *http.Request) (database.PanelAccount, bool) {
	cookie, err := r.Cookie(panelCookieName)
	if err != nil || cookie.Value == "" {
		return database.PanelAccount{}, false
	}

	account, exists, err := db.GetPanelSession(hashSessionToken(cookie.Value))
	if err != nil || !exists {
		return database.PanelAccount{}, false
	}

	return account, true
}

func requirePanelAccount(w http.ResponseWriter, r *http.Request) (database.PanelAccount, bool) {
	account, ok := currentPanelAccount(r)
	if !ok {
		writePanelError(w, http.StatusUnauthorized, "login required")
		return database.PanelAccount{}, false
	}

	return account, true
}

func requireLinkedPlayerAccount(w http.ResponseWriter, r *http.Request) (database.PanelAccount, uint32, bool) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return database.PanelAccount{}, 0, false
	}
	if account.Role != "player" {
		writePanelError(w, http.StatusForbidden, "player account required")
		return database.PanelAccount{}, 0, false
	}
	if account.ProfileID == 0 {
		writePanelError(w, http.StatusForbidden, "linked profile required")
		return database.PanelAccount{}, 0, false
	}

	return account, account.ProfileID, true
}

func requireModeratorAccount(w http.ResponseWriter, r *http.Request) (database.PanelAccount, bool) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return database.PanelAccount{}, false
	}
	if account.Role == "player" {
		writePanelError(w, http.StatusForbidden, "moderator account required")
		return database.PanelAccount{}, false
	}

	return account, true
}

func requirePanelRight(w http.ResponseWriter, r *http.Request, right string, label string) (database.PanelAccount, bool) {
	account, ok := requirePanelAccount(w, r)
	if !ok {
		return database.PanelAccount{}, false
	}
	if !panelAccountHasRight(account, right) {
		if label == "" {
			label = right
		}
		writePanelError(w, http.StatusForbidden, label+" right required")
		return database.PanelAccount{}, false
	}

	return account, true
}

func panelAccountHasRight(account database.PanelAccount, right string) bool {
	if account.Role == "owner" {
		return true
	}
	if account.Role == "player" {
		return false
	}
	switch right {
	case panelRightViewUsers:
		return account.Rights.ViewUsers
	case panelRightViewBans:
		return account.Rights.ViewBans
	case panelRightBan:
		return account.Rights.Ban
	case panelRightUnban:
		return account.Rights.Unban
	case panelRightKick:
		return account.Rights.Kick
	case panelRightMessage:
		return account.Rights.Message
	case panelRightManageRoomsCreate:
		return account.Rights.ManageRoomsCreate
	case panelRightManageRoomsJoin:
		return account.Rights.ManageRoomsJoin
	case panelRightManageRoomsDelete:
		return account.Rights.ManageRoomsDelete
	case panelRightManageRoomsEndGame:
		return account.Rights.ManageRoomsEndGame
	case panelRightViewLogs:
		return account.Rights.ViewLogs
	case panelRightViewReports:
		return account.Rights.ViewReports
	case panelRightManageGhosts:
		return account.Rights.ManageGhosts
	case panelRightManageGames:
		return account.Rights.ManageGames
	default:
		return false
	}
}

func panelAccountHasAnyRoomRight(account database.PanelAccount) bool {
	return panelAccountHasRight(account, panelRightManageRoomsCreate) ||
		panelAccountHasRight(account, panelRightManageRoomsJoin) ||
		panelAccountHasRight(account, panelRightManageRoomsDelete) ||
		panelAccountHasRight(account, panelRightManageRoomsEndGame)
}

func panelAccountProfileIDs(account database.PanelAccount) []uint32 {
	seen := map[uint32]bool{}
	out := []uint32{}
	add := func(profileID uint32) {
		if profileID == 0 || seen[profileID] {
			return
		}
		seen[profileID] = true
		out = append(out, profileID)
	}
	add(account.ProfileID)
	for _, profile := range account.LinkedProfiles {
		add(profile.ProfileID)
	}
	return out
}

func startPanelSession(w http.ResponseWriter, account database.PanelAccount) bool {
	token, err := newSessionToken()
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to create session")
		return false
	}

	expiresAt := time.Now().UTC().Add(panelSessionTTL)
	if err := db.CreatePanelSession(hashSessionToken(token), account.ID, expiresAt); err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to store session")
		return false
	}

	http.SetCookie(w, &http.Cookie{
		Name:     panelCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(panelSessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return true
}

func newSessionToken() (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}

	return hex.EncodeToString(token), nil
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newProfileLinkCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashProfileLinkCode(accountID int, profileID uint32, code string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", accountID, profileID, strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}

func readCredentials(w http.ResponseWriter, r *http.Request) (panelCredentialsRequest, bool) {
	req := panelCredentialsRequest{}
	if !readPanelJSON(w, r, &req) {
		return req, false
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		writePanelError(w, http.StatusBadRequest, "username required")
		return req, false
	}
	if len(req.Username) > 64 {
		writePanelError(w, http.StatusBadRequest, "username too long")
		return req, false
	}
	if len(req.Password) < 8 {
		writePanelError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return req, false
	}

	return req, true
}

func resolvePanelProfileID(w http.ResponseWriter, profileID uint32, identifier string, friendCode string) (uint32, bool) {
	if profileID != 0 {
		return profileID, true
	}

	if strings.TrimSpace(identifier) == "" {
		identifier = friendCode
	}

	resolvedProfileID, exists, err := db.ResolveProfileIdentifier(identifier)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to resolve profile")
		return 0, false
	}
	if !exists {
		writePanelError(w, http.StatusBadRequest, "profile ID or friend code required")
		return 0, false
	}

	return resolvedProfileID, true
}

func onlineNameForProfile(profileID uint32) string {
	for _, user := range gpcm.GetOnlineUsers() {
		if user.ProfileID == profileID {
			return user.Name
		}
	}

	return ""
}

func panelBanExpiry(w http.ResponseWriter, req panelBanRequest) (*time.Time, uint64, bool, bool) {
	minutes := uint64(0)
	permanent := false

	switch strings.TrimSpace(strings.ToLower(req.Duration)) {
	case "":
		minutes = req.Days*24*60 + req.Hours*60 + req.Minutes
	case "3m":
		minutes = 3
	case "1h":
		minutes = 60
	case "1d":
		minutes = 24 * 60
	case "3d":
		minutes = 3 * 24 * 60
	case "1w":
		minutes = 7 * 24 * 60
	case "4w":
		minutes = 4 * 7 * 24 * 60
	case "6mo":
		minutes = 180 * 24 * 60
	case "permanent":
		permanent = true
	default:
		writePanelError(w, http.StatusBadRequest, "invalid duration")
		return nil, 0, false, false
	}

	if permanent {
		return nil, 0, true, true
	}
	if minutes == 0 {
		writePanelError(w, http.StatusBadRequest, "duration required")
		return nil, 0, false, false
	}

	expiresAt := time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
	return &expiresAt, minutes, false, true
}

func publicOnlineUsers() []gpcm.OnlineUserInfo {
	users := gpcm.GetOnlineUsers()
	for i := range users {
		users[i].ProfileID = 0
		users[i].RemoteAddr = ""
		users[i].Restricted = false
		users[i].Status = ""
		users[i].PayloadFeatures = false
	}

	return users
}

func writeBans(w http.ResponseWriter, includePrivate bool) {
	bans, err := db.ListBans(includePrivate)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to list bans")
		return
	}

	enrichBansWithOnlineUsers(bans)
	if !includePrivate {
		for i := range bans {
			bans[i].ProfileID = 0
			bans[i].TOS = false
		}
	}

	writePanelJSON(w, http.StatusOK, bans)
}

func enrichBansWithOnlineUsers(bans []database.BanRecord) {
	if len(bans) == 0 {
		return
	}

	onlineByProfileID := map[uint32]gpcm.OnlineUserInfo{}
	for _, user := range gpcm.GetOnlineUsers() {
		onlineByProfileID[user.ProfileID] = user
	}

	for i := range bans {
		user, ok := onlineByProfileID[bans[i].ProfileID]
		if !ok {
			continue
		}
		if bans[i].InGameName == "" {
			bans[i].InGameName = user.Name
		}
		if bans[i].FriendCode == "" {
			bans[i].FriendCode = user.FriendCode
		}
	}
}

func writeRooms(w http.ResponseWriter, includePrivate bool) {
	rooms := qr2.GetRooms(true)
	if rooms == nil {
		rooms = []qr2.RoomInfo{}
	}
	if !includePrivate {
		for i := range rooms {
			rooms[i].HostProfileID = ""
			rooms[i].RaceEnd = nil
			for joinIndex, player := range rooms[i].Players {
				player.ProfileID = ""
				rooms[i].Players[joinIndex] = player
			}
		}
	}

	writePanelJSON(w, http.StatusOK, rooms)
}

func writePublicRooms(w http.ResponseWriter, ownProfileIDs []uint32) {
	rooms := qr2.GetRooms(true)
	if rooms == nil {
		rooms = []qr2.RoomInfo{}
	}
	ownProfileIDSet := map[string]bool{}
	for _, profileID := range ownProfileIDs {
		if profileID != 0 {
			ownProfileIDSet[strconv.FormatUint(uint64(profileID), 10)] = true
		}
	}
	for i := range rooms {
		rooms[i].RaceEnd = nil
		if publicRoomContainsAnyProfile(rooms[i], ownProfileIDSet) {
			continue
		}
		rooms[i].HostProfileID = ""
		for joinIndex, player := range rooms[i].Players {
			player.ProfileID = ""
			rooms[i].Players[joinIndex] = player
		}
	}

	writePanelJSON(w, http.StatusOK, rooms)
}

func publicRoomContainsAnyProfile(room qr2.RoomInfo, profileIDs map[string]bool) bool {
	if len(profileIDs) == 0 {
		return false
	}
	if profileIDs[room.HostProfileID] {
		return true
	}
	for _, player := range room.Players {
		if profileIDs[strings.TrimSpace(player.ProfileID)] {
			return true
		}
	}
	return false
}

func ownedRooms(profileID uint32) []qr2.RoomInfo {
	allRooms := qr2.GetRooms(true)
	if allRooms == nil {
		return []qr2.RoomInfo{}
	}

	profileIDString := strconv.FormatUint(uint64(profileID), 10)
	rooms := []qr2.RoomInfo{}
	for _, room := range allRooms {
		if room.HostProfileID == profileIDString {
			rooms = append(rooms, room)
		}
	}

	return rooms
}

func ownedRoomPlayer(ownerProfileID uint32, roomID string, targetProfileID uint32) (qr2.RoomInfo, qr2.PlayerInfo, bool) {
	for _, room := range ownedRooms(ownerProfileID) {
		if room.ID != roomID {
			continue
		}
		if player, ok := roomPlayerByProfileID(room, targetProfileID); ok {
			return room, player, true
		}
	}

	return qr2.RoomInfo{}, qr2.PlayerInfo{}, false
}

func roomWithReporterAndTarget(reporterProfileIDs []uint32, roomID string, targetProfileID uint32) (qr2.RoomInfo, uint32, qr2.PlayerInfo, qr2.PlayerInfo, bool) {
	if roomID == "" || targetProfileID == 0 || len(reporterProfileIDs) == 0 {
		return qr2.RoomInfo{}, 0, qr2.PlayerInfo{}, qr2.PlayerInfo{}, false
	}
	reporterSet := map[uint32]bool{}
	for _, profileID := range reporterProfileIDs {
		if profileID != 0 {
			reporterSet[profileID] = true
		}
	}
	for _, room := range qr2.GetRooms(true) {
		if room.ID != roomID {
			continue
		}
		targetPlayer, targetFound := roomPlayerByProfileID(room, targetProfileID)
		if !targetFound {
			return qr2.RoomInfo{}, 0, qr2.PlayerInfo{}, qr2.PlayerInfo{}, false
		}
		for reporterProfileID := range reporterSet {
			if reporterPlayer, reporterFound := roomPlayerByProfileID(room, reporterProfileID); reporterFound {
				return room, reporterProfileID, reporterPlayer, targetPlayer, true
			}
		}
		return qr2.RoomInfo{}, 0, qr2.PlayerInfo{}, qr2.PlayerInfo{}, false
	}
	return qr2.RoomInfo{}, 0, qr2.PlayerInfo{}, qr2.PlayerInfo{}, false
}

func activeRoomForProfile(profileID uint32) (qr2.RoomInfo, qr2.PlayerInfo, bool) {
	if profileID == 0 {
		return qr2.RoomInfo{}, qr2.PlayerInfo{}, false
	}
	for _, room := range qr2.GetRooms(true) {
		if player, ok := roomPlayerByProfileID(room, profileID); ok {
			return room, player, true
		}
	}
	return qr2.RoomInfo{}, qr2.PlayerInfo{}, false
}

func roomPlayerByProfileID(room qr2.RoomInfo, profileID uint32) (qr2.PlayerInfo, bool) {
	if profileID == 0 {
		return qr2.PlayerInfo{}, false
	}
	targetProfileIDString := strconv.FormatUint(uint64(profileID), 10)
	for _, player := range room.Players {
		if strings.TrimSpace(player.ProfileID) == targetProfileIDString {
			return player, true
		}
	}
	return qr2.PlayerInfo{}, false
}

func panelAccountProfileDisplay(account database.PanelAccount, profileID uint32) (string, string) {
	if profileID == account.ProfileID {
		return account.InGameName, account.FriendCode
	}
	for _, profile := range account.LinkedProfiles {
		if profile.ProfileID == profileID {
			return profile.InGameName, profile.FriendCode
		}
	}
	return "", ""
}

func panelReportType(account database.PanelAccount) string {
	switch strings.TrimSpace(strings.ToLower(account.Role)) {
	case "owner", "moderator":
		return "moderator"
	default:
		return "user"
	}
}

func profileDisplayForPanelAction(profileID uint32) (string, string) {
	name := onlineNameForProfile(profileID)
	friendCode := ""
	if user, found := db.GetProfile(profileID); found {
		if name == "" {
			name = user.LastInGameSn
		}
		if len(user.GsbrCode) >= 4 {
			friendCode = common.CalcFriendCodeString(profileID, user.GsbrCode[:4])
		}
	}
	return name, friendCode
}

func allowMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}

	w.Header().Set("Allow", method)
	writePanelError(w, http.StatusMethodNotAllowed, "method not allowed")
	return false
}

func readPanelJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writePanelError(w, http.StatusBadRequest, "request body too large")
		return false
	}
	defer func() {
		_ = r.Body.Close()
	}()

	if len(body) == 0 {
		writePanelError(w, http.StatusBadRequest, "request body required")
		return false
	}

	if err := json.Unmarshal(body, dst); err != nil {
		writePanelError(w, http.StatusBadRequest, "invalid JSON")
		return false
	}

	return true
}

func writePanelJSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		writePanelError(w, http.StatusInternalServerError, "failed to encode response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writePanelError(w http.ResponseWriter, status int, message string) {
	if message == "" {
		message = http.StatusText(status)
	}

	data, err := json.Marshal(map[string]string{"error": message})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
