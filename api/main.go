package api

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/database"
	"net/http"
)

var (
	db database.Connection

	apiSecret string
)

func StartServer(reload bool) {
	// Get config
	config := common.GetConfig()

	apiSecret = config.APISecret

	// Start SQL
	db = database.Start(config)
	if err := db.UpdatePanelTables(); err != nil {
		panic(err)
	}
	loadDisabledGames()
	loadProfiles()

	db.RegisterEvents(config, []string{
		"profile_kicked",
		"profile_banned",
		"profile_unbanned",
		"login_message_created",
		"profile_forced_room_join",
		"matchmaking_room_created",
		"matchmaking_room_deleted",
		"matchmaking_room_game_end_attempted",
		"matchmaking_room_game_end_result",
		"matchmaking_room_game_end_fallback",
		"profile_link_requested",
		"profile_linked",
		"profile_unlinked",
		"room_player_kicked",
		"room_player_reported",
		"panel_mkw_ranking_deleted",
		"panel_mkw_profile_cleared",
		"panel_mkw_ghost_downloaded",
		"disabled_game_added",
		"disabled_game_updated",
		"disabled_game_removed",
		"server_update_scheduled",
		"server_update_installing",
		"server_updated",
		"server_settings_changed",
		"server_restart_requested",
	})
	startPanelEventStream()

	// After RegisterEvents, so the "server updated" event reaches the database.
	startUpdater()
}

func Shutdown() {
	db.Close()
}

func RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/panel", HandlePanel)
	mux.HandleFunc("/panel/", HandlePanel)
	mux.HandleFunc("/settings", HandleSettings)
	mux.HandleFunc("/settings/", HandleSettings)
	mux.HandleFunc("/errors", HandleErrorsPage)
	mux.HandleFunc("/errors/", HandleErrorsPage)
	mux.HandleFunc("/login", HandleLogin)
	mux.HandleFunc("/login/", HandleLogin)
	mux.HandleFunc("/assets/", HandleAssets)
	mux.HandleFunc("/u/", HandleProfilePage)

	mux.HandleFunc("/api/public/errors", HandlePublicErrors)
	mux.HandleFunc("/api/public/bans", HandlePublicBans)
	mux.HandleFunc("/api/public/rooms", HandlePublicRooms)
	mux.HandleFunc("/api/public/users", HandlePublicUsers)
	mux.HandleFunc("/api/public/profiles/", HandlePublicProfile)
	mux.HandleFunc("/api/public/avatars/", HandlePublicAvatar)
	mux.HandleFunc("/api/public/profile-links", HandlePublicProfileLinks)
	mux.HandleFunc("/api/panel/session", HandlePanelSession)
	mux.HandleFunc("/api/panel/setup", HandlePanelSetup)
	mux.HandleFunc("/api/panel/login", HandlePanelLogin)
	mux.HandleFunc("/api/panel/register", HandlePanelRegister)
	mux.HandleFunc("/api/panel/logout", HandlePanelLogout)
	mux.HandleFunc("/api/panel/accounts", HandlePanelAccounts)
	mux.HandleFunc("/api/panel/users", HandlePanelUsers)
	mux.HandleFunc("/api/panel/bans", HandlePanelBans)
	mux.HandleFunc("/api/panel/rooms", HandlePanelRooms)
	mux.HandleFunc("/api/panel/matches", HandlePanelMatches)
	mux.HandleFunc("/api/panel/logs", HandlePanelLogs)
	mux.HandleFunc("/api/panel/events", HandlePanelEvents)
	mux.HandleFunc("/api/panel/link/start", HandlePanelProfileLinkStart)
	mux.HandleFunc("/api/panel/link/complete", HandlePanelProfileLinkComplete)
	mux.HandleFunc("/api/panel/link/unlink", HandlePanelProfileUnlink)
	mux.HandleFunc("/api/panel/console/me", HandlePanelConsoleSummary)
	mux.HandleFunc("/api/panel/console/primary", HandlePanelConsolePrimary)
	mux.HandleFunc("/api/panel/my/rooms", HandlePanelMyRooms)
	mux.HandleFunc("/api/panel/my/kick", HandlePanelMyRoomKick)
	mux.HandleFunc("/api/panel/my/report", HandlePanelMyRoomReport)
	mux.HandleFunc("/api/panel/my/open-host", HandlePanelMyOpenHost)
	mux.HandleFunc("/api/panel/my/profile", HandlePanelMyProfile)
	mux.HandleFunc("/api/panel/my/profile/avatar", HandlePanelMyProfileAvatar)
	mux.HandleFunc("/api/panel/profile-moderation", HandlePanelProfileModeration)
	mux.HandleFunc("/api/panel/reports", HandlePanelReports)
	mux.HandleFunc("/api/panel/reports/details", HandlePanelReportDetails)
	mux.HandleFunc("/api/panel/misc-rights", HandlePanelMiscRights)
	mux.HandleFunc("/api/panel/ban", HandlePanelBan)
	mux.HandleFunc("/api/panel/unban", HandlePanelUnban)
	mux.HandleFunc("/api/panel/kick", HandlePanelKick)
	mux.HandleFunc("/api/panel/login-message", HandlePanelLoginMessage)
	mux.HandleFunc("/api/panel/join-room", HandlePanelJoinRoom)
	mux.HandleFunc("/api/panel/create-room", HandlePanelCreateRoom)
	mux.HandleFunc("/api/panel/delete-room", HandlePanelDeleteRoom)
	mux.HandleFunc("/api/panel/end-game", HandlePanelEndGame)
	mux.HandleFunc("/api/panel/mkw/top10", HandlePanelMKWTop10)
	mux.HandleFunc("/api/panel/mkw/search", HandlePanelMKWSearch)
	mux.HandleFunc("/api/panel/mkw/ghost", HandlePanelMKWGhostDownload)
	mux.HandleFunc("/api/panel/mkw/delete-ranking", HandlePanelMKWDeleteRanking)
	mux.HandleFunc("/api/panel/mkw/clear-profile", HandlePanelMKWClearProfile)
	mux.HandleFunc("/api/panel/disabled-games", HandlePanelDisabledGames)
	mux.HandleFunc("/api/panel/game-list", HandlePanelGameList)
	mux.HandleFunc("/api/panel/update", HandlePanelUpdate)
	mux.HandleFunc("/api/panel/server-settings", HandlePanelServerSettings)
	mux.HandleFunc("/api/panel/https-certificates", HandlePanelHTTPSCertificates)
	mux.HandleFunc("/api/panel/server-restart", HandlePanelServerRestart)
	mux.HandleFunc("/api/panel/server-console", HandlePanelServerConsole)
	mux.HandleFunc("/api/panel/docs", HandlePanelDocs)
	mux.HandleFunc("/api/panel/server-files", HandlePanelServerFiles)
	mux.HandleFunc("/api/panel/server-files/download", HandlePanelServerFileDownload)
	mux.HandleFunc("/api/panel/server-files/content", HandlePanelServerFileContent)
	mux.HandleFunc("/api/panel/server-files/upload", HandlePanelServerFileUpload)
	mux.HandleFunc("/api/panel/server-files/mkdir", HandlePanelServerFolderCreate)
	mux.HandleFunc("/api/panel/server-files/open", HandlePanelServerFolderOpen)

	mux.HandleFunc("/api/groups", HandleGroups)
	mux.HandleFunc("/api/stats", HandleStats)
	mux.HandleFunc("/api/ban", HandleBan)
	mux.HandleFunc("/api/unban", HandleUnban)
	mux.HandleFunc("/api/kick", HandleKick)
}
