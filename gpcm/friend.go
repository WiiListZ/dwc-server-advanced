package gpcm

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/logging"
	"dwc-server-advanced/qr2"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora/v3"
)

func removeFromUint32Array(arrayPointer *[]uint32, index int) {
	array := *arrayPointer
	arrayLength := len(array)

	if index < 0 || index >= arrayLength {
		return // Ignore?
	}

	lastIndex := arrayLength - 1

	array[index] = array[lastIndex]
	*arrayPointer = array[:lastIndex]
}

func appendUniqueUint32(arrayPointer *[]uint32, value uint32) bool {
	for _, stored := range *arrayPointer {
		if stored == value {
			return false
		}
	}
	*arrayPointer = append(*arrayPointer, value)
	return true
}

func (g *GameSpySession) isFriendAdded(profileId uint32) bool {
	for _, storedPid := range g.FriendList {
		if storedPid == profileId {
			return true
		}
	}
	return false
}

func (g *GameSpySession) getFriendIndex(profileId uint32) int {
	for i, storedPid := range g.FriendList {
		if storedPid == profileId {
			return i
		}
	}
	return -1
}

func (g *GameSpySession) isOpenHostFriend(profileId uint32) bool {
	for _, storedPid := range g.OpenHostFriendList {
		if storedPid == profileId {
			return true
		}
	}
	return false
}

func (g *GameSpySession) getOpenHostFriendIndex(profileId uint32) int {
	for i, storedPid := range g.OpenHostFriendList {
		if storedPid == profileId {
			return i
		}
	}
	return -1
}

func (g *GameSpySession) removeOpenHostFriendMarker(profileId uint32) {
	if idx := g.getOpenHostFriendIndex(profileId); idx >= 0 {
		removeFromUint32Array(&g.OpenHostFriendList, idx)
	}
}

func (g *GameSpySession) addOpenHostFriend(profileId uint32) {
	if !g.isFriendAdded(profileId) {
		g.FriendList = append(g.FriendList, profileId)
		appendUniqueUint32(&g.OpenHostFriendList, profileId)
	}
}

func (g *GameSpySession) removeOpenHostFriend(profileId uint32) {
	if !g.isOpenHostFriend(profileId) {
		return
	}
	g.removeOpenHostFriendMarker(profileId)
	if idx := g.getFriendIndex(profileId); idx >= 0 {
		removeFromUint32Array(&g.FriendList, idx)
	}
}

func (g *GameSpySession) isFriendAuthorized(profileId uint32) bool {
	for _, storedPid := range g.AuthFriendList {
		if storedPid == profileId {
			return true
		}
	}
	return false
}

func (g *GameSpySession) getAuthorizedFriendIndex(profileId uint32) int {
	for i, storedPid := range g.AuthFriendList {
		if storedPid == profileId {
			return i
		}
	}
	return -1
}

const (
	// addFriendMessage = "\r\n\r\n|signed|00000000000000000000000000000000"

	// Message used by DS games and some Wii games
	bm1AuthMessage = "I have authorized your request to add me to your list"

	logOutMessage = "|s|0|ss|Offline|ls||ip|0|p|0|qm|0"
)

func (g *GameSpySession) isBm1AuthMessageNeeded() bool {
	return g.UnitCode == UnitCodeDS || g.UnitCode == UnitCodeDSAndWii || g.GameName == "jissenpachwii" || g.GameName == "drmariowii" || g.GameName == "pokebattlewii"
}

func (g *GameSpySession) addFriend(command common.GameSpyCommand) {
	strNewProfileId := command.OtherValues["newprofileid"]
	newProfileId, err := strconv.ParseUint(strNewProfileId, 10, 32)
	if err != nil {
		g.replyError(ErrAddFriend)
		return
	}

	if newProfileId == uint64(g.User.ProfileId) {
		logging.Error(g.ModuleName, "Attempt to add self as friend")
		g.replyError(ErrAddFriendBadNew)
		return
	}

	fc := common.CalcFriendCodeString(uint32(newProfileId), g.User.GsbrCode[:4])
	logging.Info(g.ModuleName, "Add friend:", aurora.Cyan(strNewProfileId), aurora.Cyan(fc))

	mutex.Lock()
	defer mutex.Unlock()

	authorized := g.isFriendAuthorized(uint32(newProfileId))
	openHostForPeer := g.User.IsOpenHostFor(uint32(newProfileId))
	duplicateAuthorized := !openHostForPeer && authorized

	newProfileID32 := uint32(newProfileId)

	// TODO: Add a limit
	if g.isFriendAdded(newProfileID32) {
		// The local player explicitly added this profile, so a previous
		// temporary Open Host entry is now a normal friend entry.
		g.removeOpenHostFriendMarker(newProfileID32)
	} else {
		g.FriendList = append(g.FriendList, newProfileID32)
	}

	// Check if destination has added the sender
	newSession, ok := sessions[newProfileID32]
	if !ok || newSession == nil || !newSession.LoggedIn {
		logging.Info(g.ModuleName, "Destination is not online")
		return
	}

	if newSession.GameName != g.GameName {
		logging.Error(g.ModuleName, "Destination is not playing the same game")
		// g.replyError(ErrAddFriendBadNew)
		return
	}

	destinationOpenHostForSender := newSession.User.IsOpenHostFor(g.User.ProfileId)
	if duplicateAuthorized && !destinationOpenHostForSender {
		logging.Info(g.ModuleName, "Attempt to add a friend who is already authorized")
		// This seems to always happen, do we need to return an error?
		// DWC vocally ignores the error anyway, so let's not bother
		// g.replyError(ErrAddFriendAlreadyFriends)
		return
	}
	if !destinationOpenHostForSender && !newSession.isFriendAdded(g.User.ProfileId) {
		// Not an error, just ignore for now
		logging.Info(g.ModuleName, "Destination has not added sender")
		return
	}
	if destinationOpenHostForSender {
		newSession.addOpenHostFriend(g.User.ProfileId)
	}

	// Friends are now mutual!
	// TODO: Add a limit
	appendUniqueUint32(&g.AuthFriendList, newProfileID32)
	appendUniqueUint32(&newSession.AuthFriendList, g.User.ProfileId)

	// Send friend auth message
	sendMessageToSessionBuffer("4", newSession.User.ProfileId, g, "")

	if g.isBm1AuthMessageNeeded() {
		sendMessageToSessionBuffer("1", newSession.User.ProfileId, g, bm1AuthMessage)
	}

	if newSession.isFriendAdded(g.User.ProfileId) && !openHostForPeer {
		// If we're open host for them then this would've been sent already
		sendMessageToSession("4", g.User.ProfileId, newSession, "")

		if newSession.isBm1AuthMessageNeeded() {
			sendMessageToSession("1", g.User.ProfileId, newSession, bm1AuthMessage)
		}

		g.sendFriendStatus(newSession.User.ProfileId)
	}

	newSession.sendFriendStatus(g.User.ProfileId)
}

func (g *GameSpySession) removeFriend(command common.GameSpyCommand) {
	strDelProfileID := command.OtherValues["delprofileid"]
	delProfileID64, err := strconv.ParseUint(strDelProfileID, 10, 32)
	if err != nil {
		logging.Error(g.ModuleName, aurora.Cyan(strDelProfileID), "is not a valid profile id")
		g.replyError(ErrDeleteFriend)
		return
	}
	delProfileID32 := uint32(delProfileID64)

	fc := common.CalcFriendCodeString(delProfileID32, g.User.GsbrCode[:4])
	logging.Info(g.ModuleName, "Remove friend:", aurora.Cyan(strDelProfileID), aurora.Cyan(fc))

	mutex.Lock()
	defer mutex.Unlock()

	if g.isFriendAdded(delProfileID32) {
		delProfileIDIndex := g.getFriendIndex(delProfileID32)
		removeFromUint32Array(&g.FriendList, delProfileIDIndex)
	}

	if !g.User.IsOpenHostFor(delProfileID32) {
		if g.isFriendAuthorized(delProfileID32) {
			delProfileIDIndex := g.getAuthorizedFriendIndex(delProfileID32)
			removeFromUint32Array(&g.AuthFriendList, delProfileIDIndex)
		}

		if session, ok := sessions[delProfileID32]; ok && session.LoggedIn && session.isFriendAuthorized(g.User.ProfileId) {
			if session.isOpenHostFriend(g.User.ProfileId) {
				session.removeOpenHostFriend(g.User.ProfileId)
				if idx := session.getAuthorizedFriendIndex(g.User.ProfileId); idx >= 0 {
					removeFromUint32Array(&session.AuthFriendList, idx)
				}
			}
			sendMessageToSession("100", g.User.ProfileId, session, logOutMessage)
		}
	}
}

func (g *GameSpySession) authAddFriend(command common.GameSpyCommand) {
	strFromProfileId := command.OtherValues["fromprofileid"]
	fromProfileId, err := strconv.ParseUint(strFromProfileId, 10, 32)
	if err != nil {
		logging.Error(g.ModuleName, "Invalid profile ID string:", aurora.Cyan(strFromProfileId))
		g.replyError(ErrAuthAddBadFrom)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	if !g.isFriendAuthorized(uint32(fromProfileId)) {
		logging.Error(g.ModuleName, "Sender", aurora.Cyan(fromProfileId), "is not an authorized friend")
		g.replyError(ErrAuthAddBadFrom)
		return
	}

	g.exchangeFriendStatus(uint32(fromProfileId))
}

func (g *GameSpySession) setStatus(command common.GameSpyCommand) {
	status := command.CommandValue
	logging.Notice(g.ModuleName, "New status:", aurora.BrightMagenta(status))

	qr2.ProcessGPStatusUpdate(g.User.ProfileId, g.QR2IP, status)

	statstring, ok := command.OtherValues["statstring"]
	if !ok {
		logging.Warn(g.ModuleName, "Missing statstring")
		statstring = ""
	}

	locstring, ok := command.OtherValues["locstring"]
	if !ok {
		logging.Warn(g.ModuleName, "Missing locstring")
		locstring = ""
	}

	statusMsg := "|s|" + status + "|ss|" + statstring + "|ls|" + locstring + "|ip|0|p|0|qm|0"

	mutex.Lock()
	defer mutex.Unlock()

	if status == "3" && g.User.Restricted {
		logging.Warn(g.ModuleName, "Restricted user searching for public rooms")
		kickPlayer(g.User.ProfileId, "restricted_join")
	}

	g.LocString = locstring
	g.Status = statusMsg

	if !g.StatusSet && g.User.OpenHost {
		g.openHostEnabled(false, false)
	}

	g.StatusSet = true

	for _, storedPid := range g.AuthFriendList {
		g.sendFriendStatus(storedPid)
	}
}

func sendMessageToSession(msgType string, from uint32, session *GameSpySession, msg string) {
	message := common.CreateGameSpyMessage(common.GameSpyCommand{
		Command:      "bm",
		CommandValue: msgType,
		OtherValues: map[string]string{
			"f":   strconv.FormatUint(uint64(from), 10),
			"msg": msg,
		},
	})
	if err := common.SendPacket(ServerName, session.ConnIndex, []byte(message)); err != nil {
		logging.Error("GPCM", "Failed to send packet:", err)
		_ = common.CloseConnection(ServerName, session.ConnIndex)
	}
}

func sendMessageToSessionBuffer(msgType string, from uint32, session *GameSpySession, msg string) {
	session.WriteBuffer += common.CreateGameSpyMessage(common.GameSpyCommand{
		Command:      "bm",
		CommandValue: msgType,
		OtherValues: map[string]string{
			"f":   strconv.FormatUint(uint64(from), 10),
			"msg": msg,
		},
	})
}

func sendMessageToProfileId(msgType string, from uint32, to uint32, msg string) bool {
	if session, ok := sessions[to]; ok && session.LoggedIn {
		sendMessageToSession(msgType, from, session, msg)
		return true
	}

	logging.Info("GPCM", "Destination", aurora.Cyan(to), "from", aurora.Cyan(from), "is not online")
	return false
}

func (g *GameSpySession) sendFriendStatus(profileId uint32) {
	common.MaybeUnused(sendMessageToProfileId)

	if !g.isFriendAuthorized(profileId) {
		return
	}

	if session, ok := sessions[profileId]; ok && session.LoggedIn && session.isFriendAdded(g.User.ProfileId) {
		// Prevent players abusing a stack overflow exploit with the locstring in Mario Kart Wii
		if session.NeedsExploit && strings.HasPrefix(session.GameCode, "RMC") && len(g.LocString) > 0x14 {
			logging.Warn("GPCM", "Blocked message from", aurora.Cyan(g.User.ProfileId), "to", aurora.Cyan(session.User.ProfileId), "due to a stack overflow exploit")
			return
		}

		session.recordStatusSent(g.User.ProfileId)
		sendMessageToSession("100", g.User.ProfileId, session, g.Status)
	}
}

func (g *GameSpySession) exchangeFriendStatus(profileId uint32) {
	if session, ok := sessions[profileId]; ok && session.LoggedIn {
		if session.isFriendAdded(g.User.ProfileId) && session.isFriendAuthorized(g.User.ProfileId) {
			if session.NeedsExploit && strings.HasPrefix(session.GameCode, "RMC") && len(g.LocString) > 0x14 {
				logging.Warn("GPCM", "Blocked message from", aurora.Cyan(g.User.ProfileId), "to", aurora.Cyan(session.User.ProfileId), "due to a stack overflow exploit")
				return
			}

			session.recordStatusSent(g.User.ProfileId)
			sendMessageToSession("100", g.User.ProfileId, session, g.Status)
		}

		if g.isFriendAdded(profileId) && g.isFriendAuthorized(profileId) {
			if g.NeedsExploit && strings.HasPrefix(g.GameCode, "RMC") && len(session.LocString) > 0x14 {
				logging.Warn("GPCM", "Blocked message from", aurora.Cyan(session.User.ProfileId), "to", aurora.Cyan(g.User.ProfileId), "due to a stack overflow exploit")
				return
			}

			g.recordStatusSent(profileId)
			sendMessageToSessionBuffer("100", profileId, g, session.Status)
		}
	}
}

func (g *GameSpySession) recordStatusSent(sender uint32) {
	for _, friend := range g.RecvStatusFromList {
		if friend == sender {
			return
		}
	}

	g.RecvStatusFromList = append(g.RecvStatusFromList, sender)
}

func (g *GameSpySession) sendLogoutStatus() {
	mutex.Lock()
	defer mutex.Unlock()

	for _, storedPid := range g.AuthFriendList {
		if session, ok := sessions[storedPid]; ok && session.LoggedIn && session.isFriendAuthorized(g.User.ProfileId) {
			delProfileIDIndex := session.getAuthorizedFriendIndex(g.User.ProfileId)
			removeFromUint32Array(&session.AuthFriendList, delProfileIDIndex)
			sendMessageToSession("100", g.User.ProfileId, session, logOutMessage)
		}
	}
}

func (g *GameSpySession) openHostEnabled(sendStatus bool, lock bool) {
	if lock {
		mutex.Lock()
		defer mutex.Unlock()
	}

	for _, session := range sessions {
		if !session.LoggedIn {
			continue
		}
		if !session.isFriendAdded(g.User.ProfileId) {
			continue
		}
		if session.isFriendAuthorized(g.User.ProfileId) && g.isFriendAdded(session.User.ProfileId) {
			continue
		}
		if !g.User.IsOpenHostFor(session.User.ProfileId) {
			continue
		}

		g.addOpenHostFriend(session.User.ProfileId)
		appendUniqueUint32(&session.AuthFriendList, g.User.ProfileId)
		appendUniqueUint32(&g.AuthFriendList, session.User.ProfileId)
		sendMessageToSession("4", g.User.ProfileId, session, "")
		sendMessageToSession("4", session.User.ProfileId, g, "")

		if session.isBm1AuthMessageNeeded() {
			sendMessageToSession("1", g.User.ProfileId, session, bm1AuthMessage)
		}
		if g.isBm1AuthMessageNeeded() {
			sendMessageToSession("1", session.User.ProfileId, g, bm1AuthMessage)
		}

		if sendStatus {
			g.sendFriendStatus(session.User.ProfileId)
			session.sendFriendStatus(g.User.ProfileId)
		}
	}
}

func (g *GameSpySession) openHostDisabled() {
	g.openHostDisabledLocked(true)
}

// openHostDisabledLocked drops auto-authorized friend ties that are no longer
// warranted under g's current Open Host mode/list. A tie is "auto" when g is
// authorized with peer but g has not added peer to its FriendList. Those ties
// only existed because Open Host was applying for that peer; if it no longer
// applies, the tie is revoked and the peer is told that g went offline.
func (g *GameSpySession) openHostDisabledLocked(lock bool) {
	if lock {
		mutex.Lock()
		defer mutex.Unlock()
	}

	authCopy := append([]uint32(nil), g.AuthFriendList...)
	for _, id := range authCopy {
		if g.isFriendAdded(id) && !g.isOpenHostFriend(id) {
			continue
		}
		if g.User.IsOpenHostFor(id) {
			continue
		}

		wasOpenHostFriend := g.isOpenHostFriend(id)
		g.removeOpenHostFriend(id)
		if idx := g.getAuthorizedFriendIndex(id); idx >= 0 {
			removeFromUint32Array(&g.AuthFriendList, idx)
		}

		if session, ok := sessions[id]; ok && session.LoggedIn && session.isFriendAuthorized(g.User.ProfileId) {
			if idx := session.getAuthorizedFriendIndex(g.User.ProfileId); idx >= 0 {
				removeFromUint32Array(&session.AuthFriendList, idx)
			}
			sendMessageToSession("100", g.User.ProfileId, session, logOutMessage)
		}
		if wasOpenHostFriend && g.LoggedIn {
			sendMessageToSession("100", id, g, logOutMessage)
		}
	}
}
