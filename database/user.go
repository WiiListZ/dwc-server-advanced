package database

import (
	"dwc-server-advanced/common"
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

const (
	InsertUser              = `INSERT INTO users (user_id, gsbrcd, password, ng_device_id, email, unique_nick) VALUES ($1, $2, $3, $4, $5, $6) RETURNING profile_id`
	InsertUserWithProfileID = `INSERT INTO users (profile_id, user_id, gsbrcd, password, ng_device_id, email, unique_nick) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	UpdateUserTable         = `UPDATE users SET
		firstname = CASE WHEN $3 THEN $2 ELSE firstname END,
		lastname = CASE WHEN $5 THEN $4 ELSE lastname END,
		open_host = CASE WHEN $7 AND COALESCE(right_open_host, true) THEN $6 ELSE open_host END,
		open_host_mode = CASE WHEN $7 AND COALESCE(right_open_host, true) THEN (CASE WHEN $6 THEN 'blacklist' ELSE 'disabled' END) ELSE open_host_mode END
		WHERE profile_id = $1`
	UpdateUserProfileID             = `UPDATE users SET profile_id = $3 WHERE user_id = $1 AND gsbrcd = $2`
	UpdateUserNGDeviceID            = `UPDATE users SET ng_device_id = $2 WHERE profile_id = $1`
	GetUser                         = `SELECT user_id, gsbrcd, email, unique_nick, firstname, lastname, open_host, COALESCE(open_host_mode, 'disabled'), COALESCE(open_host_list, ARRAY[]::bigint[]), COALESCE(right_open_host, true), last_ip_address, last_ingamesn FROM users WHERE profile_id = $1`
	ClearProfileQuery               = `DELETE FROM users WHERE profile_id = $1 RETURNING user_id, gsbrcd, email, unique_nick, firstname, lastname, open_host, COALESCE(open_host_mode, 'disabled'), COALESCE(open_host_list, ARRAY[]::bigint[]), COALESCE(right_open_host, true), last_ip_address, last_ingamesn`
	DoesUserExist                   = `SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1 AND gsbrcd = $2)`
	IsProfileIDInUse                = `SELECT EXISTS(SELECT 1 FROM users WHERE profile_id = $1)`
	DeleteUserSession               = `DELETE FROM sessions WHERE profile_id = $1`
	GetUserProfileID                = `SELECT profile_id, ng_device_id, email, unique_nick, firstname, lastname, open_host, COALESCE(open_host_mode, 'disabled'), COALESCE(open_host_list, ARRAY[]::bigint[]), COALESCE(right_open_host, true), last_ip_address, allow_default_keys FROM users WHERE user_id = $1 AND gsbrcd = $2`
	UpdateUserLastIPAddress         = `UPDATE users SET last_ip_address = $2, last_ingamesn = $3 WHERE profile_id = $1`
	UpdateUserBan                   = `UPDATE users SET has_ban = true, ban_issued = $2, ban_expires = $3, ban_revoked = NULL, ban_reason = $4, ban_reason_hidden = $5, ban_moderator = $6, ban_tos = $7, last_ingamesn = CASE WHEN $8 <> '' THEN $8 ELSE last_ingamesn END WHERE profile_id = $1`
	DisableUserBan                  = `UPDATE users SET has_ban = false, ban_revoked = $2 WHERE profile_id = $1 AND (has_ban = true OR ban_issued IS NOT NULL)`
	SetOpenHostFlag                 = `UPDATE users SET open_host = $2 WHERE profile_id = $1`
	GetOpenHostFlag                 = `SELECT open_host FROM users WHERE profile_id = $1`
	SetOpenHostModeQuery            = `UPDATE users SET open_host_mode = $2::text, open_host_list = $3, open_host = ($2::text <> 'disabled') WHERE profile_id = $1 AND COALESCE(right_open_host, true)`
	SetOpenHostModeWithDisplayQuery = `UPDATE users SET open_host_mode = $2::text, open_host_list = $3, open_host_list_display = $4, open_host = ($2::text <> 'disabled') WHERE profile_id = $1 AND COALESCE(right_open_host, true)`
	GetOpenHostModeQuery            = `SELECT COALESCE(open_host_mode, 'disabled'), COALESCE(open_host_list, ARRAY[]::bigint[]), open_host FROM users WHERE profile_id = $1`
	GetOpenHostModeWithDisplayQuery = `SELECT COALESCE(open_host_mode, 'disabled'), COALESCE(open_host_list, ARRAY[]::bigint[]), COALESCE(open_host_list_display, ARRAY[]::text[]), open_host FROM users WHERE profile_id = $1`
	GetMiscRightsQuery              = `SELECT COALESCE(right_open_host, true) FROM users WHERE profile_id = $1`
	SetMiscRightOpenHostQuery       = `UPDATE users SET right_open_host = $2::boolean,
		open_host = CASE WHEN NOT $2::boolean THEN false ELSE open_host END,
		open_host_mode = CASE WHEN NOT $2::boolean THEN 'disabled' ELSE open_host_mode END
		WHERE profile_id = $1`
)

const (
	OpenHostModeDisabled  = "disabled"
	OpenHostModeWhitelist = "whitelist"
	OpenHostModeBlacklist = "blacklist"
)

func NormalizeOpenHostMode(mode string) string {
	switch mode {
	case OpenHostModeDisabled, OpenHostModeWhitelist, OpenHostModeBlacklist:
		return mode
	default:
		return OpenHostModeDisabled
	}
}

// IsOpenHostFor returns true when the auto-friend ("Open Host") behavior
// applies for the given peer profile ID, considering the user's mode + list.
func (u *User) IsOpenHostFor(peerProfileID uint32) bool {
	mode := NormalizeOpenHostMode(u.OpenHostMode)
	if mode == OpenHostModeDisabled || !u.OpenHost {
		return false
	}

	switch mode {
	case OpenHostModeDisabled:
		return false
	case OpenHostModeWhitelist:
		for _, id := range u.OpenHostList {
			if id == peerProfileID {
				return true
			}
		}
		return false
	case OpenHostModeBlacklist:
		for _, id := range u.OpenHostList {
			if id == peerProfileID {
				return false
			}
		}
		return true
	}
	return false
}

type User struct {
	ProfileId          uint32
	UserId             uint64
	GsbrCode           string
	NgDeviceId         []uint32
	Email              string
	UniqueNick         string
	FirstName          string
	LastName           string
	Restricted         bool
	RestrictedDeviceId uint32
	BanReason          string
	OpenHost           bool
	OpenHostMode       string
	OpenHostList       []uint32
	RightOpenHost      bool
	LastInGameSn       string
	LastIPAddress      string
	Created            bool
}

// MiscRightKey is one of the per-profile "Miscellaneous Rights" keys exposed
// to admins. Add new keys here and to GetMiscRights / SetMiscRight.
type MiscRightKey string

const (
	MiscRightOpenHost MiscRightKey = "open_host"
)

func AllMiscRightKeys() []MiscRightKey {
	return []MiscRightKey{MiscRightOpenHost}
}

var (
	ErrProfileIDInUse         = errors.New("profile ID is already in use")
	ErrReservedProfileIDRange = errors.New("profile ID is in reserved range")
)

func (c *Connection) CreateUser(user *User) error {
	if user.ProfileId == 0 {
		return c.pool.QueryRow(c.ctx, InsertUser, user.UserId, user.GsbrCode, "", user.NgDeviceId, user.Email, user.UniqueNick).Scan(&user.ProfileId)
	}

	if user.ProfileId >= 1000000000 {
		return ErrReservedProfileIDRange
	}

	var exists bool
	err := c.pool.QueryRow(c.ctx, IsProfileIDInUse, user.ProfileId).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		return ErrProfileIDInUse
	}

	_, err = c.pool.Exec(c.ctx, InsertUserWithProfileID, user.ProfileId, user.UserId, user.GsbrCode, "", user.NgDeviceId, user.Email, user.UniqueNick)
	return err
}

func (c *Connection) UpdateProfileID(user *User, newProfileId uint32) error {
	if newProfileId >= 1000000000 {
		return ErrReservedProfileIDRange
	}

	var exists bool
	err := c.pool.QueryRow(c.ctx, IsProfileIDInUse, newProfileId).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		return ErrProfileIDInUse
	}

	_, err = c.pool.Exec(c.ctx, UpdateUserProfileID, user.UserId, user.GsbrCode, newProfileId)
	if err == nil {
		user.ProfileId = newProfileId
	}

	return err
}

func GetUniqueUserID() uint64 {
	// Not guaranteed unique but doesn't matter in practice if multiple people have the same user ID.
	return uint64(rand.Int63n(0x80000000000))
}

func (c *Connection) UpdateProfile(user *User, data map[string]string) {
	firstName, firstNameExists := data["firstname"]
	lastName, lastNameExists := data["lastname"]
	openHost, openHostExists := data["wl:oh"]
	openHostBool := openHostExists && openHost != "0"

	_, err := c.pool.Exec(c.ctx, UpdateUserTable, user.ProfileId, firstName, firstNameExists, lastName, lastNameExists, openHostBool, openHostExists)
	if err != nil {
		panic(err)
	}

	if firstNameExists {
		user.FirstName = firstName
	}

	if lastNameExists {
		user.LastName = lastName
	}

	if openHostExists {
		user.OpenHost = openHostBool
		if openHostBool {
			user.OpenHostMode = OpenHostModeBlacklist
		} else {
			user.OpenHostMode = OpenHostModeDisabled
		}
	}
}

func (c *Connection) GetProfile(profileId uint32) (User, bool) {
	user := User{}
	row := c.pool.QueryRow(c.ctx, GetUser, profileId)
	err := row.Scan(&user.UserId, &user.GsbrCode, &user.Email, &user.UniqueNick, &user.FirstName, &user.LastName, &user.OpenHost, &user.OpenHostMode, &user.OpenHostList, &user.RightOpenHost, &user.LastIPAddress, &user.LastInGameSn)
	if err != nil {
		return User{}, false
	}

	user.OpenHostMode = NormalizeOpenHostMode(user.OpenHostMode)
	user.ProfileId = profileId
	return user, true
}

func (c *Connection) ClearProfile(profileId uint32) (User, bool) {
	user := User{}
	row := c.pool.QueryRow(c.ctx, ClearProfileQuery, profileId)
	err := row.Scan(&user.UserId, &user.GsbrCode, &user.Email, &user.UniqueNick, &user.FirstName, &user.LastName, &user.OpenHost, &user.OpenHostMode, &user.OpenHostList, &user.RightOpenHost, &user.LastIPAddress, &user.LastInGameSn)

	if err != nil {
		return User{}, false
	}

	user.OpenHostMode = NormalizeOpenHostMode(user.OpenHostMode)
	user.ProfileId = profileId
	return user, true
}

func (c *Connection) SetUserOpenHost(profileID uint32, enabled bool) error {
	_, err := c.pool.Exec(c.ctx, SetOpenHostFlag, profileID, enabled)
	return err
}

func (c *Connection) GetUserOpenHost(profileID uint32) (bool, bool, error) {
	var enabled bool
	err := c.pool.QueryRow(c.ctx, GetOpenHostFlag, profileID).Scan(&enabled)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, false, nil
		}
		return false, false, err
	}
	return enabled, true, nil
}

// MiscRights groups every per-profile "Miscellaneous Right" admins can toggle.
type MiscRights struct {
	OpenHost bool `json:"open_host"`
}

func (c *Connection) GetMiscRights(profileID uint32) (MiscRights, bool, error) {
	rights := MiscRights{}
	err := c.pool.QueryRow(c.ctx, GetMiscRightsQuery, profileID).Scan(&rights.OpenHost)
	if err != nil {
		if err == pgx.ErrNoRows {
			return MiscRights{}, false, nil
		}
		return MiscRights{}, false, err
	}
	return rights, true, nil
}

func (c *Connection) SetMiscRight(profileID uint32, key MiscRightKey, value bool) (bool, error) {
	switch key {
	case MiscRightOpenHost:
		res, err := c.pool.Exec(c.ctx, SetMiscRightOpenHostQuery, profileID, value)
		if err != nil {
			return false, err
		}
		return res.RowsAffected() > 0, nil
	default:
		return false, errors.New("unknown misc right key")
	}
}

// ErrOpenHostRightRevoked is returned when SetUserOpenHostMode is called for a
// profile whose right_open_host has been turned off by a moderator.
var ErrOpenHostRightRevoked = errors.New("open host right revoked")

func (c *Connection) SetUserOpenHostMode(profileID uint32, mode string, list []uint32) error {
	return c.SetUserOpenHostModeWithDisplay(profileID, mode, list, nil)
}

func (c *Connection) SetUserOpenHostModeWithDisplay(profileID uint32, mode string, list []uint32, display []string) error {
	mode = NormalizeOpenHostMode(mode)
	if list == nil {
		list = []uint32{}
	}
	display = normalizeOpenHostListDisplay(list, display)
	res, err := c.pool.Exec(c.ctx, SetOpenHostModeWithDisplayQuery, profileID, mode, list, display)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrOpenHostRightRevoked
	}
	return nil
}

func (c *Connection) GetUserOpenHostMode(profileID uint32) (string, []uint32, bool, bool, error) {
	var mode string
	var list []uint32
	var enabled bool
	err := c.pool.QueryRow(c.ctx, GetOpenHostModeQuery, profileID).Scan(&mode, &list, &enabled)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil, false, false, nil
		}
		return "", nil, false, false, err
	}
	return NormalizeOpenHostMode(mode), list, enabled, true, nil
}

func (c *Connection) GetUserOpenHostModeWithDisplay(profileID uint32) (string, []uint32, []string, bool, bool, error) {
	var mode string
	var list []uint32
	var display []string
	var enabled bool
	err := c.pool.QueryRow(c.ctx, GetOpenHostModeWithDisplayQuery, profileID).Scan(&mode, &list, &display, &enabled)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil, nil, false, false, nil
		}
		return "", nil, nil, false, false, err
	}
	return NormalizeOpenHostMode(mode), list, normalizeOpenHostListDisplay(list, display), enabled, true, nil
}

func normalizeOpenHostListDisplay(list []uint32, display []string) []string {
	out := make([]string, len(list))
	for i, profileID := range list {
		if i < len(display) {
			out[i] = strings.TrimSpace(display[i])
		}
		if out[i] == "" {
			out[i] = "PID " + strconv.FormatUint(uint64(profileID), 10)
		}
	}
	return out
}

func (c *Connection) BanUser(profileId uint32, tos bool, length time.Duration, reason string, reasonHidden string, moderator string) bool {
	return c.BanUserWithName(profileId, tos, length, reason, reasonHidden, moderator, "")
}

func (c *Connection) BanUserWithName(profileId uint32, tos bool, length time.Duration, reason string, reasonHidden string, moderator string, inGameName string) bool {
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(length)
	return c.BanUserUntilWithName(profileId, tos, &expiresAt, reason, reasonHidden, moderator, inGameName)
}

func (c *Connection) BanUserUntilWithName(profileId uint32, tos bool, expiresAt *time.Time, reason string, reasonHidden string, moderator string, inGameName string) bool {
	consoleID, err := c.ResolveConsoleForProfile(profileId)
	if err == nil && consoleID > 0 {
		if err := c.BanConsole(consoleID, profileId, tos, expiresAt, reason, reasonHidden, moderator, inGameName); err == nil {
			return true
		}
	}

	// Fallback: legacy per-profile ban (used when no NG device id is known).
	issuedAt := time.Now().UTC()
	var expiresValue any
	if expiresAt != nil {
		expiresValue = *expiresAt
	}

	_, err = c.pool.Exec(c.ctx, UpdateUserBan, profileId, issuedAt, expiresValue, reason, reasonHidden, moderator, tos, strings.TrimSpace(inGameName))
	if err != nil {
		return false
	}

	err = c.RecordBanHistory(profileId, tos, issuedAt, expiresAt, reason, reasonHidden, moderator, inGameName)
	return err == nil
}

func (c *Connection) UnbanUser(profileId uint32) bool {
	consoleID, err := c.ResolveConsoleForProfile(profileId)
	if err == nil && consoleID > 0 {
		if ok, err := c.UnbanConsole(consoleID); err == nil && ok {
			return true
		}
	}

	if !common.GetConfig().KeepRevokedBans {
		if _, err = c.pool.Exec(c.ctx, clearUserBanQuery+` WHERE profile_id = $1 AND has_ban = true`, profileId); err != nil {
			return false
		}
		return c.DeleteLatestBanHistory(profileId) == nil
	}

	revokedAt := time.Now().UTC()
	_, err = c.pool.Exec(c.ctx, DisableUserBan, profileId, revokedAt)
	if err != nil {
		return false
	}

	err = c.RevokeLatestBanHistory(profileId, revokedAt)
	return err == nil
}

// clearUserBanQuery lifts a user ban without keeping it as revoked; callers
// append the WHERE clause.
const clearUserBanQuery = `UPDATE public.users
	SET has_ban = false,
	    ban_issued = NULL,
	    ban_expires = NULL,
	    ban_revoked = NULL,
	    ban_reason = NULL,
	    ban_reason_hidden = NULL,
	    ban_moderator = NULL,
	    ban_tos = NULL`
