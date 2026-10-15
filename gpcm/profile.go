package gpcm

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/database"
	"dwc-server-advanced/logging"
	"dwc-server-advanced/qr2"
	"strconv"

	"github.com/logrusorgru/aurora/v3"
)

func (g *GameSpySession) getProfile(command common.GameSpyCommand) {
	strProfileId := command.OtherValues["profileid"]
	profileId, err := strconv.ParseUint(strProfileId, 10, 32)
	if err != nil {
		// There was an error getting profile info.
		g.replyError(ErrGetProfile)
		return
	}

	logging.Info(g.ModuleName, "Looking up the profile of", aurora.Cyan(profileId).String())

	user := database.User{}
	locstring := ""

	mutex.Lock()
	if session, ok := sessions[uint32(profileId)]; ok && session.LoggedIn {
		locstring = session.LocString
		user = session.User
		mutex.Unlock()
	} else {
		mutex.Unlock()
		user, ok = db.GetProfile(uint32(profileId))
		if !ok {
			// The profile info was requested on is invalid.
			g.replyError(ErrGetProfileBadProfile)
			return
		}
	}

	if user.ProfileId == g.User.ProfileId {
		g.WriteBuffer += common.CreateGameSpyMessage(common.GameSpyCommand{
			Command:      "pi",
			CommandValue: "",
			OtherValues: map[string]string{
				"profileid":  command.OtherValues["profileid"],
				"nick":       user.UniqueNick,
				"userid":     strconv.FormatUint(uint64(user.UserId), 10),
				"email":      user.Email,
				"sig":        common.RandomHexString(32),
				"uniquenick": user.UniqueNick,
				"firstname":  user.FirstName,
				"lastname":   user.LastName,
				"pid":        "11",
				"lon":        "0.000000",
				"lat":        "0.000000",
				"loc":        locstring,
				"id":         command.OtherValues["id"],
			},
		})
	} else {
		g.WriteBuffer += common.CreateGameSpyMessage(common.GameSpyCommand{
			Command:      "pi",
			CommandValue: "",
			OtherValues: map[string]string{
				"profileid":  command.OtherValues["profileid"],
				"nick":       "000000000" + user.GsbrCode[:4] + "0000000",
				"userid":     "0",
				"email":      "000000000" + user.GsbrCode[:4] + "0000000" + "@nds",
				"sig":        common.RandomHexString(32),
				"uniquenick": "000000000" + user.GsbrCode[:4] + "0000000",
				"firstname":  user.FirstName,
				"lastname":   "000000000" + user.GsbrCode[:4] + "0000000",
				"pid":        "11",
				"lon":        "0.000000",
				"lat":        "0.000000",
				"loc":        locstring,
				"id":         command.OtherValues["id"],
			},
		})
	}
}

func (g *GameSpySession) updateProfile(command common.GameSpyCommand) {
	openHostChanged := false
	openHostNew := false
	if openHost, ok := command.OtherValues["wl:oh"]; ok {
		enabled := openHost != "0"
		if g.User.OpenHost != enabled {
			openHostChanged = true
			openHostNew = enabled
			mutex.Lock()
			g.User.OpenHost = enabled
			if enabled {
				g.User.OpenHostMode = database.OpenHostModeBlacklist
			} else {
				g.User.OpenHostMode = database.OpenHostModeDisabled
			}
			g.openHostDisabledLocked(false)
			if enabled {
				g.openHostEnabled(true, false)
			}
			mutex.Unlock()
		}
	}

	db.UpdateProfile(&g.User, command.OtherValues)

	if openHostChanged {
		qr2.SetOpenHost(g.User.ProfileId, openHostNew)
	}
}

// SetOpenHost mutates the OpenHost flag on a logged-in session and runs the
// friend-list side effects. Returns true if a live session was updated.
func SetOpenHost(profileID uint32, enabled bool) bool {
	mode := database.OpenHostModeBlacklist
	if !enabled {
		mode = database.OpenHostModeDisabled
	}
	return SetOpenHostMode(profileID, mode, nil)
}

// SetOpenHostMode applies a new mode + list to a logged-in session and runs
// the friend-list reconciliation. Returns true if a live session was updated.
func SetOpenHostMode(profileID uint32, mode string, list []uint32) bool {
	mode = database.NormalizeOpenHostMode(mode)

	mutex.Lock()
	defer mutex.Unlock()

	session, ok := sessions[profileID]
	if !ok || !session.LoggedIn {
		return false
	}

	session.User.OpenHostMode = mode
	session.User.OpenHostList = append([]uint32(nil), list...)
	session.User.OpenHost = mode != database.OpenHostModeDisabled

	// Reconcile both directions: drop ties no longer warranted, then add
	// ties newly warranted by the updated mode.
	session.openHostDisabledLocked(false)
	if session.User.OpenHost {
		session.openHostEnabled(true, false)
	}
	return true
}

func VerifyPlayerSearch(profileId uint32, sessionKey int32, gameName string) (string, bool) {
	mutex.Lock()
	defer mutex.Unlock()

	if session, ok := sessions[profileId]; ok && session.LoggedIn && session.SessionKey == sessionKey && session.GameName == gameName {
		return "000000000" + session.User.GsbrCode[:4] + "0000000", true
	}

	return "", false
}
