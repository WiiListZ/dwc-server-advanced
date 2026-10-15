package database

import (
	"dwc-server-advanced/common"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
)

type PanelAccount struct {
	ID             int                 `json:"id"`
	Username       string              `json:"username"`
	Role           string              `json:"role"`
	CreatedAt      time.Time           `json:"created"`
	ProfileID      uint32              `json:"pid,omitempty"`
	FriendCode     string              `json:"fc,omitempty"`
	InGameName     string              `json:"name,omitempty"`
	LinkedAt       *time.Time          `json:"linked,omitempty"`
	LinkedProfiles []ConsoleProfileRow `json:"linked_profiles,omitempty"`
	Rights         PanelRights         `json:"rights"`
}

type PanelRights struct {
	ViewUsers          bool     `json:"view_users"`
	ViewBans           bool     `json:"view_bans"`
	Ban                bool     `json:"ban"`
	Unban              bool     `json:"unban"`
	Kick               bool     `json:"kick"`
	Message            bool     `json:"message"`
	LegacyManageRooms  bool     `json:"manage_rooms,omitempty"`
	ManageRoomsCreate  bool     `json:"manage_rooms_create"`
	ManageRoomsJoin    bool     `json:"manage_rooms_join"`
	ManageRoomsDelete  bool     `json:"manage_rooms_delete"`
	ManageRoomsEndGame bool     `json:"manage_rooms_end_game"`
	ViewLogs           bool     `json:"view_logs"`
	ViewReports        bool     `json:"view_reports"`
	ManageGhosts       bool     `json:"manage_ghosts"`
	ManageGames        bool     `json:"manage_games"`
	ManageRights       bool     `json:"manage_rights"`
	PrivateRoomKick    bool     `json:"private_room_kick"`
	ReportPlayers      bool     `json:"report_players"`
	OpenHost           bool     `json:"open_host"`
	LogTypes           []string `json:"log_types"`
}

var PanelLogTypes = []string{
	"ban", "unban", "kick", "message", "match",
	"security", "network", "ghost", "session", "panel", "error",
}

type BanRecord struct {
	Kind          string     `json:"kind,omitempty"`
	ProfileID     uint32     `json:"pid,omitempty"`
	FriendCode    string     `json:"fc,omitempty"`
	InGameName    string     `json:"name,omitempty"`
	Status        string     `json:"status,omitempty"`
	Reason        string     `json:"reason,omitempty"`
	ReasonHidden  string     `json:"reason_hidden,omitempty"`
	Moderator     string     `json:"moderator,omitempty"`
	TOS           bool       `json:"tos,omitempty"`
	Issued        *time.Time `json:"issued,omitempty"`
	Expires       *time.Time `json:"expires,omitempty"`
	Revoked       *time.Time `json:"revoked,omitempty"`
	RoomID        string     `json:"room_id,omitempty"`
	ConsoleID     int        `json:"console_id,omitempty"`
	NgDeviceID    uint32     `json:"-"`
	NgDeviceIDHex string     `json:"-"`
}

type PanelLoginMessage struct {
	ID        int       `json:"id"`
	ProfileID uint32    `json:"pid"`
	Message   string    `json:"message"`
	Moderator string    `json:"moderator"`
	CreatedAt time.Time `json:"created"`
}

type PanelMatchLogPlayer struct {
	JoinIndex    string `json:"slot,omitempty"`
	ProfileID    uint32 `json:"pid,omitempty"`
	FriendCode   string `json:"fc,omitempty"`
	InGameName   string `json:"name,omitempty"`
	LocalPlayers string `json:"count,omitempty"`
	ConnFail     string `json:"conn_fail,omitempty"`
	VersusELO    string `json:"ev,omitempty"`
	BattleELO    string `json:"eb,omitempty"`
}

type PanelMatchLog struct {
	ID            int                   `json:"id"`
	SourceKey     string                `json:"-"`
	RoomID        string                `json:"room_id"`
	DWCGroupID    uint64                `json:"group_id,omitempty"`
	GameName      string                `json:"game"`
	MatchType     string                `json:"type"`
	MKWRegion     string                `json:"rk,omitempty"`
	RaceNumber    int                   `json:"race,omitempty"`
	CourseID      int                   `json:"course"`
	EngineClassID int                   `json:"cc"`
	HostProfileID uint32                `json:"host_pid,omitempty"`
	HostName      string                `json:"host_name,omitempty"`
	PlayerCount   int                   `json:"player_count"`
	Players       []PanelMatchLogPlayer `json:"players"`
	CreatedAt     time.Time             `json:"created"`
}

type PanelEventLog struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`
	Level      string     `json:"level"`
	CreatedAt  time.Time  `json:"created"`
	Summary    string     `json:"summary"`
	Moderator  string     `json:"moderator,omitempty"`
	TargetPID  uint32     `json:"target_pid,omitempty"`
	TargetName string     `json:"target_name,omitempty"`
	TargetFC   string     `json:"target_fc,omitempty"`
	RoomID     string     `json:"room_id,omitempty"`
	Reason     string     `json:"reason,omitempty"`
	Note       string     `json:"note,omitempty"`
	Expires    *time.Time `json:"expires,omitempty"`
	TOS        bool       `json:"tos,omitempty"`
	Extra      string     `json:"extra,omitempty"`
}

type PanelRoomReportSummary struct {
	ProfileID            uint32    `json:"pid"`
	FriendCode           string    `json:"fc,omitempty"`
	InGameName           string    `json:"name,omitempty"`
	ReportCount          int       `json:"reports"`
	ModeratorReportCount int       `json:"moderator_reports"`
	UserReportCount      int       `json:"user_reports"`
	LastReported         time.Time `json:"last_reported"`
}

type PanelRoomReportDetail struct {
	ID                 int       `json:"id"`
	RoomID             string    `json:"room_id"`
	GameName           string    `json:"game,omitempty"`
	RoomType           string    `json:"type,omitempty"`
	ReporterProfileID  uint32    `json:"reporter_pid"`
	ReporterFriendCode string    `json:"reporter_fc,omitempty"`
	ReporterName       string    `json:"reporter_name,omitempty"`
	ReporterType       string    `json:"reporter_type"`
	TargetProfileID    uint32    `json:"target_pid"`
	TargetFriendCode   string    `json:"target_fc,omitempty"`
	TargetName         string    `json:"target_name,omitempty"`
	Reason             string    `json:"reason"`
	CreatedAt          time.Time `json:"created"`
}

var ErrDuplicateRoomReport = errors.New("room report already exists")

func DefaultPanelRights(role string) PanelRights {
	rights := PanelRights{}
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "owner":
		rights.ViewUsers = true
		rights.ViewBans = true
		rights.Ban = true
		rights.Unban = true
		rights.Kick = true
		rights.Message = true
		rights.ManageRoomsCreate = true
		rights.ManageRoomsJoin = true
		rights.ManageRoomsDelete = true
		rights.ManageRoomsEndGame = true
		rights.ViewLogs = true
		rights.ViewReports = true
		rights.ManageGhosts = true
		rights.ManageGames = true
		rights.ManageRights = true
		rights.ReportPlayers = true
		rights.OpenHost = true
		rights.LogTypes = append([]string(nil), PanelLogTypes...)
	case "moderator":
		rights.ViewUsers = true
		rights.ViewBans = true
		rights.Ban = true
		rights.Unban = true
		rights.Kick = true
		rights.Message = true
		rights.ManageRoomsCreate = true
		rights.ManageRoomsJoin = true
		rights.ManageRoomsDelete = true
		rights.ManageRoomsEndGame = true
		rights.ViewLogs = true
		rights.ViewReports = true
		rights.ManageGhosts = true
		rights.OpenHost = true
		rights.LogTypes = append([]string(nil), PanelLogTypes...)
	case "player":
		rights.PrivateRoomKick = true
		rights.ReportPlayers = true
	}
	return rights
}

func sanitizeLogTypes(types []string) []string {
	if len(types) == 0 {
		return []string{}
	}
	allowed := map[string]bool{}
	for _, key := range PanelLogTypes {
		allowed[key] = true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(types))
	for _, t := range types {
		key := strings.TrimSpace(strings.ToLower(t))
		if !allowed[key] || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}

func encodeLogTypes(types []string) string {
	clean := sanitizeLogTypes(types)
	return strings.Join(clean, ",")
}

func decodeLogTypes(stored string) []string {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return []string{}
	}
	parts := strings.Split(stored, ",")
	return sanitizeLogTypes(parts)
}

func NormalizePanelRights(account *PanelAccount) {
	if account == nil {
		return
	}
	switch account.Role {
	case "owner":
		account.Rights = DefaultPanelRights("owner")
	case "player":
		account.Rights.ViewUsers = false
		account.Rights.ViewBans = false
		account.Rights.Ban = false
		account.Rights.Unban = false
		account.Rights.Kick = false
		account.Rights.Message = false
		account.Rights.LegacyManageRooms = false
		account.Rights.ManageRoomsCreate = false
		account.Rights.ManageRoomsJoin = false
		account.Rights.ManageRoomsDelete = false
		account.Rights.ManageRoomsEndGame = false
		account.Rights.ViewLogs = false
		account.Rights.ViewReports = false
		account.Rights.ManageGhosts = false
		account.Rights.ManageGames = false
		account.Rights.ManageRights = false
		account.Rights.PrivateRoomKick = true
		// OpenHost and ReportPlayers are retained as-is for player role.
		account.Rights.LogTypes = []string{}
	default:
		account.Role = "moderator"
		account.Rights.ApplyLegacyRoomRights()
		account.Rights.ManageRights = false
		account.Rights.PrivateRoomKick = false
		account.Rights.ReportPlayers = false
		account.Rights.LogTypes = sanitizeLogTypes(account.Rights.LogTypes)
		if !account.Rights.ViewLogs {
			account.Rights.LogTypes = []string{}
		}
	}
}

func (r *PanelRights) ApplyLegacyRoomRights() {
	if r == nil {
		return
	}
	if r.LegacyManageRooms && !r.HasRoomRights() {
		r.ManageRoomsCreate = true
		r.ManageRoomsJoin = true
		r.ManageRoomsDelete = true
		r.ManageRoomsEndGame = true
	}
	r.LegacyManageRooms = false
}

func (r PanelRights) HasRoomRights() bool {
	return r.ManageRoomsCreate || r.ManageRoomsJoin || r.ManageRoomsDelete || r.ManageRoomsEndGame
}

func (r PanelRights) HasModerationRights() bool {
	return r.ViewUsers || r.ViewBans || r.Ban || r.Unban || r.Kick || r.Message || r.HasRoomRights() || r.ViewLogs || r.ViewReports || r.ManageGhosts || r.ManageGames
}

func scanPanelRights(rows interface {
	Scan(dest ...interface{}) error
}, account *PanelAccount, profileID *pgtype.Int8, linkedAt *pgtype.Timestamp, gsbrcd *string) error {
	var logTypes string
	err := rows.Scan(
		&account.ID,
		&account.Username,
		&account.Role,
		&account.CreatedAt,
		profileID,
		linkedAt,
		gsbrcd,
		&account.InGameName,
		&account.Rights.ViewUsers,
		&account.Rights.ViewBans,
		&account.Rights.Ban,
		&account.Rights.Unban,
		&account.Rights.Kick,
		&account.Rights.Message,
		&account.Rights.LegacyManageRooms,
		&account.Rights.ManageRoomsCreate,
		&account.Rights.ManageRoomsJoin,
		&account.Rights.ManageRoomsDelete,
		&account.Rights.ManageRoomsEndGame,
		&account.Rights.ViewLogs,
		&account.Rights.ViewReports,
		&account.Rights.ManageGhosts,
		&account.Rights.ManageGames,
		&account.Rights.ReportPlayers,
		&account.Rights.OpenHost,
		&logTypes,
	)
	if err == nil {
		account.Rights.LogTypes = decodeLogTypes(logTypes)
		NormalizePanelRights(account)
	}
	return err
}

func (c *Connection) UpdatePanelTables() error {
	_, err := c.pool.Exec(c.ctx, `
		ALTER TABLE IF EXISTS public.users
			ADD IF NOT EXISTS ban_revoked timestamp without time zone,
			ADD IF NOT EXISTS open_host_list_display text[] DEFAULT ARRAY[]::text[];

		CREATE TABLE IF NOT EXISTS public.events (
			id serial PRIMARY KEY,
			event_type character varying NOT NULL,
			event_data jsonb NOT NULL,
			event_time timestamp without time zone DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS events_event_time_idx
			ON public.events (event_time);

		CREATE TABLE IF NOT EXISTS public.panel_ban_history (
			id serial PRIMARY KEY,
			profile_id bigint NOT NULL,
			gsbrcd character varying NOT NULL DEFAULT ''::character varying,
			ingame_name character varying NOT NULL DEFAULT ''::character varying,
			reason character varying,
			reason_hidden character varying,
			moderator character varying,
			tos boolean,
			issued timestamp without time zone NOT NULL,
			expires timestamp without time zone,
			revoked timestamp without time zone
		);

		CREATE INDEX IF NOT EXISTS panel_ban_history_profile_id_idx
			ON public.panel_ban_history (profile_id);

		CREATE INDEX IF NOT EXISTS panel_ban_history_visible_idx
			ON public.panel_ban_history (issued, expires, revoked);

		INSERT INTO public.panel_ban_history (
			profile_id, gsbrcd, ingame_name, reason, reason_hidden, moderator, tos, issued, expires, revoked
		)
		SELECT u.profile_id,
		       COALESCE(u.gsbrcd, ''),
		       COALESCE(u.last_ingamesn, ''),
		       u.ban_reason,
		       u.ban_reason_hidden,
		       u.ban_moderator,
		       u.ban_tos,
		       u.ban_issued,
		       u.ban_expires,
		       u.ban_revoked
		FROM public.users u
		WHERE u.ban_issued IS NOT NULL
		  AND (
		       (u.has_ban = true AND (u.ban_expires IS NULL OR u.ban_expires > CURRENT_TIMESTAMP - INTERVAL '1 month'))
		    OR (u.has_ban = false AND u.ban_revoked IS NOT NULL AND u.ban_revoked > CURRENT_TIMESTAMP - INTERVAL '1 month')
		  )
		  AND NOT EXISTS (
		    SELECT 1
		    FROM public.panel_ban_history h
		    WHERE h.profile_id = u.profile_id
		      AND h.issued = u.ban_issued
		  );

		CREATE TABLE IF NOT EXISTS public.panel_accounts (
			id serial PRIMARY KEY,
			username character varying NOT NULL UNIQUE,
			password_hash bytea NOT NULL,
			role character varying NOT NULL,
			created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
		);

		ALTER TABLE IF EXISTS public.panel_accounts
			ADD IF NOT EXISTS profile_id bigint,
			ADD IF NOT EXISTS linked_at timestamp without time zone,
			ADD IF NOT EXISTS can_view_users boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_view_bans boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_ban boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_unban boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_kick boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_message boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_manage_rooms boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_manage_rooms_create boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_manage_rooms_join boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_manage_rooms_delete boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_view_logs boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_view_reports boolean NOT NULL DEFAULT true,
			ADD IF NOT EXISTS can_manage_ghosts boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_manage_games boolean NOT NULL DEFAULT false,
			ADD IF NOT EXISTS can_report_players boolean NOT NULL DEFAULT true,
			ADD IF NOT EXISTS right_open_host boolean NOT NULL DEFAULT true,
			ADD IF NOT EXISTS log_types text NOT NULL DEFAULT '',
			ADD IF NOT EXISTS rights_initialized boolean NOT NULL DEFAULT false;

		DO $roomendgame$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = 'panel_accounts'
				  AND column_name = 'can_manage_rooms_end_game'
			) THEN
				ALTER TABLE public.panel_accounts
					ADD COLUMN can_manage_rooms_end_game boolean NOT NULL DEFAULT false;

				UPDATE public.panel_accounts
				SET can_manage_rooms_end_game = true
				WHERE role IN ('owner', 'moderator')
				  AND (
				       can_manage_rooms = true
				    OR can_manage_rooms_create = true
				    OR can_manage_rooms_join = true
				    OR can_manage_rooms_delete = true
				  );
			END IF;
		END
		$roomendgame$;

		UPDATE public.panel_accounts
		SET can_view_users = true,
		    can_view_bans = true,
		    can_ban = true,
		    can_unban = true,
		    can_kick = true,
		    can_message = true,
		    can_manage_rooms = true,
		    can_manage_rooms_create = true,
		    can_manage_rooms_join = true,
		    can_manage_rooms_delete = true,
		    can_manage_rooms_end_game = true,
		    can_view_logs = true,
		    can_view_reports = true,
		    can_manage_ghosts = true,
		    can_report_players = true,
		    log_types = 'ban,unban,kick,message,match,security,network,ghost,session,panel,error',
		    rights_initialized = true
		WHERE role IN ('owner', 'moderator')
		  AND rights_initialized = false;

		UPDATE public.panel_accounts
		SET can_manage_rooms_create = true,
		    can_manage_rooms_join = true,
		    can_manage_rooms_delete = true,
		    can_manage_rooms_end_game = true
		WHERE can_manage_rooms = true
		  AND can_manage_rooms_create = false
		  AND can_manage_rooms_join = false
		  AND can_manage_rooms_delete = false
		  AND can_manage_rooms_end_game = false;

		UPDATE public.panel_accounts
		SET log_types = 'ban,unban,kick,message,match,security,network,ghost,session,panel,error'
		WHERE role IN ('owner', 'moderator')
		  AND can_view_logs = true
		  AND COALESCE(log_types, '') = '';

		UPDATE public.panel_accounts
		SET can_view_users = false,
		    can_view_bans = false,
		    can_ban = false,
		    can_unban = false,
		    can_kick = false,
		    can_message = false,
		    can_manage_rooms = false,
		    can_manage_rooms_create = false,
		    can_manage_rooms_join = false,
		    can_manage_rooms_delete = false,
		    can_manage_rooms_end_game = false,
		    can_view_logs = false,
		    can_view_reports = false,
		    can_manage_ghosts = false,
		    can_report_players = true,
		    rights_initialized = true
		WHERE role = 'player'
		  AND rights_initialized = false;

		ALTER TABLE IF EXISTS public.panel_accounts
			DROP CONSTRAINT IF EXISTS panel_accounts_role_check;

		ALTER TABLE IF EXISTS public.panel_accounts
			ADD CONSTRAINT panel_accounts_role_check
			CHECK (role IN ('owner', 'moderator', 'player'));

		CREATE UNIQUE INDEX IF NOT EXISTS panel_accounts_profile_id_unique_idx
			ON public.panel_accounts (profile_id)
			WHERE profile_id IS NOT NULL;

		CREATE TABLE IF NOT EXISTS public.panel_sessions (
			token_hash character varying(64) PRIMARY KEY,
			account_id integer NOT NULL REFERENCES public.panel_accounts(id) ON DELETE CASCADE,
			created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
			expires_at timestamp without time zone NOT NULL
		);

		CREATE TABLE IF NOT EXISTS public.panel_login_messages (
			id serial PRIMARY KEY,
			profile_id bigint NOT NULL,
			message text NOT NULL,
			moderator character varying,
			created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
			delivered_at timestamp without time zone
		);

		CREATE INDEX IF NOT EXISTS panel_login_messages_profile_pending_idx
			ON public.panel_login_messages (profile_id, delivered_at, created_at);

		CREATE TABLE IF NOT EXISTS public.panel_profile_link_codes (
			id serial PRIMARY KEY,
			account_id integer NOT NULL REFERENCES public.panel_accounts(id) ON DELETE CASCADE,
			profile_id bigint NOT NULL,
			code_hash character varying(64) NOT NULL,
			created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
			expires_at timestamp without time zone NOT NULL,
			consumed_at timestamp without time zone
		);

		CREATE INDEX IF NOT EXISTS panel_profile_link_codes_pending_idx
			ON public.panel_profile_link_codes (account_id, profile_id, expires_at, consumed_at);

		CREATE TABLE IF NOT EXISTS public.panel_room_kick_history (
			id serial PRIMARY KEY,
			room_id character varying NOT NULL,
			game_name character varying NOT NULL DEFAULT ''::character varying,
			owner_profile_id bigint NOT NULL,
			target_profile_id bigint NOT NULL,
			target_ingame_name character varying NOT NULL DEFAULT ''::character varying,
			target_friend_code character varying NOT NULL DEFAULT ''::character varying,
			reason character varying NOT NULL,
			moderator character varying,
			created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS panel_room_kick_history_created_idx
			ON public.panel_room_kick_history (created_at);

		CREATE TABLE IF NOT EXISTS public.panel_kick_history (
			id serial PRIMARY KEY,
			profile_id bigint NOT NULL,
			target_ingame_name character varying NOT NULL DEFAULT ''::character varying,
			target_friend_code character varying NOT NULL DEFAULT ''::character varying,
			reason character varying NOT NULL,
			moderator character varying,
			created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS panel_kick_history_created_idx
			ON public.panel_kick_history (created_at);

		CREATE INDEX IF NOT EXISTS panel_kick_history_profile_id_idx
			ON public.panel_kick_history (profile_id, created_at DESC);

		CREATE TABLE IF NOT EXISTS public.panel_room_reports (
			id serial PRIMARY KEY,
			room_id character varying NOT NULL,
			game_name character varying NOT NULL DEFAULT ''::character varying,
			room_type character varying NOT NULL DEFAULT ''::character varying,
			reporter_profile_id bigint NOT NULL,
			reporter_ingame_name character varying NOT NULL DEFAULT ''::character varying,
			reporter_friend_code character varying NOT NULL DEFAULT ''::character varying,
			reporter_type character varying NOT NULL DEFAULT 'user',
			target_profile_id bigint NOT NULL,
			target_ingame_name character varying NOT NULL DEFAULT ''::character varying,
			target_friend_code character varying NOT NULL DEFAULT ''::character varying,
			reason text NOT NULL,
			created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
		);

		CREATE UNIQUE INDEX IF NOT EXISTS panel_room_reports_once_idx
			ON public.panel_room_reports (reporter_profile_id, target_profile_id);

		CREATE INDEX IF NOT EXISTS panel_room_reports_target_idx
			ON public.panel_room_reports (target_profile_id, created_at DESC);

		CREATE INDEX IF NOT EXISTS panel_room_reports_created_idx
			ON public.panel_room_reports (created_at DESC);

		CREATE TABLE IF NOT EXISTS public.panel_match_logs (
			id serial PRIMARY KEY,
			source_key character varying NOT NULL UNIQUE,
			room_id character varying NOT NULL,
			dwc_group_id bigint NOT NULL DEFAULT 0,
			game_name character varying NOT NULL DEFAULT ''::character varying,
			match_type character varying NOT NULL DEFAULT ''::character varying,
			mkw_region character varying NOT NULL DEFAULT ''::character varying,
			race_number integer NOT NULL DEFAULT 0,
			course_id integer NOT NULL DEFAULT -1,
			engine_class_id integer NOT NULL DEFAULT -1,
			host_profile_id bigint,
			host_ingame_name character varying NOT NULL DEFAULT ''::character varying,
			player_count integer NOT NULL DEFAULT 0,
			players jsonb NOT NULL DEFAULT '[]'::jsonb,
			created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS panel_match_logs_created_idx
			ON public.panel_match_logs (created_at);

		-- Console-centric ban / link migration. The wipe runs only on the first
		-- pass that introduces public.users.console_id; subsequent restarts skip
		-- it because the column already exists.
		DO $consolemig$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'console_id'
			) THEN
				UPDATE public.users
				SET has_ban = false,
				    ban_issued = NULL,
				    ban_expires = NULL,
				    ban_revoked = NULL,
				    ban_reason = NULL,
				    ban_reason_hidden = NULL,
				    ban_moderator = NULL,
				    ban_tos = NULL;
				DELETE FROM public.panel_ban_history;
				UPDATE public.panel_accounts SET profile_id = NULL, linked_at = NULL;
			END IF;
		END
		$consolemig$;

		CREATE TABLE IF NOT EXISTS public.consoles (
			id serial PRIMARY KEY,
			ng_device_id bigint NOT NULL UNIQUE,
			account_id integer REFERENCES public.panel_accounts(id) ON DELETE SET NULL,
			primary_profile_id bigint,
			console_type text NOT NULL DEFAULT 'Wii',
			console_friend_code text NOT NULL DEFAULT ''::text,
			display_name text NOT NULL DEFAULT ''::text,
			has_ban boolean NOT NULL DEFAULT false,
			ban_tos boolean,
			ban_issued timestamp without time zone,
			ban_expires timestamp without time zone,
			ban_revoked timestamp without time zone,
			ban_reason text,
			ban_reason_hidden text,
			ban_moderator text,
			ban_ingame_name text,
			ban_profile_id bigint,
			created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
			linked_at timestamp without time zone,
			last_login_at timestamp without time zone
		);

		ALTER TABLE IF EXISTS public.consoles
			ADD IF NOT EXISTS console_type text NOT NULL DEFAULT 'Wii',
			ADD IF NOT EXISTS console_friend_code text NOT NULL DEFAULT ''::text,
			ADD IF NOT EXISTS display_name text NOT NULL DEFAULT ''::text,
			ADD IF NOT EXISTS last_login_at timestamp without time zone;

		DROP INDEX IF EXISTS public.consoles_account_unique_idx;

		CREATE INDEX IF NOT EXISTS consoles_account_idx
			ON public.consoles (account_id, linked_at DESC, id)
			WHERE account_id IS NOT NULL;

		ALTER TABLE IF EXISTS public.users
			ADD IF NOT EXISTS console_id integer REFERENCES public.consoles(id);

		CREATE INDEX IF NOT EXISTS users_console_id_idx
			ON public.users (console_id);

		ALTER TABLE IF EXISTS public.panel_accounts
			ADD IF NOT EXISTS console_id integer REFERENCES public.consoles(id) ON DELETE SET NULL;

		CREATE UNIQUE INDEX IF NOT EXISTS panel_accounts_console_unique_idx
			ON public.panel_accounts (console_id)
			WHERE console_id IS NOT NULL;

		ALTER TABLE IF EXISTS public.panel_ban_history
			ADD IF NOT EXISTS console_id integer REFERENCES public.consoles(id) ON DELETE SET NULL,
			ADD IF NOT EXISTS ng_device_id bigint;

		ALTER TABLE IF EXISTS public.panel_room_reports
			ADD IF NOT EXISTS reporter_type character varying NOT NULL DEFAULT 'user';
	`)
	return err
}

func (c *Connection) HasPanelAccounts() (bool, error) {
	var exists bool
	err := c.pool.QueryRow(c.ctx, `SELECT EXISTS(SELECT 1 FROM public.panel_accounts WHERE role IN ('owner', 'moderator'))`).Scan(&exists)
	return exists, err
}

func (c *Connection) CreatePanelAccount(username string, passwordHash []byte, role string) (PanelAccount, error) {
	account := PanelAccount{}
	rights := DefaultPanelRights(role)
	err := c.pool.QueryRow(c.ctx, `
		INSERT INTO public.panel_accounts (
			username,
			password_hash,
			role,
			can_view_users,
			can_view_bans,
			can_ban,
			can_unban,
			can_kick,
			can_message,
			can_manage_rooms,
			can_manage_rooms_create,
			can_manage_rooms_join,
			can_manage_rooms_delete,
			can_manage_rooms_end_game,
			can_view_logs,
			can_view_reports,
			can_manage_ghosts,
			can_report_players,
			right_open_host,
			log_types,
			can_manage_games,
			rights_initialized
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, true)
		RETURNING id, username, role, created_at
	`, username, passwordHash, role,
		rights.ViewUsers,
		rights.ViewBans,
		rights.Ban,
		rights.Unban,
		rights.Kick,
		rights.Message,
		rights.HasRoomRights(),
		rights.ManageRoomsCreate,
		rights.ManageRoomsJoin,
		rights.ManageRoomsDelete,
		rights.ManageRoomsEndGame,
		rights.ViewLogs,
		rights.ViewReports,
		rights.ManageGhosts,
		rights.ReportPlayers,
		rights.OpenHost,
		encodeLogTypes(rights.LogTypes),
		rights.ManageGames,
	).Scan(&account.ID, &account.Username, &account.Role, &account.CreatedAt)
	account.Rights = rights
	NormalizePanelRights(&account)
	return account, err
}

func (c *Connection) GetPanelAccountByUsername(username string) (PanelAccount, []byte, bool, error) {
	account := PanelAccount{}
	var passwordHash []byte
	var profileID pgtype.Int8
	var linkedAt pgtype.Timestamp
	var gsbrcd string
	var logTypes string
	err := c.pool.QueryRow(c.ctx, `
		SELECT a.id,
		       a.username,
		       a.role,
		       a.created_at,
		       a.password_hash,
		       a.profile_id,
		       a.linked_at,
		       COALESCE(u.gsbrcd, ''),
		       COALESCE(u.last_ingamesn, ''),
		       a.can_view_users,
		       a.can_view_bans,
		       a.can_ban,
		       a.can_unban,
		       a.can_kick,
		       a.can_message,
		       a.can_manage_rooms,
		       a.can_manage_rooms_create,
		       a.can_manage_rooms_join,
		       a.can_manage_rooms_delete,
		       a.can_manage_rooms_end_game,
		       a.can_view_logs,
		       a.can_view_reports,
		       a.can_manage_ghosts,
		       a.can_manage_games,
		       a.can_report_players,
		       a.right_open_host,
		       COALESCE(a.log_types, '')
		FROM public.panel_accounts a
		LEFT JOIN public.users u ON u.profile_id = a.profile_id
		WHERE lower(a.username) = lower($1)
	`, username).Scan(
		&account.ID,
		&account.Username,
		&account.Role,
		&account.CreatedAt,
		&passwordHash,
		&profileID,
		&linkedAt,
		&gsbrcd,
		&account.InGameName,
		&account.Rights.ViewUsers,
		&account.Rights.ViewBans,
		&account.Rights.Ban,
		&account.Rights.Unban,
		&account.Rights.Kick,
		&account.Rights.Message,
		&account.Rights.LegacyManageRooms,
		&account.Rights.ManageRoomsCreate,
		&account.Rights.ManageRoomsJoin,
		&account.Rights.ManageRoomsDelete,
		&account.Rights.ManageRoomsEndGame,
		&account.Rights.ViewLogs,
		&account.Rights.ViewReports,
		&account.Rights.ManageGhosts,
		&account.Rights.ManageGames,
		&account.Rights.ReportPlayers,
		&account.Rights.OpenHost,
		&logTypes,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return PanelAccount{}, nil, false, nil
		}

		return PanelAccount{}, nil, false, err
	}

	account.Rights.LogTypes = decodeLogTypes(logTypes)
	fillPanelAccountProfile(&account, profileID, linkedAt, gsbrcd)
	c.hydratePanelAccountLinkedProfiles(&account)
	NormalizePanelRights(&account)
	return account, passwordHash, true, nil
}

func (c *Connection) GetPanelAccountByID(accountID int) (PanelAccount, bool, error) {
	account := PanelAccount{}
	var profileID pgtype.Int8
	var linkedAt pgtype.Timestamp
	var gsbrcd string
	var logTypes string
	err := c.pool.QueryRow(c.ctx, `
		SELECT a.id,
		       a.username,
		       a.role,
		       a.created_at,
		       a.profile_id,
		       a.linked_at,
		       COALESCE(u.gsbrcd, ''),
		       COALESCE(u.last_ingamesn, ''),
		       a.can_view_users,
		       a.can_view_bans,
		       a.can_ban,
		       a.can_unban,
		       a.can_kick,
		       a.can_message,
		       a.can_manage_rooms,
		       a.can_manage_rooms_create,
		       a.can_manage_rooms_join,
		       a.can_manage_rooms_delete,
		       a.can_manage_rooms_end_game,
		       a.can_view_logs,
		       a.can_view_reports,
		       a.can_manage_ghosts,
		       a.can_manage_games,
		       a.can_report_players,
		       a.right_open_host,
		       COALESCE(a.log_types, '')
		FROM public.panel_accounts a
		LEFT JOIN public.users u ON u.profile_id = a.profile_id
		WHERE a.id = $1
	`, accountID).Scan(
		&account.ID,
		&account.Username,
		&account.Role,
		&account.CreatedAt,
		&profileID,
		&linkedAt,
		&gsbrcd,
		&account.InGameName,
		&account.Rights.ViewUsers,
		&account.Rights.ViewBans,
		&account.Rights.Ban,
		&account.Rights.Unban,
		&account.Rights.Kick,
		&account.Rights.Message,
		&account.Rights.LegacyManageRooms,
		&account.Rights.ManageRoomsCreate,
		&account.Rights.ManageRoomsJoin,
		&account.Rights.ManageRoomsDelete,
		&account.Rights.ManageRoomsEndGame,
		&account.Rights.ViewLogs,
		&account.Rights.ViewReports,
		&account.Rights.ManageGhosts,
		&account.Rights.ManageGames,
		&account.Rights.ReportPlayers,
		&account.Rights.OpenHost,
		&logTypes,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return PanelAccount{}, false, nil
		}
		return PanelAccount{}, false, err
	}

	account.Rights.LogTypes = decodeLogTypes(logTypes)
	fillPanelAccountProfile(&account, profileID, linkedAt, gsbrcd)
	c.hydratePanelAccountLinkedProfiles(&account)
	NormalizePanelRights(&account)
	return account, true, nil
}

func (c *Connection) ListPanelAccounts() ([]PanelAccount, error) {
	rows, err := c.pool.Query(c.ctx, `
		SELECT a.id,
		       a.username,
		       a.role,
		       a.created_at,
		       a.profile_id,
		       a.linked_at,
		       COALESCE(u.gsbrcd, ''),
		       COALESCE(u.last_ingamesn, ''),
		       a.can_view_users,
		       a.can_view_bans,
		       a.can_ban,
		       a.can_unban,
		       a.can_kick,
		       a.can_message,
		       a.can_manage_rooms,
		       a.can_manage_rooms_create,
		       a.can_manage_rooms_join,
		       a.can_manage_rooms_delete,
		       a.can_manage_rooms_end_game,
		       a.can_view_logs,
		       a.can_view_reports,
		       a.can_manage_ghosts,
		       a.can_manage_games,
		       a.can_report_players,
		       a.right_open_host,
		       COALESCE(a.log_types, '')
		FROM public.panel_accounts a
		LEFT JOIN public.users u ON u.profile_id = a.profile_id
		ORDER BY role, username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := []PanelAccount{}
	for rows.Next() {
		account := PanelAccount{}
		var profileID pgtype.Int8
		var linkedAt pgtype.Timestamp
		var gsbrcd string
		if err := scanPanelRights(rows, &account, &profileID, &linkedAt, &gsbrcd); err != nil {
			return nil, err
		}

		fillPanelAccountProfile(&account, profileID, linkedAt, gsbrcd)
		c.hydratePanelAccountLinkedProfiles(&account)
		accounts = append(accounts, account)
	}

	return accounts, rows.Err()
}

func (c *Connection) DeletePanelAccount(accountID int) (bool, error) {
	tx, err := c.pool.Begin(c.ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(c.ctx)

	var role string
	err = tx.QueryRow(c.ctx, `
		SELECT role
		FROM public.panel_accounts
		WHERE id = $1
		FOR UPDATE
	`, accountID).Scan(&role)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if role == "owner" {
		return false, nil
	}

	if _, err := tx.Exec(c.ctx, `
		UPDATE public.consoles
		SET account_id = NULL,
		    primary_profile_id = NULL,
		    display_name = '',
		    linked_at = NULL
		WHERE account_id = $1
	`, accountID); err != nil {
		return false, err
	}

	tag, err := tx.Exec(c.ctx, `
		DELETE FROM public.panel_accounts
		WHERE id = $1
		  AND role <> 'owner'
	`, accountID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	if err := tx.Commit(c.ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Connection) UpdatePanelAccountRole(accountID int, role string) (PanelAccount, bool, error) {
	account := PanelAccount{}
	rights := DefaultPanelRights(role)
	var profileID pgtype.Int8
	var linkedAt pgtype.Timestamp
	var gsbrcd string
	var logTypes string
	err := c.pool.QueryRow(c.ctx, `
		WITH updated AS (
			UPDATE public.panel_accounts
			SET role = $2,
			    can_view_users = $3,
			    can_view_bans = $4,
			    can_ban = $5,
			    can_unban = $6,
			    can_kick = $7,
			    can_message = $8,
			    can_manage_rooms = $9,
			    can_manage_rooms_create = $10,
			    can_manage_rooms_join = $11,
			    can_manage_rooms_delete = $12,
			    can_manage_rooms_end_game = $13,
			    can_view_logs = $14,
			    can_view_reports = $15,
			    can_manage_ghosts = $16,
			    can_report_players = $17,
			    right_open_host = $18,
			    log_types = $19,
			    can_manage_games = $20,
			    rights_initialized = true
			WHERE id = $1
			  AND role <> 'owner'
			RETURNING id,
			          username,
			          role,
			          created_at,
			          profile_id,
			          linked_at,
			          can_view_users,
			          can_view_bans,
			          can_ban,
			          can_unban,
			          can_kick,
			          can_message,
			          can_manage_rooms,
			          can_manage_rooms_create,
			          can_manage_rooms_join,
			          can_manage_rooms_delete,
			          can_manage_rooms_end_game,
			          can_view_logs,
			          can_view_reports,
			          can_manage_ghosts,
			          can_manage_games,
			          can_report_players,
			          right_open_host,
			          log_types
		)
		SELECT updated.id,
		       updated.username,
		       updated.role,
		       updated.created_at,
		       updated.profile_id,
		       updated.linked_at,
		       COALESCE(u.gsbrcd, ''),
		       COALESCE(u.last_ingamesn, ''),
		       updated.can_view_users,
		       updated.can_view_bans,
		       updated.can_ban,
		       updated.can_unban,
		       updated.can_kick,
		       updated.can_message,
		       updated.can_manage_rooms,
		       updated.can_manage_rooms_create,
		       updated.can_manage_rooms_join,
		       updated.can_manage_rooms_delete,
		       updated.can_manage_rooms_end_game,
		       updated.can_view_logs,
		       updated.can_view_reports,
		       updated.can_manage_ghosts,
		       updated.can_manage_games,
		       updated.can_report_players,
		       updated.right_open_host,
		       COALESCE(updated.log_types, '')
		FROM updated
		LEFT JOIN public.users u ON u.profile_id = updated.profile_id
	`, accountID,
		strings.TrimSpace(role),
		rights.ViewUsers,
		rights.ViewBans,
		rights.Ban,
		rights.Unban,
		rights.Kick,
		rights.Message,
		rights.HasRoomRights(),
		rights.ManageRoomsCreate,
		rights.ManageRoomsJoin,
		rights.ManageRoomsDelete,
		rights.ManageRoomsEndGame,
		rights.ViewLogs,
		rights.ViewReports,
		rights.ManageGhosts,
		rights.ReportPlayers,
		rights.OpenHost,
		encodeLogTypes(rights.LogTypes),
		rights.ManageGames,
	).Scan(
		&account.ID,
		&account.Username,
		&account.Role,
		&account.CreatedAt,
		&profileID,
		&linkedAt,
		&gsbrcd,
		&account.InGameName,
		&account.Rights.ViewUsers,
		&account.Rights.ViewBans,
		&account.Rights.Ban,
		&account.Rights.Unban,
		&account.Rights.Kick,
		&account.Rights.Message,
		&account.Rights.LegacyManageRooms,
		&account.Rights.ManageRoomsCreate,
		&account.Rights.ManageRoomsJoin,
		&account.Rights.ManageRoomsDelete,
		&account.Rights.ManageRoomsEndGame,
		&account.Rights.ViewLogs,
		&account.Rights.ViewReports,
		&account.Rights.ManageGhosts,
		&account.Rights.ManageGames,
		&account.Rights.ReportPlayers,
		&account.Rights.OpenHost,
		&logTypes,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return PanelAccount{}, false, nil
		}

		return PanelAccount{}, false, err
	}

	account.Rights.LogTypes = decodeLogTypes(logTypes)
	fillPanelAccountProfile(&account, profileID, linkedAt, gsbrcd)
	c.hydratePanelAccountLinkedProfiles(&account)
	NormalizePanelRights(&account)
	return account, true, nil
}

func (c *Connection) UpdatePanelAccountRights(accountID int, rights PanelRights) (PanelAccount, bool, error) {
	account := PanelAccount{}
	rights.ApplyLegacyRoomRights()
	role := "player"
	if rights.HasModerationRights() {
		role = "moderator"
	}

	logTypesValue := encodeLogTypes(rights.LogTypes)
	if !rights.ViewLogs {
		logTypesValue = ""
	}

	var profileID pgtype.Int8
	var linkedAt pgtype.Timestamp
	var gsbrcd string
	var logTypes string
	err := c.pool.QueryRow(c.ctx, `
		WITH updated AS (
			UPDATE public.panel_accounts
			SET role = $2,
			    can_view_users = $3,
			    can_view_bans = $4,
			    can_ban = $5,
			    can_unban = $6,
			    can_kick = $7,
			    can_message = $8,
			    can_manage_rooms = $9,
			    can_manage_rooms_create = $10,
			    can_manage_rooms_join = $11,
			    can_manage_rooms_delete = $12,
			    can_manage_rooms_end_game = $13,
			    can_view_logs = $14,
			    can_view_reports = $15,
			    can_manage_ghosts = $16,
			    can_report_players = $17,
			    right_open_host = $18,
			    log_types = $19,
			    can_manage_games = $20,
			    rights_initialized = true
			WHERE id = $1
			  AND role <> 'owner'
			RETURNING id,
			          username,
			          role,
			          created_at,
			          profile_id,
			          linked_at,
			          can_view_users,
			          can_view_bans,
			          can_ban,
			          can_unban,
			          can_kick,
			          can_message,
			          can_manage_rooms,
			          can_manage_rooms_create,
			          can_manage_rooms_join,
			          can_manage_rooms_delete,
			          can_manage_rooms_end_game,
			          can_view_logs,
			          can_view_reports,
			          can_manage_ghosts,
			          can_manage_games,
			          can_report_players,
			          right_open_host,
			          log_types
		)
		SELECT updated.id,
		       updated.username,
		       updated.role,
		       updated.created_at,
		       updated.profile_id,
		       updated.linked_at,
		       COALESCE(u.gsbrcd, ''),
		       COALESCE(u.last_ingamesn, ''),
		       updated.can_view_users,
		       updated.can_view_bans,
		       updated.can_ban,
		       updated.can_unban,
		       updated.can_kick,
		       updated.can_message,
		       updated.can_manage_rooms,
		       updated.can_manage_rooms_create,
		       updated.can_manage_rooms_join,
		       updated.can_manage_rooms_delete,
		       updated.can_manage_rooms_end_game,
		       updated.can_view_logs,
		       updated.can_view_reports,
		       updated.can_manage_ghosts,
		       updated.can_manage_games,
		       updated.can_report_players,
		       updated.right_open_host,
		       COALESCE(updated.log_types, '')
		FROM updated
		LEFT JOIN public.users u ON u.profile_id = updated.profile_id
	`, accountID,
		role,
		rights.ViewUsers,
		rights.ViewBans,
		rights.Ban,
		rights.Unban,
		rights.Kick,
		rights.Message,
		rights.HasRoomRights(),
		rights.ManageRoomsCreate,
		rights.ManageRoomsJoin,
		rights.ManageRoomsDelete,
		rights.ManageRoomsEndGame,
		rights.ViewLogs,
		rights.ViewReports,
		rights.ManageGhosts,
		rights.ReportPlayers,
		rights.OpenHost,
		logTypesValue,
		rights.ManageGames,
	).Scan(
		&account.ID,
		&account.Username,
		&account.Role,
		&account.CreatedAt,
		&profileID,
		&linkedAt,
		&gsbrcd,
		&account.InGameName,
		&account.Rights.ViewUsers,
		&account.Rights.ViewBans,
		&account.Rights.Ban,
		&account.Rights.Unban,
		&account.Rights.Kick,
		&account.Rights.Message,
		&account.Rights.LegacyManageRooms,
		&account.Rights.ManageRoomsCreate,
		&account.Rights.ManageRoomsJoin,
		&account.Rights.ManageRoomsDelete,
		&account.Rights.ManageRoomsEndGame,
		&account.Rights.ViewLogs,
		&account.Rights.ViewReports,
		&account.Rights.ManageGhosts,
		&account.Rights.ManageGames,
		&account.Rights.ReportPlayers,
		&account.Rights.OpenHost,
		&logTypes,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return PanelAccount{}, false, nil
		}

		return PanelAccount{}, false, err
	}

	account.Rights.LogTypes = decodeLogTypes(logTypes)
	fillPanelAccountProfile(&account, profileID, linkedAt, gsbrcd)
	c.hydratePanelAccountLinkedProfiles(&account)
	NormalizePanelRights(&account)
	return account, true, nil
}

func (c *Connection) CreatePanelSession(tokenHash string, accountID int, expiresAt time.Time) error {
	_, err := c.pool.Exec(c.ctx, `
		INSERT INTO public.panel_sessions (token_hash, account_id, expires_at)
		VALUES ($1, $2, $3)
	`, tokenHash, accountID, expiresAt)
	return err
}

func (c *Connection) GetPanelSession(tokenHash string) (PanelAccount, bool, error) {
	_, _ = c.pool.Exec(c.ctx, `DELETE FROM public.panel_sessions WHERE expires_at <= CURRENT_TIMESTAMP`)

	account := PanelAccount{}
	var profileID pgtype.Int8
	var linkedAt pgtype.Timestamp
	var gsbrcd string
	var logTypes string
	err := c.pool.QueryRow(c.ctx, `
		SELECT a.id,
		       a.username,
		       a.role,
		       a.created_at,
		       a.profile_id,
		       a.linked_at,
		       COALESCE(u.gsbrcd, ''),
		       COALESCE(u.last_ingamesn, ''),
		       a.can_view_users,
		       a.can_view_bans,
		       a.can_ban,
		       a.can_unban,
		       a.can_kick,
		       a.can_message,
		       a.can_manage_rooms,
		       a.can_manage_rooms_create,
		       a.can_manage_rooms_join,
		       a.can_manage_rooms_delete,
		       a.can_manage_rooms_end_game,
		       a.can_view_logs,
		       a.can_view_reports,
		       a.can_manage_ghosts,
		       a.can_manage_games,
		       a.can_report_players,
		       a.right_open_host,
		       COALESCE(a.log_types, '')
		FROM public.panel_sessions s
		JOIN public.panel_accounts a ON a.id = s.account_id
		LEFT JOIN public.users u ON u.profile_id = a.profile_id
		WHERE s.token_hash = $1
		  AND s.expires_at > CURRENT_TIMESTAMP
	`, tokenHash).Scan(
		&account.ID,
		&account.Username,
		&account.Role,
		&account.CreatedAt,
		&profileID,
		&linkedAt,
		&gsbrcd,
		&account.InGameName,
		&account.Rights.ViewUsers,
		&account.Rights.ViewBans,
		&account.Rights.Ban,
		&account.Rights.Unban,
		&account.Rights.Kick,
		&account.Rights.Message,
		&account.Rights.LegacyManageRooms,
		&account.Rights.ManageRoomsCreate,
		&account.Rights.ManageRoomsJoin,
		&account.Rights.ManageRoomsDelete,
		&account.Rights.ManageRoomsEndGame,
		&account.Rights.ViewLogs,
		&account.Rights.ViewReports,
		&account.Rights.ManageGhosts,
		&account.Rights.ManageGames,
		&account.Rights.ReportPlayers,
		&account.Rights.OpenHost,
		&logTypes,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return PanelAccount{}, false, nil
		}

		return PanelAccount{}, false, err
	}

	account.Rights.LogTypes = decodeLogTypes(logTypes)
	fillPanelAccountProfile(&account, profileID, linkedAt, gsbrcd)
	c.hydratePanelAccountLinkedProfiles(&account)
	NormalizePanelRights(&account)
	return account, true, nil
}

func (c *Connection) DeletePanelSession(tokenHash string) error {
	_, err := c.pool.Exec(c.ctx, `DELETE FROM public.panel_sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func fillPanelAccountProfile(account *PanelAccount, profileID pgtype.Int8, linkedAt pgtype.Timestamp, gsbrcd string) {
	if profileID.Status == pgtype.Present && profileID.Int > 0 {
		account.ProfileID = uint32(profileID.Int)
		if len(gsbrcd) >= 4 {
			account.FriendCode = common.CalcFriendCodeString(account.ProfileID, gsbrcd[:4])
		}
	}

	if linkedAt.Status == pgtype.Present {
		t := linkedAt.Time
		account.LinkedAt = &t
	}
}

func (c *Connection) hydratePanelAccountLinkedProfiles(account *PanelAccount) {
	if account == nil || account.ID <= 0 {
		return
	}

	rows, err := c.pool.Query(c.ctx, `
		WITH linked_profiles AS (
			SELECT u.profile_id,
			       COALESCE(u.gsbrcd, '') AS gsbrcd,
			       COALESCE(u.last_ingamesn, '') AS ingame_name,
			       u.has_ban,
			       COALESCE(u.open_host, true) AS open_host,
			       (u.profile_id = a.profile_id) AS is_primary
			FROM public.panel_accounts a
			JOIN public.consoles c ON c.account_id = a.id
			JOIN public.users u ON u.console_id = c.id
			WHERE a.id = $1
			UNION ALL
			SELECT u.profile_id,
			       COALESCE(u.gsbrcd, '') AS gsbrcd,
			       COALESCE(u.last_ingamesn, '') AS ingame_name,
			       u.has_ban,
			       COALESCE(u.open_host, true) AS open_host,
			       true AS is_primary
			FROM public.panel_accounts a
			JOIN public.users u ON u.profile_id = a.profile_id
			WHERE a.id = $1
			  AND a.profile_id IS NOT NULL
		)
		SELECT DISTINCT ON (profile_id)
		       profile_id,
		       gsbrcd,
		       ingame_name,
		       has_ban,
		       open_host,
		       is_primary
		FROM linked_profiles
		ORDER BY profile_id, is_primary DESC
	`, account.ID)
	if err != nil {
		return
	}
	defer rows.Close()

	profiles := []ConsoleProfileRow{}
	for rows.Next() {
		var pid uint32
		var gsbrcd, name string
		var hasBan, openHost, primary bool
		if err := rows.Scan(&pid, &gsbrcd, &name, &hasBan, &openHost, &primary); err != nil {
			return
		}
		fc := ""
		if len(gsbrcd) >= 4 {
			fc = common.CalcFriendCodeString(pid, gsbrcd[:4])
		}
		profiles = append(profiles, ConsoleProfileRow{
			ProfileID:  pid,
			FriendCode: fc,
			InGameName: name,
			IsPrimary:  primary,
			HasBan:     hasBan,
			OpenHost:   openHost,
		})
	}
	if err := rows.Err(); err != nil {
		return
	}
	account.LinkedProfiles = profiles
}

func (c *Connection) IsProfileLinkedToOtherPanelAccount(profileID uint32, accountID int) (bool, error) {
	var exists bool
	err := c.pool.QueryRow(c.ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM public.users u
			JOIN public.consoles c ON c.id = u.console_id
			WHERE u.profile_id = $1
			  AND c.account_id IS NOT NULL
			  AND c.account_id <> $2
			UNION ALL
			SELECT 1
			FROM public.panel_accounts a
			WHERE a.profile_id = $1
			  AND a.id <> $2
		)
	`, profileID, accountID).Scan(&exists)
	return exists, err
}

func (c *Connection) CreatePanelProfileLinkCode(accountID int, profileID uint32, codeHash string, expiresAt time.Time) error {
	tx, err := c.pool.Begin(c.ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(c.ctx)
	}()

	_, err = tx.Exec(c.ctx, `
		UPDATE public.panel_profile_link_codes
		SET consumed_at = CURRENT_TIMESTAMP
		WHERE account_id = $1
		  AND consumed_at IS NULL
	`, accountID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(c.ctx, `
		INSERT INTO public.panel_profile_link_codes (account_id, profile_id, code_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, accountID, profileID, codeHash, expiresAt)
	if err != nil {
		return err
	}

	return tx.Commit(c.ctx)
}

func (c *Connection) ConsumePanelProfileLinkCode(accountID int, profileID uint32, codeHash string) (bool, error) {
	var id int
	err := c.pool.QueryRow(c.ctx, `
		UPDATE public.panel_profile_link_codes
		SET consumed_at = CURRENT_TIMESTAMP
		WHERE id = (
			SELECT id
			FROM public.panel_profile_link_codes
			WHERE account_id = $1
			  AND profile_id = $2
			  AND code_hash = $3
			  AND consumed_at IS NULL
			  AND expires_at > CURRENT_TIMESTAMP
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		)
		RETURNING id
	`, accountID, profileID, codeHash).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (c *Connection) LinkPanelAccountProfile(accountID int, profileID uint32) (PanelAccount, error) {
	account := PanelAccount{}
	var profileIDValue pgtype.Int8
	var linkedAt pgtype.Timestamp
	var gsbrcd string
	var logTypes string
	err := c.pool.QueryRow(c.ctx, `
		WITH updated AS (
			UPDATE public.panel_accounts
			SET profile_id = $2,
			    linked_at = CURRENT_TIMESTAMP
			WHERE id = $1
			RETURNING id,
			          username,
			          role,
			          created_at,
			          profile_id,
			          linked_at,
			          can_view_users,
			          can_view_bans,
			          can_ban,
			          can_unban,
			          can_kick,
			          can_message,
			          can_manage_rooms,
			          can_manage_rooms_create,
			          can_manage_rooms_join,
			          can_manage_rooms_delete,
			          can_manage_rooms_end_game,
			          can_view_logs,
			          can_view_reports,
			          can_manage_ghosts,
			          can_manage_games,
			          can_report_players,
			          right_open_host,
			          log_types
		)
		SELECT updated.id,
		       updated.username,
		       updated.role,
		       updated.created_at,
		       updated.profile_id,
		       updated.linked_at,
		       COALESCE(u.gsbrcd, ''),
		       COALESCE(u.last_ingamesn, ''),
		       updated.can_view_users,
		       updated.can_view_bans,
		       updated.can_ban,
		       updated.can_unban,
		       updated.can_kick,
		       updated.can_message,
		       updated.can_manage_rooms,
		       updated.can_manage_rooms_create,
		       updated.can_manage_rooms_join,
		       updated.can_manage_rooms_delete,
		       updated.can_manage_rooms_end_game,
		       updated.can_view_logs,
		       updated.can_view_reports,
		       updated.can_manage_ghosts,
		       updated.can_manage_games,
		       updated.can_report_players,
		       updated.right_open_host,
		       COALESCE(updated.log_types, '')
		FROM updated
		LEFT JOIN public.users u ON u.profile_id = updated.profile_id
	`, accountID, profileID).Scan(
		&account.ID,
		&account.Username,
		&account.Role,
		&account.CreatedAt,
		&profileIDValue,
		&linkedAt,
		&gsbrcd,
		&account.InGameName,
		&account.Rights.ViewUsers,
		&account.Rights.ViewBans,
		&account.Rights.Ban,
		&account.Rights.Unban,
		&account.Rights.Kick,
		&account.Rights.Message,
		&account.Rights.LegacyManageRooms,
		&account.Rights.ManageRoomsCreate,
		&account.Rights.ManageRoomsJoin,
		&account.Rights.ManageRoomsDelete,
		&account.Rights.ManageRoomsEndGame,
		&account.Rights.ViewLogs,
		&account.Rights.ViewReports,
		&account.Rights.ManageGhosts,
		&account.Rights.ManageGames,
		&account.Rights.ReportPlayers,
		&account.Rights.OpenHost,
		&logTypes,
	)
	if err != nil {
		return PanelAccount{}, err
	}

	account.Rights.LogTypes = decodeLogTypes(logTypes)
	fillPanelAccountProfile(&account, profileIDValue, linkedAt, gsbrcd)
	c.hydratePanelAccountLinkedProfiles(&account)
	NormalizePanelRights(&account)
	return account, nil
}

func (c *Connection) UnlinkPanelAccountProfile(accountID int) (PanelAccount, bool, error) {
	account := PanelAccount{}
	var profileIDValue pgtype.Int8
	var linkedAt pgtype.Timestamp
	var gsbrcd string
	var logTypes string
	err := c.pool.QueryRow(c.ctx, `
		WITH updated AS (
			UPDATE public.panel_accounts
			SET profile_id = NULL,
			    linked_at = NULL
			WHERE id = $1
			RETURNING id,
			          username,
			          role,
			          created_at,
			          profile_id,
			          linked_at,
			          can_view_users,
			          can_view_bans,
			          can_ban,
			          can_unban,
			          can_kick,
			          can_message,
			          can_manage_rooms,
			          can_manage_rooms_create,
			          can_manage_rooms_join,
			          can_manage_rooms_delete,
			          can_manage_rooms_end_game,
			          can_view_logs,
			          can_view_reports,
			          can_manage_ghosts,
			          can_manage_games,
			          can_report_players,
			          right_open_host,
			          log_types
		)
		SELECT updated.id,
		       updated.username,
		       updated.role,
		       updated.created_at,
		       updated.profile_id,
		       updated.linked_at,
		       COALESCE(u.gsbrcd, ''),
		       COALESCE(u.last_ingamesn, ''),
		       updated.can_view_users,
		       updated.can_view_bans,
		       updated.can_ban,
		       updated.can_unban,
		       updated.can_kick,
		       updated.can_message,
		       updated.can_manage_rooms,
		       updated.can_manage_rooms_create,
		       updated.can_manage_rooms_join,
		       updated.can_manage_rooms_delete,
		       updated.can_manage_rooms_end_game,
		       updated.can_view_logs,
		       updated.can_view_reports,
		       updated.can_manage_ghosts,
		       updated.can_manage_games,
		       updated.can_report_players,
		       updated.right_open_host,
		       COALESCE(updated.log_types, '')
		FROM updated
		LEFT JOIN public.users u ON u.profile_id = updated.profile_id
	`, accountID).Scan(
		&account.ID,
		&account.Username,
		&account.Role,
		&account.CreatedAt,
		&profileIDValue,
		&linkedAt,
		&gsbrcd,
		&account.InGameName,
		&account.Rights.ViewUsers,
		&account.Rights.ViewBans,
		&account.Rights.Ban,
		&account.Rights.Unban,
		&account.Rights.Kick,
		&account.Rights.Message,
		&account.Rights.LegacyManageRooms,
		&account.Rights.ManageRoomsCreate,
		&account.Rights.ManageRoomsJoin,
		&account.Rights.ManageRoomsDelete,
		&account.Rights.ManageRoomsEndGame,
		&account.Rights.ViewLogs,
		&account.Rights.ViewReports,
		&account.Rights.ManageGhosts,
		&account.Rights.ManageGames,
		&account.Rights.ReportPlayers,
		&account.Rights.OpenHost,
		&logTypes,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return PanelAccount{}, false, nil
		}
		return PanelAccount{}, false, err
	}

	account.Rights.LogTypes = decodeLogTypes(logTypes)
	fillPanelAccountProfile(&account, profileIDValue, linkedAt, gsbrcd)
	c.hydratePanelAccountLinkedProfiles(&account)
	NormalizePanelRights(&account)
	return account, true, nil
}

func (c *Connection) CreatePanelLoginMessage(profileID uint32, message string, moderator string) (PanelLoginMessage, error) {
	loginMessage := PanelLoginMessage{}
	err := c.pool.QueryRow(c.ctx, `
		INSERT INTO public.panel_login_messages (profile_id, message, moderator)
		VALUES ($1, $2, $3)
		RETURNING id, profile_id, message, COALESCE(moderator, ''), created_at
	`, profileID, message, moderator).Scan(
		&loginMessage.ID,
		&loginMessage.ProfileID,
		&loginMessage.Message,
		&loginMessage.Moderator,
		&loginMessage.CreatedAt,
	)
	return loginMessage, err
}

func (c *Connection) HasPendingLoginMessage(profileID uint32) (bool, error) {
	exists := false
	err := c.pool.QueryRow(c.ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM public.panel_login_messages
			WHERE profile_id = $1
			  AND delivered_at IS NULL
		)
	`, profileID).Scan(&exists)
	return exists, err
}

func (c *Connection) TakePendingLoginMessage(profileID uint32) (PanelLoginMessage, bool, error) {
	tx, err := c.pool.Begin(c.ctx)
	if err != nil {
		return PanelLoginMessage{}, false, err
	}
	defer func() {
		_ = tx.Rollback(c.ctx)
	}()

	loginMessage := PanelLoginMessage{}
	err = tx.QueryRow(c.ctx, `
		SELECT id, profile_id, message, COALESCE(moderator, ''), created_at
		FROM public.panel_login_messages
		WHERE profile_id = $1
		  AND delivered_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT 1
		FOR UPDATE
	`, profileID).Scan(
		&loginMessage.ID,
		&loginMessage.ProfileID,
		&loginMessage.Message,
		&loginMessage.Moderator,
		&loginMessage.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			if commitErr := tx.Commit(c.ctx); commitErr != nil {
				return PanelLoginMessage{}, false, commitErr
			}
			return PanelLoginMessage{}, false, nil
		}
		return PanelLoginMessage{}, false, err
	}

	_, err = tx.Exec(c.ctx, `
		UPDATE public.panel_login_messages
		SET delivered_at = CURRENT_TIMESTAMP
		WHERE profile_id = $1
		  AND delivered_at IS NULL
	`, profileID)
	if err != nil {
		return PanelLoginMessage{}, false, err
	}

	if err := tx.Commit(c.ctx); err != nil {
		return PanelLoginMessage{}, false, err
	}

	return loginMessage, true, nil
}

func (c *Connection) ListBans(includePrivate bool) ([]BanRecord, error) {
	rows, err := c.pool.Query(c.ctx, `
		SELECT kind,
		       profile_id,
		       gsbrcd,
		       friend_code,
		       ingame_name,
		       reason,
		       reason_hidden,
		       moderator,
		       tos,
		       status,
		       issued,
		       expires,
		       revoked,
		       room_id,
		       console_id,
		       ng_device_id
		FROM (
			SELECT 'ban'::text AS kind,
			       h.profile_id,
			       COALESCE(NULLIF(h.gsbrcd, ''), COALESCE(u.gsbrcd, '')) AS gsbrcd,
			       ''::text AS friend_code,
			       COALESCE(NULLIF(h.ingame_name, ''), COALESCE(u.last_ingamesn, '')) AS ingame_name,
			       COALESCE(h.reason, '') AS reason,
			       COALESCE(h.reason_hidden, '') AS reason_hidden,
			       COALESCE(h.moderator, '') AS moderator,
			       COALESCE(h.tos, false) AS tos,
			       CASE
			         WHEN h.revoked IS NOT NULL THEN 'revoked'
			         WHEN h.expires IS NOT NULL AND h.expires <= CURRENT_TIMESTAMP THEN 'expired'
			         ELSE 'active'
			       END AS status,
			       h.issued,
			       h.expires,
			       h.revoked,
			       ''::text AS room_id,
			       COALESCE(h.console_id, 0) AS console_id,
			       COALESCE(h.ng_device_id, c.ng_device_id, 0) AS ng_device_id,
			       COALESCE(h.revoked, h.expires, h.issued) AS sort_time,
			       h.id AS sort_id
			FROM public.panel_ban_history h
			LEFT JOIN public.users u ON u.profile_id = h.profile_id
			LEFT JOIN public.consoles c ON c.id = h.console_id
			WHERE (
			        h.revoked IS NULL
			    AND (h.expires IS NULL OR h.expires > CURRENT_TIMESTAMP - make_interval(days => $1))
			  )
			   OR (
			        h.revoked IS NOT NULL
			    AND h.revoked > CURRENT_TIMESTAMP - make_interval(days => $1)
			  )

			UNION ALL

			SELECT 'moderator_kick'::text AS kind,
			       k.profile_id,
			       COALESCE(u.gsbrcd, '') AS gsbrcd,
			       COALESCE(k.target_friend_code, '') AS friend_code,
			       COALESCE(NULLIF(k.target_ingame_name, ''), COALESCE(u.last_ingamesn, '')) AS ingame_name,
			       k.reason,
			       ''::text AS reason_hidden,
			       COALESCE(k.moderator, '') AS moderator,
			       false AS tos,
			       'kick'::text AS status,
			       k.created_at AS issued,
			       NULL::timestamp without time zone AS expires,
			       NULL::timestamp without time zone AS revoked,
			       ''::text AS room_id,
			       0 AS console_id,
			       0::bigint AS ng_device_id,
			       k.created_at AS sort_time,
			       k.id AS sort_id
			FROM public.panel_kick_history k
			LEFT JOIN public.users u ON u.profile_id = k.profile_id
			WHERE k.created_at > CURRENT_TIMESTAMP - make_interval(days => $1)

			UNION ALL

			SELECT 'private_room_kick'::text AS kind,
			       k.target_profile_id AS profile_id,
			       COALESCE(u.gsbrcd, '') AS gsbrcd,
			       COALESCE(k.target_friend_code, '') AS friend_code,
			       COALESCE(NULLIF(k.target_ingame_name, ''), COALESCE(u.last_ingamesn, '')) AS ingame_name,
			       k.reason,
			       ''::text AS reason_hidden,
			       COALESCE(k.moderator, '') AS moderator,
			       false AS tos,
			       'kick'::text AS status,
			       k.created_at AS issued,
			       NULL::timestamp without time zone AS expires,
			       NULL::timestamp without time zone AS revoked,
			       k.room_id,
			       0 AS console_id,
			       0::bigint AS ng_device_id,
			       k.created_at AS sort_time,
			       k.id AS sort_id
			FROM public.panel_room_kick_history k
			LEFT JOIN public.users u ON u.profile_id = k.target_profile_id
			WHERE k.created_at > CURRENT_TIMESTAMP - make_interval(days => $1)
		) records
		ORDER BY sort_time DESC NULLS LAST, sort_id DESC
	`, common.GetConfig().BanHistoryDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bans := []BanRecord{}
	for rows.Next() {
		var issued pgtype.Timestamp
		var expires pgtype.Timestamp
		var revoked pgtype.Timestamp
		var gsbrcd string
		var storedFriendCode string
		var consoleID int
		var ngDeviceID int64
		ban := BanRecord{}

		if err := rows.Scan(
			&ban.Kind,
			&ban.ProfileID,
			&gsbrcd,
			&storedFriendCode,
			&ban.InGameName,
			&ban.Reason,
			&ban.ReasonHidden,
			&ban.Moderator,
			&ban.TOS,
			&ban.Status,
			&issued,
			&expires,
			&revoked,
			&ban.RoomID,
			&consoleID,
			&ngDeviceID,
		); err != nil {
			return nil, err
		}

		if storedFriendCode != "" {
			ban.FriendCode = storedFriendCode
		} else if len(gsbrcd) >= 4 {
			ban.FriendCode = common.CalcFriendCodeString(ban.ProfileID, gsbrcd[:4])
		}

		if issued.Status == pgtype.Present {
			t := issued.Time
			ban.Issued = &t
		}

		if expires.Status == pgtype.Present {
			t := expires.Time
			ban.Expires = &t
		}

		if revoked.Status == pgtype.Present {
			t := revoked.Time
			ban.Revoked = &t
		}

		ban.ConsoleID = consoleID
		if ngDeviceID > 0 {
			ban.NgDeviceID = uint32(ngDeviceID)
			ban.NgDeviceIDHex = strings.ToUpper(strconv.FormatInt(ngDeviceID, 16))
		}

		if !includePrivate {
			ban.Moderator = ""
			ban.ReasonHidden = ""
		}

		bans = append(bans, ban)
	}

	return bans, rows.Err()
}

func (c *Connection) RecordRoomKick(roomID string, gameName string, ownerProfileID uint32, targetProfileID uint32, targetName string, targetFriendCode string, reason string, moderator string) error {
	_, err := c.pool.Exec(c.ctx, `
		INSERT INTO public.panel_room_kick_history (
			room_id, game_name, owner_profile_id, target_profile_id, target_ingame_name, target_friend_code, reason, moderator
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, strings.TrimSpace(roomID), strings.TrimSpace(gameName), ownerProfileID, targetProfileID, strings.TrimSpace(targetName), strings.TrimSpace(targetFriendCode), strings.TrimSpace(reason), strings.TrimSpace(moderator))
	return err
}

func (c *Connection) RecordModeratorKick(profileID uint32, targetName string, targetFriendCode string, reason string, moderator string) error {
	_, err := c.pool.Exec(c.ctx, `
		INSERT INTO public.panel_kick_history (
			profile_id, target_ingame_name, target_friend_code, reason, moderator
		)
		VALUES ($1, $2, $3, $4, $5)
	`, profileID, strings.TrimSpace(targetName), strings.TrimSpace(targetFriendCode), strings.TrimSpace(reason), strings.TrimSpace(moderator))
	return err
}

func normalizePanelRoomReportType(reportType string) string {
	switch strings.TrimSpace(strings.ToLower(reportType)) {
	case "moderator":
		return "moderator"
	default:
		return "user"
	}
}

func (c *Connection) RecordRoomReport(roomID string, gameName string, roomType string, reporterProfileID uint32, reporterName string, reporterFriendCode string, reporterType string, targetProfileID uint32, targetName string, targetFriendCode string, reason string) error {
	if err := c.PruneRoomReports(); err != nil {
		return err
	}
	reporterType = normalizePanelRoomReportType(reporterType)

	var id int
	err := c.pool.QueryRow(c.ctx, `
		INSERT INTO public.panel_room_reports (
			room_id,
			game_name,
			room_type,
			reporter_profile_id,
			reporter_ingame_name,
			reporter_friend_code,
			reporter_type,
			target_profile_id,
			target_ingame_name,
			target_friend_code,
			reason
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (reporter_profile_id, target_profile_id) DO NOTHING
		RETURNING id
	`, strings.TrimSpace(roomID), strings.TrimSpace(gameName), strings.TrimSpace(roomType), reporterProfileID, strings.TrimSpace(reporterName), strings.TrimSpace(reporterFriendCode), reporterType, targetProfileID, strings.TrimSpace(targetName), strings.TrimSpace(targetFriendCode), strings.TrimSpace(reason)).Scan(&id)
	if err == pgx.ErrNoRows {
		return ErrDuplicateRoomReport
	}
	return err
}

func (c *Connection) PruneRoomReports() error {
	_, err := c.pool.Exec(c.ctx, `
		DELETE FROM public.panel_room_reports
		WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '7 days'
	`)
	return err
}

func (c *Connection) ClearRoomReportsForProfile(targetProfileID uint32) (int64, error) {
	if targetProfileID == 0 {
		return 0, nil
	}
	tag, err := c.pool.Exec(c.ctx, `
		DELETE FROM public.panel_room_reports
		WHERE target_profile_id = $1
	`, targetProfileID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (c *Connection) DeleteRoomReport(reportID int) (bool, error) {
	if reportID <= 0 {
		return false, nil
	}
	tag, err := c.pool.Exec(c.ctx, `
		DELETE FROM public.panel_room_reports
		WHERE id = $1
	`, reportID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (c *Connection) ListRoomReportSummaries() ([]PanelRoomReportSummary, error) {
	if err := c.PruneRoomReports(); err != nil {
		return nil, err
	}

	rows, err := c.pool.Query(c.ctx, `
		WITH counts AS (
			SELECT target_profile_id,
			       COUNT(*) AS report_count,
			       COUNT(*) FILTER (WHERE lower(reporter_type) = 'moderator') AS moderator_report_count,
			       COUNT(*) FILTER (WHERE lower(reporter_type) <> 'moderator') AS user_report_count,
			       MAX(created_at) AS last_reported
			FROM public.panel_room_reports
			GROUP BY target_profile_id
		),
		latest AS (
			SELECT DISTINCT ON (target_profile_id)
			       target_profile_id,
			       target_ingame_name,
			       target_friend_code
			FROM public.panel_room_reports
			ORDER BY target_profile_id, created_at DESC, id DESC
		)
		SELECT counts.target_profile_id,
		       COALESCE(NULLIF(u.last_ingamesn, ''), NULLIF(latest.target_ingame_name, ''), ''),
		       COALESCE(latest.target_friend_code, ''),
		       COALESCE(u.gsbrcd, ''),
		       counts.report_count,
		       counts.moderator_report_count,
		       counts.user_report_count,
		       counts.last_reported
		FROM counts
		LEFT JOIN latest ON latest.target_profile_id = counts.target_profile_id
		LEFT JOIN public.users u ON u.profile_id = counts.target_profile_id
		ORDER BY counts.report_count DESC, counts.last_reported DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := []PanelRoomReportSummary{}
	for rows.Next() {
		var summary PanelRoomReportSummary
		var gsbrcd string
		if err := rows.Scan(&summary.ProfileID, &summary.InGameName, &summary.FriendCode, &gsbrcd, &summary.ReportCount, &summary.ModeratorReportCount, &summary.UserReportCount, &summary.LastReported); err != nil {
			return nil, err
		}
		if summary.FriendCode == "" && len(gsbrcd) >= 4 {
			summary.FriendCode = common.CalcFriendCodeString(summary.ProfileID, gsbrcd[:4])
		}
		reports = append(reports, summary)
	}
	return reports, rows.Err()
}

func (c *Connection) ListRoomReportDetails(targetProfileID uint32) ([]PanelRoomReportDetail, error) {
	if err := c.PruneRoomReports(); err != nil {
		return nil, err
	}

	rows, err := c.pool.Query(c.ctx, `
		SELECT id,
		       room_id,
		       game_name,
		       room_type,
		       reporter_profile_id,
		       reporter_ingame_name,
		       reporter_friend_code,
		       reporter_type,
		       target_profile_id,
		       target_ingame_name,
		       target_friend_code,
		       reason,
		       created_at
		FROM public.panel_room_reports
		WHERE target_profile_id = $1
		ORDER BY created_at DESC, id DESC
	`, targetProfileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := []PanelRoomReportDetail{}
	for rows.Next() {
		var report PanelRoomReportDetail
		if err := rows.Scan(
			&report.ID,
			&report.RoomID,
			&report.GameName,
			&report.RoomType,
			&report.ReporterProfileID,
			&report.ReporterName,
			&report.ReporterFriendCode,
			&report.ReporterType,
			&report.TargetProfileID,
			&report.TargetName,
			&report.TargetFriendCode,
			&report.Reason,
			&report.CreatedAt,
		); err != nil {
			return nil, err
		}
		report.ReporterType = normalizePanelRoomReportType(report.ReporterType)
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (c *Connection) RecordPanelMatchLog(log PanelMatchLog) error {
	log.SourceKey = strings.TrimSpace(log.SourceKey)
	if log.SourceKey == "" {
		return nil
	}

	playersJSON, err := json.Marshal(log.Players)
	if err != nil {
		return err
	}

	if err := c.PrunePanelMatchLogs(); err != nil {
		return err
	}

	var hostProfileID any
	if log.HostProfileID != 0 {
		hostProfileID = int64(log.HostProfileID)
	}

	_, err = c.pool.Exec(c.ctx, `
		INSERT INTO public.panel_match_logs (
			source_key,
			room_id,
			dwc_group_id,
			game_name,
			match_type,
			mkw_region,
			race_number,
			course_id,
			engine_class_id,
			host_profile_id,
			host_ingame_name,
			player_count,
			players
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb)
		ON CONFLICT (source_key) DO UPDATE
		SET room_id = EXCLUDED.room_id,
		    dwc_group_id = EXCLUDED.dwc_group_id,
		    game_name = EXCLUDED.game_name,
		    match_type = EXCLUDED.match_type,
		    mkw_region = EXCLUDED.mkw_region,
		    race_number = EXCLUDED.race_number,
		    course_id = EXCLUDED.course_id,
		    engine_class_id = EXCLUDED.engine_class_id,
		    host_profile_id = EXCLUDED.host_profile_id,
		    host_ingame_name = EXCLUDED.host_ingame_name,
		    player_count = EXCLUDED.player_count,
		    players = EXCLUDED.players
	`, log.SourceKey,
		strings.TrimSpace(log.RoomID),
		int64(log.DWCGroupID),
		strings.TrimSpace(log.GameName),
		strings.TrimSpace(log.MatchType),
		strings.TrimSpace(log.MKWRegion),
		log.RaceNumber,
		log.CourseID,
		log.EngineClassID,
		hostProfileID,
		strings.TrimSpace(log.HostName),
		len(log.Players),
		string(playersJSON),
	)
	return err
}

func (c *Connection) ListPanelMatchLogs(limit int) ([]PanelMatchLog, error) {
	if err := c.PrunePanelMatchLogs(); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > 500 {
		limit = 200
	}

	rows, err := c.pool.Query(c.ctx, `
		SELECT id,
		       source_key,
		       room_id,
		       dwc_group_id,
		       game_name,
		       match_type,
		       mkw_region,
		       race_number,
		       course_id,
		       engine_class_id,
		       host_profile_id,
		       host_ingame_name,
		       player_count,
		       players,
		       created_at
		FROM public.panel_match_logs
		WHERE created_at >= CURRENT_TIMESTAMP - INTERVAL '14 days'
		ORDER BY created_at DESC, id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []PanelMatchLog{}
	for rows.Next() {
		log := PanelMatchLog{}
		var hostProfileID pgtype.Int8
		var playersJSON string
		var dwcGroupID int64
		if err := rows.Scan(
			&log.ID,
			&log.SourceKey,
			&log.RoomID,
			&dwcGroupID,
			&log.GameName,
			&log.MatchType,
			&log.MKWRegion,
			&log.RaceNumber,
			&log.CourseID,
			&log.EngineClassID,
			&hostProfileID,
			&log.HostName,
			&log.PlayerCount,
			&playersJSON,
			&log.CreatedAt,
		); err != nil {
			return nil, err
		}

		if dwcGroupID > 0 {
			log.DWCGroupID = uint64(dwcGroupID)
		}
		if hostProfileID.Status == pgtype.Present && hostProfileID.Int > 0 {
			log.HostProfileID = uint32(hostProfileID.Int)
		}
		if playersJSON != "" {
			if err := json.Unmarshal([]byte(playersJSON), &log.Players); err != nil {
				return nil, err
			}
		}

		logs = append(logs, log)
	}

	return logs, rows.Err()
}

func (c *Connection) PrunePanelMatchLogs() error {
	_, err := c.pool.Exec(c.ctx, `
		DELETE FROM public.panel_match_logs
		WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '14 days'
	`)
	return err
}

func (c *Connection) ListPanelEventLogs(fromTime, toTime time.Time, includePrivate bool, limit int) ([]PanelEventLog, error) {
	if err := c.PrunePanelMatchLogs(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 5000 {
		limit = 2000
	}

	windowStart := time.Now().UTC().Add(-14 * 24 * time.Hour)
	if fromTime.IsZero() || fromTime.Before(windowStart) {
		fromTime = windowStart
	}
	if toTime.IsZero() {
		toTime = time.Now().UTC().Add(24 * time.Hour)
	}
	if !toTime.After(fromTime) {
		return []PanelEventLog{}, nil
	}

	events := []PanelEventLog{}

	matchRows, err := c.pool.Query(c.ctx, `
		SELECT id, room_id, game_name, match_type, mkw_region, race_number, course_id, engine_class_id,
		       host_profile_id, host_ingame_name, player_count, COALESCE(players::text, ''), created_at
		FROM public.panel_match_logs
		WHERE created_at >= $1 AND created_at < $2
		ORDER BY created_at DESC, id DESC
		LIMIT $3
	`, fromTime, toTime, limit)
	if err != nil {
		return nil, err
	}
	for matchRows.Next() {
		var id int
		var roomID, gameName, matchType, region, hostName string
		var raceNum, course, cc, playerCount int
		var playersJSON string
		var hostPID pgtype.Int8
		var createdAt time.Time
		if err := matchRows.Scan(&id, &roomID, &gameName, &matchType, &region, &raceNum, &course, &cc,
			&hostPID, &hostName, &playerCount, &playersJSON, &createdAt); err != nil {
			matchRows.Close()
			return nil, err
		}
		players := []PanelMatchLogPlayer{}
		if playersJSON != "" {
			if err := json.Unmarshal([]byte(playersJSON), &players); err != nil {
				matchRows.Close()
				return nil, err
			}
		}
		parts := []string{}
		if gameName != "" {
			parts = append(parts, gameName)
		}
		if matchType != "" {
			parts = append(parts, matchType)
		}
		if raceNum > 0 {
			parts = append(parts, "race "+strconv.Itoa(raceNum))
		}
		if course >= 0 {
			parts = append(parts, "course "+strconv.Itoa(course))
		}
		if cc >= 0 {
			parts = append(parts, "class "+strconv.Itoa(cc))
		}
		if region != "" {
			parts = append(parts, "rk "+region)
		}
		parts = append(parts, strconv.Itoa(playerCount)+"p")
		if playerSummary := panelMatchPlayersSummary(players); playerSummary != "" {
			parts = append(parts, playerSummary)
		}
		summary := "Match in room " + roomID
		if hostName != "" {
			summary += " hosted by " + hostName
		} else if hostPID.Status == pgtype.Present && hostPID.Int > 0 {
			summary += " hosted by PID " + strconv.FormatInt(hostPID.Int, 10)
		}
		ev := PanelEventLog{
			ID:         "match-" + strconv.Itoa(id),
			Type:       "match",
			Level:      "info",
			CreatedAt:  createdAt,
			Summary:    summary,
			TargetName: hostName,
			RoomID:     roomID,
			Extra:      strings.Join(parts, " | "),
		}
		if hostPID.Status == pgtype.Present && hostPID.Int > 0 {
			ev.TargetPID = uint32(hostPID.Int)
		}
		events = append(events, ev)
	}
	matchRows.Close()

	banRows, err := c.pool.Query(c.ctx, `
		SELECT h.id,
		       h.profile_id,
		       COALESCE(NULLIF(h.gsbrcd, ''), COALESCE(u.gsbrcd, '')) AS gsbrcd,
		       COALESCE(NULLIF(h.ingame_name, ''), COALESCE(u.last_ingamesn, '')) AS ingame_name,
		       COALESCE(h.reason, ''),
		       COALESCE(h.reason_hidden, ''),
		       COALESCE(h.moderator, ''),
		       COALESCE(h.tos, false),
		       h.issued,
		       h.expires,
		       h.revoked
		FROM public.panel_ban_history h
		LEFT JOIN public.users u ON u.profile_id = h.profile_id
		WHERE (h.issued >= $1 AND h.issued < $2)
		   OR (h.revoked IS NOT NULL AND h.revoked >= $1 AND h.revoked < $2)
		ORDER BY COALESCE(h.revoked, h.issued) DESC, h.id DESC
		LIMIT $3
	`, fromTime, toTime, limit)
	if err != nil {
		return nil, err
	}
	for banRows.Next() {
		var id int
		var pid uint32
		var gsbrcd, name, reason, reasonHidden, moderator string
		var tos bool
		var issued pgtype.Timestamp
		var expires pgtype.Timestamp
		var revoked pgtype.Timestamp
		if err := banRows.Scan(&id, &pid, &gsbrcd, &name, &reason, &reasonHidden, &moderator, &tos,
			&issued, &expires, &revoked); err != nil {
			banRows.Close()
			return nil, err
		}
		fc := ""
		if len(gsbrcd) >= 4 {
			fc = common.CalcFriendCodeString(pid, gsbrcd[:4])
		}

		if issued.Status == pgtype.Present && !issued.Time.Before(fromTime) && issued.Time.Before(toTime) {
			label := "Ban"
			if tos {
				label = "TOS ban"
			}
			target := name
			if target == "" {
				target = "PID " + strconv.FormatUint(uint64(pid), 10)
			}
			summary := label + " issued on " + target
			if moderator != "" {
				summary += " by " + moderator
			}
			ev := PanelEventLog{
				ID:         "ban-" + strconv.Itoa(id),
				Type:       "ban",
				Level:      "danger",
				CreatedAt:  issued.Time,
				Summary:    summary,
				Moderator:  moderator,
				TargetPID:  pid,
				TargetName: name,
				TargetFC:   fc,
				Reason:     reason,
				TOS:        tos,
			}
			if expires.Status == pgtype.Present {
				t := expires.Time
				ev.Expires = &t
			}
			if includePrivate {
				ev.Note = reasonHidden
			}
			events = append(events, ev)
		}
		if revoked.Status == pgtype.Present && !revoked.Time.Before(fromTime) && revoked.Time.Before(toTime) {
			target := name
			if target == "" {
				target = "PID " + strconv.FormatUint(uint64(pid), 10)
			}
			summary := "Unban " + target
			if moderator != "" {
				summary += " (ban by " + moderator + ")"
			}
			events = append(events, PanelEventLog{
				ID:         "unban-" + strconv.Itoa(id),
				Type:       "unban",
				Level:      "success",
				CreatedAt:  revoked.Time,
				Summary:    summary,
				Moderator:  moderator,
				TargetPID:  pid,
				TargetName: name,
				TargetFC:   fc,
				Reason:     reason,
			})
		}
	}
	banRows.Close()

	kickRows, err := c.pool.Query(c.ctx, `
		SELECT k.id, k.room_id, COALESCE(k.game_name, ''), k.owner_profile_id, k.target_profile_id,
		       COALESCE(k.target_ingame_name, ''), COALESCE(k.target_friend_code, ''), COALESCE(k.reason, ''),
		       COALESCE(k.moderator, ''), k.created_at
		FROM public.panel_room_kick_history k
		WHERE k.created_at >= $1 AND k.created_at < $2
		ORDER BY k.created_at DESC, k.id DESC
		LIMIT $3
	`, fromTime, toTime, limit)
	if err != nil {
		return nil, err
	}
	for kickRows.Next() {
		var id int
		var roomID, gameName, targetName, targetFC, reason, moderator string
		var ownerPID, targetPID uint32
		var createdAt time.Time
		if err := kickRows.Scan(&id, &roomID, &gameName, &ownerPID, &targetPID,
			&targetName, &targetFC, &reason, &moderator, &createdAt); err != nil {
			kickRows.Close()
			return nil, err
		}
		target := targetName
		if target == "" {
			target = "PID " + strconv.FormatUint(uint64(targetPID), 10)
		}
		summary := "Room kick: " + target + " from room " + roomID
		if moderator != "" {
			summary += " by " + moderator
		}
		events = append(events, PanelEventLog{
			ID:         "kick-" + strconv.Itoa(id),
			Type:       "kick",
			Level:      "warn",
			CreatedAt:  createdAt,
			Summary:    summary,
			Moderator:  moderator,
			TargetPID:  targetPID,
			TargetName: targetName,
			TargetFC:   targetFC,
			RoomID:     roomID,
			Reason:     reason,
			Extra:      gameName,
		})
	}
	kickRows.Close()

	msgRows, err := c.pool.Query(c.ctx, `
		SELECT m.id, m.profile_id, COALESCE(u.last_ingamesn, ''), COALESCE(u.gsbrcd, ''),
		       m.message, COALESCE(m.moderator, ''), m.created_at, m.delivered_at
		FROM public.panel_login_messages m
		LEFT JOIN public.users u ON u.profile_id = m.profile_id
		WHERE m.created_at >= $1 AND m.created_at < $2
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT $3
	`, fromTime, toTime, limit)
	if err != nil {
		return nil, err
	}
	for msgRows.Next() {
		var id int
		var pid uint32
		var name, gsbrcd, message, moderator string
		var createdAt time.Time
		var deliveredAt pgtype.Timestamp
		if err := msgRows.Scan(&id, &pid, &name, &gsbrcd, &message, &moderator, &createdAt, &deliveredAt); err != nil {
			msgRows.Close()
			return nil, err
		}
		fc := ""
		if len(gsbrcd) >= 4 {
			fc = common.CalcFriendCodeString(pid, gsbrcd[:4])
		}
		target := name
		if target == "" {
			target = "PID " + strconv.FormatUint(uint64(pid), 10)
		}
		summary := "Login message to " + target
		if moderator != "" {
			summary += " from " + moderator
		}
		status := "pending"
		if deliveredAt.Status == pgtype.Present {
			status = "delivered"
		}
		events = append(events, PanelEventLog{
			ID:         "msg-" + strconv.Itoa(id),
			Type:       "message",
			Level:      "info",
			CreatedAt:  createdAt,
			Summary:    summary,
			Moderator:  moderator,
			TargetPID:  pid,
			TargetName: name,
			TargetFC:   fc,
			Reason:     message,
			Extra:      status,
		})
	}
	msgRows.Close()

	eventRows, err := c.pool.Query(c.ctx, `
		SELECT id, event_type, event_data::text, event_time
		FROM public.events
		WHERE event_time >= $1 AND event_time < $2
		  AND event_type IN (
		      'profile_created',
		      'logged_in',
		      'logged_out',
		      'received_login_info',
		      'device_authenticated',
		      'login_message_delivered',
		      'reported_bad_packet',
		      'reported_stall',
		      'room_player_reported',
		      'gpcm_returned_error',
		      'group_created',
		      'group_deleted',
		      'group_joined',
		      'group_left',
		      'group_host_changed',
		      'natneg_succeeded',
		      'natneg_failed',
		      'profile_kicked',
		      'profile_forced_room_join',
		      'matchmaking_room_created',
		      'matchmaking_room_deleted',
		      'matchmaking_room_game_end_attempted',
		      'matchmaking_room_game_end_result',
		      'matchmaking_room_game_end_fallback',
		      'profile_link_requested',
		      'profile_linked',
		      'profile_unlinked',
		      'panel_mkw_ranking_deleted',
		      'panel_mkw_profile_cleared',
		      'panel_mkw_ghost_downloaded',
		      'mkw_ghost_downloaded',
		      'mkw_ghost_uploaded',
		      'mkw_invalid_ghost_upload',
		      'disabled_game_added',
		      'disabled_game_updated',
		      'disabled_game_removed',
		      'server_update_scheduled',
		      'server_update_installing',
		      'server_updated',
		      'server_settings_changed',
		      'server_restart_requested'
		  )
		ORDER BY event_time DESC, id DESC
		LIMIT $3
	`, fromTime, toTime, limit)
	if err != nil {
		return nil, err
	}
	for eventRows.Next() {
		var id int
		var eventType, eventData string
		var eventTime time.Time
		if err := eventRows.Scan(&id, &eventType, &eventData, &eventTime); err != nil {
			eventRows.Close()
			return nil, err
		}
		if ev, ok := panelEventLogFromDatabaseEvent(id, eventType, eventData, eventTime); ok {
			events = append(events, ev)
		}
	}
	eventRows.Close()

	sort.Slice(events, func(i, j int) bool {
		if events[i].CreatedAt.Equal(events[j].CreatedAt) {
			return events[i].ID > events[j].ID
		}
		return events[i].CreatedAt.After(events[j].CreatedAt)
	})

	if len(events) > limit {
		events = events[:limit]
	}

	return events, nil
}

func panelMatchPlayersSummary(players []PanelMatchLogPlayer) string {
	if len(players) == 0 {
		return ""
	}

	summaries := make([]string, 0, len(players))
	for _, player := range players {
		label := strings.TrimSpace(player.InGameName)
		if label == "" && player.ProfileID != 0 {
			label = "PID " + strconv.FormatUint(uint64(player.ProfileID), 10)
		}
		if label == "" {
			label = "unknown"
		}

		details := []string{}
		if player.ProfileID != 0 {
			details = append(details, "pid="+strconv.FormatUint(uint64(player.ProfileID), 10))
		}
		if player.FriendCode != "" {
			details = append(details, "fc="+player.FriendCode)
		}
		if player.JoinIndex != "" {
			details = append(details, "slot="+player.JoinIndex)
		}
		if player.LocalPlayers != "" && player.LocalPlayers != "1" {
			details = append(details, "local="+player.LocalPlayers)
		}
		if player.ConnFail != "" && player.ConnFail != "0" {
			details = append(details, "conn_fail="+player.ConnFail)
		}
		if player.VersusELO != "" {
			details = append(details, "ev="+player.VersusELO)
		}
		if player.BattleELO != "" {
			details = append(details, "eb="+player.BattleELO)
		}
		if len(details) > 0 {
			label += " (" + strings.Join(details, ", ") + ")"
		}
		summaries = append(summaries, label)
	}

	return "players: " + strings.Join(summaries, "; ")
}

func panelEventLogFromDatabaseEvent(id int, eventType string, eventData string, eventTime time.Time) (PanelEventLog, bool) {
	data := map[string]any{}
	if eventData != "" {
		if err := json.Unmarshal([]byte(eventData), &data); err != nil {
			data["raw"] = eventData
		}
	}

	ev := PanelEventLog{
		ID:        "event-" + strconv.Itoa(id),
		Type:      "event",
		Level:     "info",
		CreatedAt: eventTime,
		Summary:   strings.ReplaceAll(eventType, "_", " "),
	}
	omit := []string{}

	setTargetPID := func(keys ...string) {
		if pid := panelEventUint32(data, keys...); pid != 0 {
			ev.TargetPID = pid
			omit = append(omit, keys...)
		}
	}
	setRoom := func(keys ...string) {
		if roomID := panelEventString(data, keys...); roomID != "" {
			ev.RoomID = roomID
			omit = append(omit, keys...)
		}
	}
	setModerator := func(keys ...string) {
		if moderator := panelEventString(data, keys...); moderator != "" {
			ev.Moderator = moderator
			omit = append(omit, keys...)
		}
	}
	setReason := func(keys ...string) {
		if reason := panelEventString(data, keys...); reason != "" {
			ev.Reason = reason
			omit = append(omit, keys...)
		}
	}

	switch eventType {
	case "profile_created":
		ev.Type = "session"
		ev.Level = "success"
		ev.Summary = "Profile created"
		setTargetPID("profile_id")
	case "received_login_info":
		ev.Type = "session"
		ev.Summary = "Login information received"
		ev.TargetName = panelEventString(data, "in_game_name")
		omit = append(omit, "in_game_name")
	case "logged_in":
		ev.Type = "session"
		ev.Summary = "Logged in"
		ev.TargetName = panelEventString(data, "in_game_name")
		omit = append(omit, "in_game_name")
		setTargetPID("profile_id")
	case "logged_out":
		ev.Type = "session"
		ev.Summary = "Logged out"
		setTargetPID("profile_id")
	case "device_authenticated":
		ev.Type = "session"
		ev.Summary = "Payload device authenticated"
		setTargetPID("profile_id")
	case "login_message_delivered":
		ev.Type = "message"
		ev.Summary = "Login message delivered"
		setTargetPID("profile_id")
		setModerator("moderator")
	case "reported_bad_packet":
		ev.Type = "security"
		ev.Level = "danger"
		ev.Summary = "Potential cheating: bad packet reported"
		if suspect := panelEventString(data, "sender_id"); suspect != "" {
			ev.Summary += " against PID " + suspect
		}
		setTargetPID("sender_id")
	case "reported_stall":
		ev.Type = "security"
		ev.Level = "warn"
		ev.Summary = "Potential cheating: room stall reported"
		if suspect := panelEventString(data, "stalling_id"); suspect != "" {
			ev.Summary += " against PID " + suspect
		}
		setTargetPID("stalling_id")
	case "room_player_reported":
		ev.Type = "security"
		ev.Level = "warn"
		ev.Summary = "Room player reported"
		setTargetPID("profile_id")
		setRoom("room_id")
		setReason("reason")
		setModerator("account")
	case "gpcm_returned_error":
		ev.Type = "error"
		ev.Level = "warn"
		if panelEventString(data, "fatal") == "true" {
			ev.Level = "danger"
		}
		code := panelEventString(data, "dwc_error_code")
		if code == "" || code == "0" {
			code = panelEventString(data, "error_code")
		}
		ev.Summary = "GPCM error returned"
		if code != "" {
			ev.Summary += ": " + code
		}
		setTargetPID("profile_id")
		setReason("error_string")
	case "group_created":
		ev.Type = "match"
		ev.Summary = "Match group created"
		setRoom("group_name")
		setTargetPID("host_id")
	case "group_deleted":
		ev.Type = "match"
		ev.Summary = "Match group deleted"
		setRoom("group_name")
		setReason("reason")
	case "group_joined":
		ev.Type = "match"
		ev.Summary = "Player joined match group"
		setRoom("group_name")
		setTargetPID("profile_id")
	case "group_left":
		ev.Type = "match"
		ev.Summary = "Player left match group"
		setRoom("group_name")
		setTargetPID("profile_id")
	case "group_host_changed":
		ev.Type = "match"
		ev.Summary = "Match host changed"
		setRoom("group_name")
		setTargetPID("new_host_id")
	case "natneg_succeeded":
		ev.Type = "network"
		ev.Summary = "NAT negotiation succeeded"
		setRoom("group_name")
	case "natneg_failed":
		ev.Type = "network"
		ev.Level = "warn"
		ev.Summary = "NAT negotiation failed"
		setRoom("group_name")
	case "profile_kicked":
		ev.Type = "kick"
		ev.Level = "warn"
		ev.Summary = "Profile kicked"
		setTargetPID("profile_id")
		setModerator("moderator")
		setReason("reason")
	case "profile_forced_room_join":
		ev.Type = "panel"
		ev.Level = "warn"
		ev.Summary = "Forced room join queued"
		setTargetPID("profile_id")
		setRoom("room_id")
		setModerator("moderator")
	case "matchmaking_room_created":
		ev.Type = "panel"
		ev.Summary = "Matchmaking room created"
		setTargetPID("profile_id")
		setRoom("room_id")
		setModerator("moderator")
	case "matchmaking_room_deleted":
		ev.Type = "panel"
		ev.Level = "warn"
		ev.Summary = "Matchmaking room deleted"
		setRoom("room_id")
		setModerator("moderator")
	case "matchmaking_room_game_end_attempted":
		ev.Type = "panel"
		ev.Level = "warn"
		ev.Summary = "Started game end attempted"
		setRoom("room_id")
		setModerator("moderator")
	case "matchmaking_room_game_end_result":
		ev.Type = "panel"
		ev.Summary = "Race end result: " + panelEventString(data, "result")
		if panelEventString(data, "result") == "finished" {
			ev.Level = "success"
		}
		setTargetPID("profile_id")
		setRoom("room_id")
	case "matchmaking_room_game_end_fallback":
		ev.Type = "panel"
		ev.Level = "warn"
		ev.Summary = "Race end not confirmed, peer links closed"
		setRoom("room_id")
	case "profile_link_requested":
		ev.Type = "panel"
		ev.Summary = "Profile link code requested"
		setTargetPID("profile_id")
		setModerator("account")
	case "profile_linked":
		ev.Type = "panel"
		ev.Level = "success"
		ev.Summary = "Panel account linked to profile"
		setTargetPID("profile_id")
		setModerator("account")
	case "profile_unlinked":
		ev.Type = "panel"
		ev.Level = "warn"
		ev.Summary = "Panel account unlinked from profile"
		setTargetPID("profile_id")
		setModerator("account")
	case "panel_mkw_ranking_deleted":
		ev.Type = "ghost"
		ev.Level = "warn"
		ev.Summary = "Mario Kart Wii ranking deleted"
		setModerator("moderator")
	case "panel_mkw_profile_cleared":
		ev.Type = "ghost"
		ev.Level = "danger"
		ev.Summary = "Mario Kart Wii profile rankings cleared"
		setTargetPID("profile_id")
		setModerator("moderator")
	case "panel_mkw_ghost_downloaded":
		ev.Type = "ghost"
		ev.Summary = "Moderator downloaded Mario Kart Wii ghost"
		setTargetPID("profile_id")
		setModerator("moderator")
	case "mkw_ghost_downloaded":
		ev.Type = "ghost"
		ev.Summary = "Mario Kart Wii ghost downloaded by game client"
		setTargetPID("profile_id")
	case "mkw_ghost_uploaded":
		ev.Type = "ghost"
		ev.Level = "success"
		ev.Summary = "Mario Kart Wii ghost uploaded"
		setTargetPID("profile_id")
	case "disabled_game_added":
		ev.Type = "panel"
		ev.Level = "warn"
		ev.Summary = "Game disabled"
		setModerator("moderator")
	case "disabled_game_updated":
		ev.Type = "panel"
		ev.Summary = "Disabled game updated"
		setModerator("moderator")
	case "disabled_game_removed":
		ev.Type = "panel"
		ev.Level = "success"
		ev.Summary = "Game re-enabled"
		setModerator("moderator")
	case "server_update_scheduled":
		ev.Type = "panel"
		ev.Summary = "Server update scheduled"
		setModerator("moderator")
	case "server_update_installing":
		ev.Type = "panel"
		ev.Level = "warn"
		ev.Summary = "Installing server update, restarting"
	case "server_settings_changed":
		ev.Type = "panel"
		ev.Summary = "Server settings changed"
		setModerator("moderator")
	case "server_restart_requested":
		ev.Type = "panel"
		ev.Level = "warn"
		ev.Summary = "Server restart requested"
		setModerator("moderator")
	case "server_updated":
		ev.Type = "panel"
		ev.Level = "success"
		ev.Summary = "Server updated"
	case "mkw_invalid_ghost_upload":
		ev.Type = "security"
		ev.Level = "danger"
		ev.Summary = "Potential cheating: invalid Mario Kart Wii ghost upload"
		setTargetPID("profile_id")
	}

	ev.Extra = panelEventExtra(data, omit...)
	return ev, true
}

func panelEventString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := data[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			return strings.TrimSpace(v)
		case bool:
			return strconv.FormatBool(v)
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case json.Number:
			return v.String()
		default:
			encoded, err := json.Marshal(v)
			if err != nil {
				continue
			}
			return string(encoded)
		}
	}

	return ""
}

func panelEventUint32(data map[string]any, keys ...string) uint32 {
	for _, key := range keys {
		value := panelEventString(data, key)
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err == nil {
			return uint32(parsed)
		}
	}

	return 0
}

func panelEventExtra(data map[string]any, omitted ...string) string {
	if len(data) == 0 {
		return ""
	}

	omit := map[string]bool{}
	for _, key := range omitted {
		omit[key] = true
	}

	keys := make([]string, 0, len(data))
	for key := range data {
		if !omit[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	parts := []string{}
	for _, key := range keys {
		value := panelEventString(data, key)
		if value != "" {
			parts = append(parts, key+"="+value)
		}
	}

	return strings.Join(parts, " | ")
}

func (c *Connection) RecordBanHistory(profileID uint32, tos bool, issuedAt time.Time, expiresAt *time.Time, reason string, reasonHidden string, moderator string, inGameName string) error {
	_, err := c.pool.Exec(c.ctx, `
		UPDATE public.panel_ban_history
		SET expires = $2
		WHERE profile_id = $1
		  AND revoked IS NULL
		  AND (expires IS NULL OR expires > $2)
	`, profileID, issuedAt)
	if err != nil {
		return err
	}

	var expiresValue any
	if expiresAt != nil {
		expiresValue = *expiresAt
	}

	_, err = c.pool.Exec(c.ctx, `
		INSERT INTO public.panel_ban_history (
			profile_id, gsbrcd, ingame_name, reason, reason_hidden, moderator, tos, issued, expires
		)
		SELECT u.profile_id,
		       COALESCE(u.gsbrcd, ''),
		       COALESCE(NULLIF($8, ''), COALESCE(u.last_ingamesn, '')),
		       $4,
		       $5,
		       $6,
		       $7,
		       $2,
		       $3
		FROM public.users u
		WHERE u.profile_id = $1
	`, profileID, issuedAt, expiresValue, reason, reasonHidden, moderator, tos, strings.TrimSpace(inGameName))
	return err
}

// DeleteLatestBanHistory removes the profile's ban that is being lifted.
func (c *Connection) DeleteLatestBanHistory(profileID uint32) error {
	_, err := c.pool.Exec(c.ctx, `
		DELETE FROM public.panel_ban_history
		WHERE id = (
			SELECT id
			FROM public.panel_ban_history
			WHERE profile_id = $1
			  AND revoked IS NULL
			  AND (expires IS NULL OR expires > CURRENT_TIMESTAMP)
			ORDER BY issued DESC, id DESC
			LIMIT 1
		)
	`, profileID)
	return err
}

func (c *Connection) RevokeLatestBanHistory(profileID uint32, revokedAt time.Time) error {
	_, err := c.pool.Exec(c.ctx, `
		UPDATE public.panel_ban_history
		SET revoked = $2
		WHERE id = (
			SELECT id
			FROM public.panel_ban_history
			WHERE profile_id = $1
			  AND revoked IS NULL
			  AND (expires IS NULL OR expires > $2)
			ORDER BY issued DESC, id DESC
			LIMIT 1
		)
	`, profileID, revokedAt)
	return err
}

type ConsoleBanRecord struct {
	HasBan       bool
	TOS          bool
	Issued       *time.Time
	Expires      *time.Time
	Revoked      *time.Time
	Reason       string
	ReasonHidden string
	Moderator    string
}

type ConsoleSummary struct {
	ID                int                 `json:"id"`
	NgDeviceID        uint32              `json:"-"`
	NgDeviceIDHex     string              `json:"-"`
	AccountID         uint32              `json:"account_id,omitempty"`
	PrimaryProfileID  uint32              `json:"primary_profile_id,omitempty"`
	ConsoleType       string              `json:"type"`
	ConsoleFriendCode string              `json:"console_fc,omitempty"`
	DisplayName       string              `json:"placeholder,omitempty"`
	HasBan            bool                `json:"has_ban"`
	BanReason         string              `json:"ban_reason,omitempty"`
	BanExpires        *time.Time          `json:"ban_expires,omitempty"`
	LinkedAt          *time.Time          `json:"linked_at,omitempty"`
	AssignedAt        *time.Time          `json:"assigned_at,omitempty"`
	LastLoginAt       *time.Time          `json:"last_login,omitempty"`
	CreatedAt         time.Time           `json:"created"`
	Profiles          []ConsoleProfileRow `json:"profiles"`
}

type ConsoleProfileRow struct {
	ProfileID  uint32 `json:"pid"`
	FriendCode string `json:"fc,omitempty"`
	InGameName string `json:"name,omitempty"`
	IsPrimary  bool   `json:"primary"`
	HasBan     bool   `json:"has_ban"`
	OpenHost   bool   `json:"open_host"`
}

// EnsureConsoleByDeviceID inserts a row for the given NG device ID if one
// does not exist and returns the console row id. Device id 0 is treated as
// "no console" and returns (0, false, nil).
func (c *Connection) EnsureConsoleByDeviceID(deviceID uint32) (int, bool, error) {
	if deviceID == 0 {
		return 0, false, nil
	}
	var id int
	err := c.pool.QueryRow(c.ctx, `
		INSERT INTO public.consoles (ng_device_id)
		VALUES ($1)
		ON CONFLICT (ng_device_id) DO UPDATE SET ng_device_id = EXCLUDED.ng_device_id
		RETURNING id
	`, int64(deviceID)).Scan(&id)
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// AttachUserToConsole sets users.console_id when not already set. Best-effort.
func (c *Connection) AttachUserToConsole(profileID uint32, consoleID int) error {
	if consoleID <= 0 {
		return nil
	}
	_, err := c.pool.Exec(c.ctx, `
		UPDATE public.users
		SET console_id = $2
		WHERE profile_id = $1
		  AND (console_id IS NULL OR console_id <> $2)
	`, profileID, consoleID)
	return err
}

func (c *Connection) TouchConsoleLogin(consoleID int, consoleType string, consoleFriendCode uint64) error {
	if consoleID <= 0 {
		return nil
	}
	consoleType = strings.TrimSpace(consoleType)
	if consoleType == "" {
		consoleType = "Wii"
	}
	consoleFriendCodeValue := ""
	if consoleFriendCode != 0 {
		consoleFriendCodeValue = strconv.FormatUint(consoleFriendCode, 10)
	}
	_, err := c.pool.Exec(c.ctx, `
		UPDATE public.consoles
		SET last_login_at = CURRENT_TIMESTAMP,
		    console_type = NULLIF($2, ''),
		    console_friend_code = CASE WHEN $3 <> '' THEN $3 ELSE console_friend_code END
		WHERE id = $1
	`, consoleID, consoleType, consoleFriendCodeValue)
	return err
}

func (c *Connection) GetConsoleIDForProfile(profileID uint32) (int, error) {
	var id pgtype.Int4
	err := c.pool.QueryRow(c.ctx, `
		SELECT console_id FROM public.users WHERE profile_id = $1
	`, profileID).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	if id.Status != pgtype.Present {
		return 0, nil
	}
	return int(id.Int), nil
}

// ResolveConsoleForProfile returns a consoles.id for the given profile by
// inspecting users.console_id first, then falling back to the profile's first
// non-zero NG device ID. May lazily create a console row.
func (c *Connection) ResolveConsoleForProfile(profileID uint32) (int, error) {
	if id, err := c.GetConsoleIDForProfile(profileID); err != nil {
		return 0, err
	} else if id > 0 {
		return id, nil
	}

	var devices []uint32
	err := c.pool.QueryRow(c.ctx, `
		SELECT COALESCE(ng_device_id, '{}')
		FROM public.users
		WHERE profile_id = $1
	`, profileID).Scan(&devices)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}

	for _, d := range devices {
		if d == 0 {
			continue
		}
		consoleID, ok, err := c.EnsureConsoleByDeviceID(d)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		_ = c.AttachUserToConsole(profileID, consoleID)
		return consoleID, nil
	}

	return 0, nil
}

func (c *Connection) BanConsole(consoleID int, profileID uint32, tos bool, expiresAt *time.Time, reason string, reasonHidden string, moderator string, inGameName string) error {
	issuedAt := time.Now().UTC()
	var expiresValue any
	if expiresAt != nil {
		expiresValue = *expiresAt
	}
	_, err := c.pool.Exec(c.ctx, `
		UPDATE public.consoles
		SET has_ban = true,
		    ban_tos = $2,
		    ban_issued = $3,
		    ban_expires = $4,
		    ban_revoked = NULL,
		    ban_reason = $5,
		    ban_reason_hidden = $6,
		    ban_moderator = $7,
		    ban_ingame_name = NULLIF($8, ''),
		    ban_profile_id = NULLIF($9, 0)
		WHERE id = $1
	`, consoleID, tos, issuedAt, expiresValue, reason, reasonHidden, moderator, strings.TrimSpace(inGameName), int64(profileID))
	if err != nil {
		return err
	}

	// Mirror to every user attached to this console so legacy SearchUserBan and
	// existing UI keep showing the ban while we migrate code paths.
	_, err = c.pool.Exec(c.ctx, `
		UPDATE public.users
		SET has_ban = true,
		    ban_issued = $2,
		    ban_expires = $3,
		    ban_revoked = NULL,
		    ban_reason = $4,
		    ban_reason_hidden = $5,
		    ban_moderator = $6,
		    ban_tos = $7,
		    last_ingamesn = CASE WHEN $8 <> '' THEN $8 ELSE last_ingamesn END
		WHERE console_id = $1
	`, consoleID, issuedAt, expiresValue, reason, reasonHidden, moderator, tos, strings.TrimSpace(inGameName))
	if err != nil {
		return err
	}

	_, err = c.pool.Exec(c.ctx, `
		INSERT INTO public.panel_ban_history
			(profile_id, console_id, ng_device_id, gsbrcd, ingame_name, reason, reason_hidden, moderator, tos, issued, expires)
		SELECT
			COALESCE(NULLIF($2, 0), 0),
			$1,
			c.ng_device_id,
			COALESCE(u.gsbrcd, ''),
			COALESCE(NULLIF($8, ''), COALESCE(u.last_ingamesn, '')),
			$3,
			$4,
			$5,
			$6,
			$7,
			$9
		FROM public.consoles c
		LEFT JOIN public.users u ON u.profile_id = NULLIF($2, 0)
		WHERE c.id = $1
	`, consoleID, int64(profileID), reason, reasonHidden, moderator, tos, issuedAt, strings.TrimSpace(inGameName), expiresValue)
	return err
}

func (c *Connection) UnbanConsole(consoleID int) (bool, error) {
	if !common.GetConfig().KeepRevokedBans {
		return c.deleteConsoleBan(consoleID)
	}

	revokedAt := time.Now().UTC()
	tag, err := c.pool.Exec(c.ctx, `
		UPDATE public.consoles
		SET has_ban = false,
		    ban_revoked = $2
		WHERE id = $1
		  AND (has_ban = true OR ban_issued IS NOT NULL)
	`, consoleID, revokedAt)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	_, err = c.pool.Exec(c.ctx, `
		UPDATE public.users
		SET has_ban = false,
		    ban_revoked = $2
		WHERE console_id = $1
		  AND (has_ban = true OR ban_issued IS NOT NULL)
	`, consoleID, revokedAt)
	if err != nil {
		return false, err
	}

	_, err = c.pool.Exec(c.ctx, `
		UPDATE public.panel_ban_history
		SET revoked = $2
		WHERE id = (
			SELECT id FROM public.panel_ban_history
			WHERE console_id = $1
			  AND revoked IS NULL
			  AND (expires IS NULL OR expires > $2)
			ORDER BY issued DESC, id DESC
			LIMIT 1
		)
	`, consoleID, revokedAt)
	return true, err
}

// deleteConsoleBan lifts a console ban without keeping it as revoked: the ban
// fields are cleared and the lifted ban is removed from the history.
func (c *Connection) deleteConsoleBan(consoleID int) (bool, error) {
	tx, err := c.pool.Begin(c.ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(c.ctx) }()

	tag, err := tx.Exec(c.ctx, `
		UPDATE public.consoles
		SET has_ban = false,
		    ban_tos = NULL,
		    ban_issued = NULL,
		    ban_expires = NULL,
		    ban_revoked = NULL,
		    ban_reason = NULL,
		    ban_reason_hidden = NULL,
		    ban_moderator = NULL,
		    ban_ingame_name = NULL,
		    ban_profile_id = NULL
		WHERE id = $1
		  AND has_ban = true
	`, consoleID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	if _, err := tx.Exec(c.ctx, clearUserBanQuery+` WHERE console_id = $1 AND has_ban = true`, consoleID); err != nil {
		return false, err
	}

	if _, err := tx.Exec(c.ctx, `
		DELETE FROM public.panel_ban_history
		WHERE id = (
			SELECT id FROM public.panel_ban_history
			WHERE console_id = $1
			  AND revoked IS NULL
			  AND (expires IS NULL OR expires > CURRENT_TIMESTAMP)
			ORDER BY issued DESC, id DESC
			LIMIT 1
		)
	`, consoleID); err != nil {
		return false, err
	}

	return true, tx.Commit(c.ctx)
}

// LinkPanelAccountConsole assigns a console to a panel account. If another
// account already owns the console returns (account, false, nil).
func (c *Connection) LinkPanelAccountConsole(accountID int, consoleID int, primaryProfileID uint32) (PanelAccount, bool, error) {
	tx, err := c.pool.Begin(c.ctx)
	if err != nil {
		return PanelAccount{}, false, err
	}
	defer tx.Rollback(c.ctx)

	var existingAccount pgtype.Int4
	err = tx.QueryRow(c.ctx, `SELECT account_id FROM public.consoles WHERE id = $1`, consoleID).Scan(&existingAccount)
	if err != nil {
		return PanelAccount{}, false, err
	}
	if existingAccount.Status == pgtype.Present && int(existingAccount.Int) != 0 && int(existingAccount.Int) != accountID {
		return PanelAccount{}, false, nil
	}

	_, err = tx.Exec(c.ctx, `
		UPDATE public.consoles
		SET account_id = $2,
		    primary_profile_id = NULLIF($3, 0),
		    linked_at = CASE
		        WHEN account_id IS DISTINCT FROM $2 OR linked_at IS NULL THEN CURRENT_TIMESTAMP
		        ELSE linked_at
		    END
		WHERE id = $1
	`, consoleID, accountID, int64(primaryProfileID))
	if err != nil {
		return PanelAccount{}, false, err
	}

	_, err = tx.Exec(c.ctx, `
		UPDATE public.panel_accounts
		SET console_id = $2,
		    profile_id = NULLIF($3, 0),
		    linked_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, accountID, consoleID, int64(primaryProfileID))
	if err != nil {
		return PanelAccount{}, false, err
	}

	if err := tx.Commit(c.ctx); err != nil {
		return PanelAccount{}, false, err
	}

	account, _, err := c.GetPanelAccountByID(accountID)
	return account, true, err
}

func (c *Connection) UnlinkPanelAccountConsole(accountID int, consoleID int) (PanelAccount, bool, error) {
	tx, err := c.pool.Begin(c.ctx)
	if err != nil {
		return PanelAccount{}, false, err
	}
	defer tx.Rollback(c.ctx)

	rowsAffected := int64(0)
	if consoleID > 0 {
		tag, execErr := tx.Exec(c.ctx, `
			UPDATE public.consoles
			SET account_id = NULL,
			    primary_profile_id = NULL,
			    display_name = '',
			    linked_at = NULL
			WHERE account_id = $1
			  AND id = $2
		`, accountID, consoleID)
		err = execErr
		rowsAffected = tag.RowsAffected()
	} else {
		tag, execErr := tx.Exec(c.ctx, `
			UPDATE public.consoles
			SET account_id = NULL,
			    primary_profile_id = NULL,
			    display_name = '',
			    linked_at = NULL
			WHERE account_id = $1
		`, accountID)
		err = execErr
		rowsAffected = tag.RowsAffected()
	}
	if err != nil {
		return PanelAccount{}, false, err
	}
	if rowsAffected == 0 {
		return PanelAccount{}, false, nil
	}

	if err := c.refreshPanelAccountPrimaryConsoleTx(tx, accountID); err != nil {
		return PanelAccount{}, false, err
	}

	if err := tx.Commit(c.ctx); err != nil {
		return PanelAccount{}, false, err
	}

	account, ok, err := c.GetPanelAccountByID(accountID)
	return account, ok, err
}

func (c *Connection) refreshPanelAccountPrimaryConsoleTx(tx pgx.Tx, accountID int) error {
	var consoleID int
	var profileID int64
	var linkedAt pgtype.Timestamp
	err := tx.QueryRow(c.ctx, `
		SELECT c.id,
		       COALESCE(c.primary_profile_id, first_profile.profile_id, 0),
		       c.linked_at
		FROM public.consoles c
		LEFT JOIN LATERAL (
			SELECT u.profile_id
			FROM public.users u
			WHERE u.console_id = c.id
			ORDER BY u.profile_id
			LIMIT 1
		) first_profile ON true
		WHERE c.account_id = $1
		ORDER BY CASE WHEN c.primary_profile_id IS NOT NULL THEN 0 ELSE 1 END,
		         c.linked_at DESC NULLS LAST,
		         c.id
		LIMIT 1
	`, accountID).Scan(&consoleID, &profileID, &linkedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			_, err = tx.Exec(c.ctx, `
				UPDATE public.panel_accounts
				SET console_id = NULL,
				    profile_id = NULL,
				    linked_at = NULL
				WHERE id = $1
			`, accountID)
			return err
		}
		return err
	}

	var linkedValue any
	if linkedAt.Status == pgtype.Present {
		linkedValue = linkedAt.Time
	}
	_, err = tx.Exec(c.ctx, `
		UPDATE public.panel_accounts
		SET console_id = $2,
		    profile_id = NULLIF($3, 0),
		    linked_at = $4
		WHERE id = $1
	`, accountID, consoleID, profileID, linkedValue)
	return err
}

func (c *Connection) SetConsolePrimaryProfile(accountID int, profileID uint32) error {
	_, err := c.pool.Exec(c.ctx, `
		WITH target AS (
			SELECT c.id
			FROM public.consoles c
			JOIN public.users u ON u.console_id = c.id
			WHERE c.account_id = $1
			  AND u.profile_id = $2
			LIMIT 1
		),
		updated AS (
			UPDATE public.consoles c
			SET primary_profile_id = $2
			FROM target
			WHERE c.id = target.id
			RETURNING c.id
		)
		UPDATE public.panel_accounts a
		SET console_id = updated.id,
		    profile_id = $2,
		    linked_at = COALESCE(a.linked_at, CURRENT_TIMESTAMP)
		FROM updated
		WHERE a.id = $1
	`, accountID, int64(profileID))
	return err
}

func (c *Connection) GetConsoleSummaryForAccount(accountID int) (ConsoleSummary, bool, error) {
	summaries, err := c.ListConsoleSummariesForAccount(accountID)
	if err != nil {
		return ConsoleSummary{}, false, err
	}
	if len(summaries) == 0 {
		return ConsoleSummary{}, false, nil
	}
	return summaries[0], true, nil
}

func (c *Connection) ListConsoleSummariesForAccount(accountID int) ([]ConsoleSummary, error) {
	rows, err := c.pool.Query(c.ctx, `
		SELECT c.id,
		       c.ng_device_id,
		       COALESCE(c.account_id, 0),
		       c.primary_profile_id,
		       COALESCE(c.console_type, 'Wii'),
		       COALESCE(c.console_friend_code, ''),
		       COALESCE(c.display_name, ''),
		       c.has_ban,
		       COALESCE(c.ban_reason, ''),
		       c.ban_expires,
		       c.linked_at,
		       c.last_login_at,
		       c.created_at
		FROM public.consoles c
		LEFT JOIN public.panel_accounts a ON a.id = $1
		WHERE c.account_id = $1
		ORDER BY CASE WHEN a.console_id = c.id THEN 0 ELSE 1 END,
		         c.linked_at DESC NULLS LAST,
		         c.id
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := []ConsoleSummary{}
	for rows.Next() {
		summary := ConsoleSummary{}
		var ngDeviceID int64
		var accountIDValue int64
		var primary pgtype.Int8
		var banReason pgtype.Text
		var banExpires pgtype.Timestamp
		var linkedAt pgtype.Timestamp
		var lastLoginAt pgtype.Timestamp
		err := rows.Scan(
			&summary.ID,
			&ngDeviceID,
			&accountIDValue,
			&primary,
			&summary.ConsoleType,
			&summary.ConsoleFriendCode,
			&summary.DisplayName,
			&summary.HasBan,
			&banReason,
			&banExpires,
			&linkedAt,
			&lastLoginAt,
			&summary.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		summary.NgDeviceID = uint32(ngDeviceID)
		summary.NgDeviceIDHex = strings.ToUpper(strconv.FormatInt(ngDeviceID, 16))
		summary.AccountID = uint32(accountIDValue)
		if primary.Status == pgtype.Present {
			summary.PrimaryProfileID = uint32(primary.Int)
		}
		if banReason.Status == pgtype.Present {
			summary.BanReason = banReason.String
		}
		if banExpires.Status == pgtype.Present {
			t := banExpires.Time
			summary.BanExpires = &t
		}
		if linkedAt.Status == pgtype.Present {
			t := linkedAt.Time
			summary.LinkedAt = &t
			summary.AssignedAt = &t
		}
		if lastLoginAt.Status == pgtype.Present {
			t := lastLoginAt.Time
			summary.LastLoginAt = &t
		}
		if strings.TrimSpace(summary.ConsoleType) == "" {
			summary.ConsoleType = "Wii"
		}

		if err := c.loadConsoleSummaryProfiles(&summary); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, rows.Err()
}

func (c *Connection) loadConsoleSummaryProfiles(summary *ConsoleSummary) error {
	storedConsoleFriendCode := strings.TrimSpace(summary.ConsoleFriendCode)
	rows, err := c.pool.Query(c.ctx, `
		SELECT u.profile_id, COALESCE(u.gsbrcd, ''), COALESCE(u.last_ingamesn, ''),
		       u.has_ban, COALESCE(u.open_host, true)
		FROM public.users u
		WHERE u.console_id = $1
		ORDER BY u.profile_id
	`, summary.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var pid uint32
		var gsbrcd, name string
		var hasBan, openHost bool
		if err := rows.Scan(&pid, &gsbrcd, &name, &hasBan, &openHost); err != nil {
			return err
		}
		fc := ""
		if len(gsbrcd) >= 4 {
			fc = common.CalcFriendCodeString(pid, gsbrcd[:4])
		}
		row := ConsoleProfileRow{
			ProfileID:  pid,
			FriendCode: fc,
			InGameName: name,
			IsPrimary:  pid == summary.PrimaryProfileID,
			HasBan:     hasBan,
			OpenHost:   openHost,
		}
		if storedConsoleFriendCode == "" {
			if summary.ConsoleFriendCode == "" && fc != "" {
				summary.ConsoleFriendCode = fc
			}
			if row.IsPrimary && fc != "" {
				summary.ConsoleFriendCode = fc
			}
		}
		summary.Profiles = append(summary.Profiles, row)
	}
	return rows.Err()
}

func (c *Connection) SetConsoleDisplayName(accountID int, consoleID int, displayName string) (bool, error) {
	displayName = strings.TrimSpace(displayName)
	tag, err := c.pool.Exec(c.ctx, `
		UPDATE public.consoles
		SET display_name = $3
		WHERE account_id = $1
		  AND id = $2
	`, accountID, consoleID, displayName)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (c *Connection) ListPanelAccountProfileIDs(accountID int) ([]uint32, error) {
	rows, err := c.pool.Query(c.ctx, `
		WITH linked_profiles AS (
			SELECT u.profile_id
			FROM public.consoles c
			JOIN public.users u ON u.console_id = c.id
			WHERE c.account_id = $1
			UNION
			SELECT a.profile_id
			FROM public.panel_accounts a
			WHERE a.id = $1
			  AND a.profile_id IS NOT NULL
		)
		SELECT DISTINCT profile_id
		FROM linked_profiles
		WHERE profile_id IS NOT NULL
		ORDER BY profile_id
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profileIDs := []uint32{}
	for rows.Next() {
		var profileID uint32
		if err := rows.Scan(&profileID); err != nil {
			return nil, err
		}
		profileIDs = append(profileIDs, profileID)
	}
	return profileIDs, rows.Err()
}

func (c *Connection) ResolveProfileIdentifier(identifier string) (uint32, bool, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return 0, false, nil
	}

	friendCode := normalizeFriendCode(identifier)
	if friendCode != "" && (friendCode != identifier || len(friendCode) == 12) {
		return c.resolveProfileIDByFriendCode(friendCode)
	}

	if profileID, err := strconv.ParseUint(identifier, 10, 32); err == nil {
		if profileID == 0 {
			return 0, false, nil
		}

		return uint32(profileID), true, nil
	}

	if friendCode == "" {
		return 0, false, nil
	}

	return c.resolveProfileIDByFriendCode(friendCode)
}

func (c *Connection) resolveProfileIDByFriendCode(friendCode string) (uint32, bool, error) {
	rows, err := c.pool.Query(c.ctx, `
		SELECT profile_id, COALESCE(gsbrcd, '')
		FROM public.users
		WHERE COALESCE(gsbrcd, '') <> ''
		ORDER BY profile_id
	`)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()

	for rows.Next() {
		var profileID uint32
		var gsbrcd string
		if err := rows.Scan(&profileID, &gsbrcd); err != nil {
			return 0, false, err
		}
		if len(gsbrcd) < 4 {
			continue
		}
		if normalizeFriendCode(common.CalcFriendCodeString(profileID, gsbrcd[:4])) == friendCode {
			return profileID, true, nil
		}
	}

	if err := rows.Err(); err != nil {
		return 0, false, err
	}

	return 0, false, nil
}

func normalizeFriendCode(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, ch := range value {
		if ch >= '0' && ch <= '9' {
			builder.WriteRune(ch)
		}
	}

	return builder.String()
}
