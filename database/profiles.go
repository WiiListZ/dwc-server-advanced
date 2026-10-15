package database

import (
	"strings"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
)

// SiteProfile is the public profile of a site account.
type SiteProfile struct {
	AccountID     int        `json:"-"`
	Username      string     `json:"username"`
	Role          string     `json:"role"`
	Created       time.Time  `json:"created"`
	Bio           string     `json:"bio"`
	HasAvatar     bool       `json:"has_avatar"`
	AvatarUpdated *time.Time `json:"avatar_updated,omitempty"`
	ShowStats     bool       `json:"show_stats"`
}

// ProfileStats are computed from the game profiles linked to an account.
type ProfileStats struct {
	Matches      int        `json:"matches"`
	LastMatch    *time.Time `json:"last_match,omitempty"`
	Games        []string   `json:"games"`
	VersusRating *int       `json:"vr,omitempty"`
	BattleRating *int       `json:"br,omitempty"`
	TimeTrials   int        `json:"time_trials"`
}

func (c *Connection) UpdateProfilesTable() error {
	_, err := c.pool.Exec(c.ctx, `
		CREATE TABLE IF NOT EXISTS public.panel_profiles (
			account_id integer PRIMARY KEY REFERENCES public.panel_accounts(id) ON DELETE CASCADE,
			bio text NOT NULL DEFAULT '',
			avatar bytea,
			avatar_updated timestamp without time zone,
			show_stats boolean NOT NULL DEFAULT true,
			updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

const siteProfileQuery = `
	SELECT a.id, a.username, a.role, a.created_at,
		COALESCE(p.bio, ''), p.avatar IS NOT NULL, p.avatar_updated, COALESCE(p.show_stats, true)
	FROM public.panel_accounts a
	LEFT JOIN public.panel_profiles p ON p.account_id = a.id
`

func scanSiteProfile(row pgx.Row) (SiteProfile, error) {
	profile := SiteProfile{}
	var created, avatarUpdated pgtype.Timestamp
	err := row.Scan(&profile.AccountID, &profile.Username, &profile.Role, &created,
		&profile.Bio, &profile.HasAvatar, &avatarUpdated, &profile.ShowStats)
	if err != nil {
		return profile, err
	}
	if created.Status == pgtype.Present {
		profile.Created = created.Time
	}
	if avatarUpdated.Status == pgtype.Present {
		t := avatarUpdated.Time
		profile.AvatarUpdated = &t
	}
	return profile, nil
}

// GetSiteProfile looks a profile up by username, ignoring case.
func (c *Connection) GetSiteProfile(username string) (SiteProfile, bool, error) {
	profile, err := scanSiteProfile(c.pool.QueryRow(c.ctx,
		siteProfileQuery+` WHERE lower(a.username) = lower($1) ORDER BY (a.username = $1) DESC LIMIT 1`, strings.TrimSpace(username)))
	if err == pgx.ErrNoRows {
		return SiteProfile{}, false, nil
	}
	return profile, err == nil, err
}

// GetSiteProfileByAccount returns the profile of an account.
func (c *Connection) GetSiteProfileByAccount(accountID int) (SiteProfile, error) {
	return scanSiteProfile(c.pool.QueryRow(c.ctx, siteProfileQuery+` WHERE a.id = $1`, accountID))
}

// SaveProfileSettings stores the bio and the stats visibility.
func (c *Connection) SaveProfileSettings(accountID int, bio string, showStats bool) error {
	_, err := c.pool.Exec(c.ctx, `
		INSERT INTO public.panel_profiles (account_id, bio, show_stats, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (account_id) DO UPDATE
		SET bio = EXCLUDED.bio, show_stats = EXCLUDED.show_stats, updated_at = CURRENT_TIMESTAMP
	`, accountID, bio, showStats)
	return err
}

// SetProfileAvatar stores an already validated PNG; nil removes the avatar.
func (c *Connection) SetProfileAvatar(accountID int, png []byte) error {
	var updated *time.Time
	if png != nil {
		now := time.Now().UTC()
		updated = &now
	}
	_, err := c.pool.Exec(c.ctx, `
		INSERT INTO public.panel_profiles (account_id, avatar, avatar_updated, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (account_id) DO UPDATE
		SET avatar = EXCLUDED.avatar, avatar_updated = EXCLUDED.avatar_updated, updated_at = CURRENT_TIMESTAMP
	`, accountID, png, updated)
	return err
}

// ClearProfileBio removes the bio of an account (moderation).
func (c *Connection) ClearProfileBio(accountID int) error {
	_, err := c.pool.Exec(c.ctx, `UPDATE public.panel_profiles SET bio = '', updated_at = CURRENT_TIMESTAMP WHERE account_id = $1`, accountID)
	return err
}

// GetProfileAvatar returns the PNG avatar of a username.
func (c *Connection) GetProfileAvatar(username string) ([]byte, time.Time, bool, error) {
	var data []byte
	var updated pgtype.Timestamp
	err := c.pool.QueryRow(c.ctx, `
		SELECT p.avatar, p.avatar_updated
		FROM public.panel_profiles p
		JOIN public.panel_accounts a ON a.id = p.account_id
		WHERE lower(a.username) = lower($1) AND p.avatar IS NOT NULL
		ORDER BY (a.username = $1) DESC LIMIT 1
	`, strings.TrimSpace(username)).Scan(&data, &updated)
	if err == pgx.ErrNoRows {
		return nil, time.Time{}, false, nil
	}
	if err != nil {
		return nil, time.Time{}, false, err
	}
	return data, updated.Time, true, nil
}

func profileIDArgs(profileIDs []uint32) []int64 {
	ids := make([]int64, len(profileIDs))
	for i, id := range profileIDs {
		ids[i] = int64(id)
	}
	return ids
}

// GetProfileStats aggregates the match logs and time trials of game profiles.
func (c *Connection) GetProfileStats(profileIDs []uint32) (ProfileStats, error) {
	stats := ProfileStats{Games: []string{}}
	if len(profileIDs) == 0 {
		return stats, nil
	}
	ids := profileIDArgs(profileIDs)

	rows, err := c.pool.Query(c.ctx, `
		SELECT m.game_name, m.created_at,
			CASE WHEN p->>'ev' ~ '^[0-9]+$' THEN (p->>'ev')::int END,
			CASE WHEN p->>'eb' ~ '^[0-9]+$' THEN (p->>'eb')::int END
		FROM public.panel_match_logs m
		CROSS JOIN LATERAL jsonb_array_elements(m.players) p
		WHERE p->>'pid' ~ '^[0-9]+$' AND (p->>'pid')::bigint = ANY($1)
		ORDER BY m.created_at DESC
	`, ids)
	if err != nil {
		return stats, err
	}
	seenGames := map[string]bool{}
	for rows.Next() {
		var game string
		var created pgtype.Timestamp
		var vr, br *int
		if err := rows.Scan(&game, &created, &vr, &br); err != nil {
			rows.Close()
			return stats, err
		}
		if stats.Matches == 0 && created.Status == pgtype.Present {
			t := created.Time
			stats.LastMatch = &t
		}
		stats.Matches++
		if game != "" && !seenGames[game] {
			seenGames[game] = true
			stats.Games = append(stats.Games, game)
		}
		if stats.VersusRating == nil && vr != nil && *vr > 0 {
			stats.VersusRating = vr
		}
		if stats.BattleRating == nil && br != nil && *br > 0 {
			stats.BattleRating = br
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return stats, err
	}

	err = c.pool.QueryRow(c.ctx, `SELECT count(*) FROM public.mario_kart_wii_sake WHERE pid = ANY($1)`, ids).Scan(&stats.TimeTrials)
	return stats, err
}

// ProfileUsernames maps game profile IDs to the username of the site account
// they belong to, either directly or through a linked console.
func (c *Connection) ProfileUsernames(profileIDs []uint32) (map[uint32]string, error) {
	out := map[uint32]string{}
	if len(profileIDs) == 0 {
		return out, nil
	}
	rows, err := c.pool.Query(c.ctx, `
		SELECT u.profile_id, a.username
		FROM public.users u
		JOIN public.consoles c ON c.id = u.console_id
		JOIN public.panel_accounts a ON a.id = c.account_id
		WHERE u.profile_id = ANY($1)
		UNION
		SELECT a.profile_id, a.username
		FROM public.panel_accounts a
		WHERE a.profile_id = ANY($1)
	`, profileIDArgs(profileIDs))
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var pid int64
		var username string
		if err := rows.Scan(&pid, &username); err != nil {
			return out, err
		}
		out[uint32(pid)] = username
	}
	return out, rows.Err()
}

// AccountGameProfiles lists the game profiles linked to an account.
func (c *Connection) AccountGameProfiles(accountID int) []ConsoleProfileRow {
	account := PanelAccount{ID: accountID}
	c.hydratePanelAccountLinkedProfiles(&account)
	return account.LinkedProfiles
}
