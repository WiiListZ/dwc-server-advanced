package qr2

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/database"
	"dwc-server-advanced/logging"
	"encoding/base64"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/logrusorgru/aurora/v3"
)

type Group struct {
	GroupID       uint32
	GroupName     string
	CreateTime    time.Time
	GameName      string
	MatchType     string
	MKWRegion     string
	LastJoinIndex int
	server        *Session
	players       map[*Session]bool
	AdminCreated  bool

	MKWRaceNumber    int
	MKWCourseID      int
	MKWEngineClassID int

	mkwRaceEnd *mkwRaceEndRequest
}

type CreatedMatchmakingRoom struct {
	ID            string `json:"id"`
	GameName      string `json:"game"`
	MatchType     string `json:"type"`
	HostProfileID string `json:"host_pid"`
	Status        string `json:"status"`
}

type DeletedMatchmakingRoom struct {
	ID               string   `json:"id"`
	GameName         string   `json:"game"`
	PlayerProfileIDs []uint32 `json:"players"`
}

type EndedMKWRace struct {
	ID                string   `json:"id"`
	GameName          string   `json:"game"`
	RaceInfo          RaceInfo `json:"race"`
	InGameEndAttempts int      `json:"in_game_end_attempts"`
	ServerStateReset  bool     `json:"server_state_reset"`
	PayloadTargets    int      `json:"payload_targets"`
	DisconnectTargets int      `json:"disconnect_targets"`
	FallbackSeconds   int      `json:"fallback_seconds,omitempty"`
}

const (
	MKWRaceEndModeTrackingOnly    = "tracking_only"
	MKWRaceEndModeNativeTry       = "native_try"
	MKWRaceEndModeForceDisconnect = "force_disconnect"
)

// MKWRaceEndModes lists the accepted end modes.
var MKWRaceEndModes = []string{MKWRaceEndModeAuto, MKWRaceEndModeNativeTry, MKWRaceEndModeForceDisconnect, MKWRaceEndModeTrackingOnly}

var groups = map[string]*Group{}

func CreateMatchmakingRoom(profileID uint32, matchType string) (CreatedMatchmakingRoom, bool, string) {
	if profileID == 0 {
		return CreatedMatchmakingRoom{}, false, "host profile ID or friend code required"
	}

	normalizedMatchType, ok := normalizeAdminRoomMatchType(matchType)
	if !ok {
		return CreatedMatchmakingRoom{}, false, "invalid room type"
	}

	now := time.Now().UTC()
	moduleName := "QR2:AdminRoom:" + strconv.FormatUint(uint64(profileID), 10)

	mutex.Lock()
	defer mutex.Unlock()

	login := logins[profileID]
	if login == nil {
		return CreatedMatchmakingRoom{}, false, "host profile is not logged in"
	}

	session := login.session
	if session == nil || !session.Authenticated || session.LastKeepAlive < now.Unix()-60 {
		return CreatedMatchmakingRoom{}, false, "host profile has no active QR2 session"
	}
	if session.Data["+deviceauth"] != "1" {
		return CreatedMatchmakingRoom{}, false, "host profile is not device authenticated"
	}
	if session.Data["gamename"] == "" {
		return CreatedMatchmakingRoom{}, false, "host session has no game name"
	}

	if session.groupPointer != nil {
		return createdMatchmakingRoomInfo(session.groupPointer, profileID, "existing"), true, ""
	}
	if len(groups) >= 100000 {
		return CreatedMatchmakingRoom{}, false, "server room limit reached"
	}

	prepareAdminRoomSessionData(session, normalizedMatchType)

	group := &Group{
		GroupID:       uint32(now.UnixNano()),
		GroupName:     "",
		CreateTime:    now,
		GameName:      session.Data["gamename"],
		MatchType:     session.Data["dwc_mtype"],
		MKWRegion:     "",
		LastJoinIndex: 0,
		server:        session,
		players:       map[*Session]bool{session: true},
		AdminCreated:  true,
	}

	for {
		groupName := common.RandomString(6)
		if groups[groupName] != nil {
			continue
		}

		group.GroupName = groupName
		break
	}

	if group.GameName == "mariokartwii" {
		group.MKWRegion = normalizedMKWRegion(session.Data["rk"])
		session.Data["rk"] = group.MKWRegion
	}

	session.Data["+joinindex"] = "0"
	if session.Data["+localplayers"] == "" {
		session.Data["+localplayers"] = "1"
	}
	session.groupPointer = group
	session.GroupName = group.GroupName
	groups[group.GroupName] = group

	logging.Notice(moduleName, "Created admin matchmaking room", aurora.Cyan(group.GroupName))
	logging.Event("group_created", map[string]any{
		"dwc_group_id": group.GroupID,
		"group_name":   group.GroupName,
		"game_name":    group.GameName,
		"match_type":   group.MatchType,
		"host_id":      session.Data["dwc_pid"],
	})
	if matchLog, ok := panelMatchLogFromGroupLocked(group); ok {
		go recordPanelMatchLog(matchLog, true)
	}

	return createdMatchmakingRoomInfo(group, profileID, "created"), true, ""
}

func normalizeAdminRoomMatchType(matchType string) (string, bool) {
	switch strings.TrimSpace(strings.ToLower(matchType)) {
	case "", "public", "anybody", "0", "1":
		return "0", true
	case "private", "friend", "friends", "2", "3":
		return "2", true
	default:
		return "", false
	}
}

func prepareAdminRoomSessionData(session *Session, matchType string) {
	session.Data["dwc_mtype"] = matchType
	session.Data["dwc_hoststate"] = "2"
	session.Data["dwc_suspend"] = "0"
	session.Data["dwc_mresv"] = "0"

	if session.Data["numplayers"] == "" {
		session.Data["numplayers"] = "1"
	}

	if session.Data["gamename"] == "mariokartwii" {
		if session.Data["dwc_mver"] == "" {
			session.Data["dwc_mver"] = "90"
		}
		if session.Data["maxplayers"] == "" {
			session.Data["maxplayers"] = "11"
		}
		if session.Data["rk"] == "" {
			session.Data["rk"] = "vs"
		}
		if session.Data["p"] == "" {
			session.Data["p"] = "0"
		}
		if session.Data["ev"] == "" {
			session.Data["ev"] = "5000"
		}
		if session.Data["eb"] == "" {
			session.Data["eb"] = "5000"
		}
		return
	}

	if session.Data["dwc_mver"] == "" {
		session.Data["dwc_mver"] = "3"
	}
	if session.Data["maxplayers"] == "" {
		session.Data["maxplayers"] = "4"
	}
}

func normalizedMKWRegion(rk string) string {
	if rk == "" {
		return "vs"
	}
	if len(rk) == 4 && (strings.HasPrefix(rk, "vs_") || strings.HasPrefix(rk, "bt_")) && rk[3] >= '0' && rk[3] < '6' {
		return rk[:2]
	}

	return rk
}

func createdMatchmakingRoomInfo(group *Group, profileID uint32, status string) CreatedMatchmakingRoom {
	return CreatedMatchmakingRoom{
		ID:            group.GroupName,
		GameName:      group.GameName,
		MatchType:     matchTypeLabel(group.MatchType),
		HostProfileID: strconv.FormatUint(uint64(profileID), 10),
		Status:        status,
	}
}

func DeleteMatchmakingRoom(roomID string) (DeletedMatchmakingRoom, bool) {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return DeletedMatchmakingRoom{}, false
	}

	mutex.Lock()
	defer mutex.Unlock()

	group := groups[roomID]
	if group == nil {
		return deleteStandaloneMatchmakingRoom(roomID, time.Now().UTC().Unix())
	}
	if matchLog, ok := panelMatchLogFromGroupLocked(group); ok {
		go recordPanelMatchLog(matchLog, true)
	}

	deleted := DeletedMatchmakingRoom{
		ID:       group.GroupName,
		GameName: group.GameName,
	}
	seenProfileIDs := map[uint32]bool{}

	for session := range group.players {
		if profileID, err := strconv.ParseUint(session.Data["dwc_pid"], 10, 32); err == nil && !seenProfileIDs[uint32(profileID)] {
			deleted.PlayerProfileIDs = append(deleted.PlayerProfileIDs, uint32(profileID))
			seenProfileIDs[uint32(profileID)] = true
		}

		clearSessionRoomState(session)
	}

	for profileID, assignment := range forcedRoomJoins {
		if assignment.RoomID == roomID {
			delete(forcedRoomJoins, profileID)
		}
	}

	delete(groups, roomID)
	group.players = map[*Session]bool{}
	group.server = nil

	logging.Event("group_deleted", map[string]any{
		"dwc_group_id": group.GroupID,
		"group_name":   group.GroupName,
		"reason":       "admin_deleted",
	})

	return deleted, true
}

func EndMKWRace(roomID string, endMode string) (EndedMKWRace, bool, string) {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return EndedMKWRace{}, false, "room ID required"
	}
	if endMode == "" {
		endMode = MKWRaceEndModeTrackingOnly
	}

	mutex.Lock()

	group := groups[roomID]
	if group == nil {
		mutex.Unlock()
		return EndedMKWRace{}, false, "room not found"
	}
	if group.GameName != "mariokartwii" {
		mutex.Unlock()
		return EndedMKWRace{}, false, "room is not a Mario Kart Wii room"
	}
	if !mkwRaceActive(group) {
		mutex.Unlock()
		return EndedMKWRace{}, false, "room is not in a started game"
	}

	ended := EndedMKWRace{
		ID:       group.GroupName,
		GameName: group.GameName,
		RaceInfo: RaceInfo{
			RaceNumber:    group.MKWRaceNumber,
			CourseID:      group.MKWCourseID,
			EngineClassID: group.MKWEngineClassID,
		},
	}
	var inGameEndMessages []queuedServerMatchMessage
	var payloadIDs, disconnectIDs []uint32
	switch endMode {
	case MKWRaceEndModeAuto:
		inGameEndMessages, payloadIDs, disconnectIDs = buildMKWRaceAutoEndMessagesLocked(group)
		if len(inGameEndMessages) == 0 {
			mutex.Unlock()
			return EndedMKWRace{}, false, "no eligible in-game players to notify"
		}
	case MKWRaceEndModeNativeTry:
		inGameEndMessages = buildMKWRacePayloadEndMessagesLocked(group)
		if len(inGameEndMessages) == 0 {
			mutex.Unlock()
			return EndedMKWRace{}, false, "no eligible in-game players to notify"
		}
		for _, message := range inGameEndMessages {
			if profileID, err := strconv.ParseUint(message.receiver.Data["dwc_pid"], 10, 32); err == nil {
				payloadIDs = append(payloadIDs, uint32(profileID))
			}
		}
	case MKWRaceEndModeForceDisconnect:
		inGameEndMessages = buildMKWRaceForceDisconnectMessagesLocked(group)
		if len(inGameEndMessages) == 0 {
			mutex.Unlock()
			return EndedMKWRace{}, false, "no eligible in-game peer links to force-close"
		}
		for _, message := range inGameEndMessages {
			if profileID, err := strconv.ParseUint(message.receiver.Data["dwc_pid"], 10, 32); err == nil {
				disconnectIDs = append(disconnectIDs, uint32(profileID))
			}
		}
	case MKWRaceEndModeTrackingOnly:
	default:
		mutex.Unlock()
		return EndedMKWRace{}, false, "invalid end mode"
	}

	if matchLog, ok := panelMatchLogFromGroupLocked(group); ok {
		go recordPanelMatchLog(matchLog, true)
	}
	markMKWRaceEndedLocked(group)
	ended.ServerStateReset = true

	var request *mkwRaceEndRequest
	if endMode != MKWRaceEndModeTrackingOnly {
		request = startMKWRaceEndRequestLocked(group, endMode, payloadIDs, disconnectIDs)
	}
	ended.PayloadTargets = len(payloadIDs)
	ended.DisconnectTargets = len(disconnectIDs)
	fallback := endMode == MKWRaceEndModeAuto && len(payloadIDs) > 0
	if fallback {
		ended.FallbackSeconds = int(mkwRaceEndFallbackDelay / time.Second)
	}

	mutex.Unlock()

	ended.InGameEndAttempts = sendQueuedServerMatchMessages(inGameEndMessages)
	if fallback {
		scheduleMKWRaceEndFallback(group, request)
	}
	return ended, true, ""
}

func buildMKWRacePayloadEndMessagesLocked(group *Group) []queuedServerMatchMessage {
	if group == nil || group.GameName != "mariokartwii" {
		return nil
	}

	sender, targets := mkwRaceEndCommandParticipantsLocked(group)
	if sender == nil || len(targets) == 0 {
		return nil
	}

	message, ok := buildServerMatchCommandMessageLocked(sender, common.MatchWWFCMKWRaceEnd, common.MatchCommandData{
		Version: 90,
	})
	if !ok {
		return nil
	}

	messages := make([]queuedServerMatchMessage, 0, len(targets))
	for _, target := range targets {
		messages = append(messages, queuedServerMatchMessage{
			receiver: target.session,
			message:  message,
		})
	}
	return messages
}

func buildMKWRaceForceDisconnectMessagesLocked(group *Group) []queuedServerMatchMessage {
	if group == nil || group.GameName != "mariokartwii" {
		return nil
	}

	sender, targets := mkwRaceEndCommandParticipantsLocked(group)
	if sender == nil || len(targets) == 0 {
		return nil
	}

	profileIDs := []uint32{}
	for _, target := range targets {
		profileIDs = append(profileIDs, target.profileID)
	}

	messages := make([]queuedServerMatchMessage, 0, len(targets))
	for _, target := range targets {
		closeProfileIDs := make([]uint32, 0, len(profileIDs)-1)
		for _, profileID := range profileIDs {
			if profileID != target.profileID {
				closeProfileIDs = append(closeProfileIDs, profileID)
			}
		}
		if len(closeProfileIDs) == 0 {
			continue
		}

		message, ok := buildServerMatchCommandMessageLocked(sender, common.MatchServerCloseClient, common.MatchCommandData{
			Version: 90,
			ServerCloseClient: &common.MatchCommandDataServerCloseClient{
				ProfileIDs: closeProfileIDs,
			},
		})
		if !ok {
			continue
		}

		messages = append(messages, queuedServerMatchMessage{
			receiver: target.session,
			message:  message,
		})
	}
	return messages
}

type mkwRaceEndCommandTarget struct {
	session   *Session
	profileID uint32
}

func mkwRaceEndCommandParticipantsLocked(group *Group) (*Session, []mkwRaceEndCommandTarget) {
	targets := []mkwRaceEndCommandTarget{}
	seenProfileIDs := map[uint32]bool{}
	for session := range group.players {
		if session == nil || !session.Authenticated {
			continue
		}
		profileID, err := strconv.ParseUint(session.Data["dwc_pid"], 10, 32)
		if err != nil || profileID == 0 {
			continue
		}

		if seenProfileIDs[uint32(profileID)] {
			continue
		}
		targets = append(targets, mkwRaceEndCommandTarget{
			session:   session,
			profileID: uint32(profileID),
		})
		seenProfileIDs[uint32(profileID)] = true
	}
	if len(targets) == 0 {
		return nil, nil
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].profileID < targets[j].profileID
	})

	sender := group.server
	if sender == nil || !group.players[sender] || !sender.Authenticated {
		sender = nil
		for _, target := range targets {
			sender = target.session
			break
		}
	}
	if sender == nil {
		return nil, nil
	}
	return sender, targets
}

func markMKWRaceEndedLocked(group *Group) {
	if group == nil {
		return
	}
	group.MKWCourseID = -1
	group.MKWEngineClassID = -1
}

func deleteStandaloneMatchmakingRoom(roomID string, currentTime int64) (DeletedMatchmakingRoom, bool) {
	session := standaloneRoomSession(roomID, currentTime)
	if session == nil {
		return DeletedMatchmakingRoom{}, false
	}

	deleted := DeletedMatchmakingRoom{
		ID:       roomID,
		GameName: session.Data["gamename"],
	}
	if profileID, err := strconv.ParseUint(session.Data["dwc_pid"], 10, 32); err == nil {
		deleted.PlayerProfileIDs = append(deleted.PlayerProfileIDs, uint32(profileID))
	}

	clearSessionRoomState(session)
	for profileID, assignment := range forcedRoomJoins {
		if assignment.RoomID == roomID {
			delete(forcedRoomJoins, profileID)
		}
	}

	return deleted, true
}

func processResvOK(moduleName string, matchVersion int, reservation common.MatchCommandDataReservation, resvOK common.MatchCommandDataResvOK, sender, destination *Session) bool {
	if len(groups) >= 100000 {
		logging.Error(moduleName, "Hit arbitrary global maximum group count (somehow)")
		return false
	}

	group := sender.groupPointer
	if group == nil {
		group = &Group{
			GroupID:       resvOK.GroupID,
			GroupName:     "",
			CreateTime:    time.Now().UTC(),
			GameName:      sender.Data["gamename"],
			MatchType:     sender.Data["dwc_mtype"],
			MKWRegion:     "",
			LastJoinIndex: 0,
			server:        sender,
			players:       map[*Session]bool{sender: true},
		}

		for {
			groupName := common.RandomString(6)
			if groups[groupName] != nil {
				continue
			}

			group.GroupName = groupName
			break
		}

		if group.GameName == "mariokartwii" {
			rk := sender.Data["rk"]

			// Check and remove regional searches due to the limited player count
			// China (ID 6) gets a pass because it was never released
			if len(rk) == 4 && (strings.HasPrefix(rk, "vs_") || strings.HasPrefix(rk, "bt_")) && rk[3] >= '0' && rk[3] < '6' {
				rk = rk[:2]
			}

			group.MKWRegion = rk
		}

		sender.Data["+joinindex"] = "0"
		if matchVersion == 90 {
			sender.Data["+localplayers"] = strconv.FormatUint(uint64(resvOK.LocalPlayerCount), 10)
		}

		sender.groupPointer = group
		sender.GroupName = group.GroupName
		groups[group.GroupName] = group

		logging.Notice(moduleName, "Created new group", aurora.Cyan(group.GroupName))
		eventData := map[string]any{
			"dwc_group_id": group.GroupID,
			"group_name":   group.GroupName,
			"game_name":    group.GameName,
			"match_type":   group.MatchType,
			"host_id":      sender.Data["dwc_pid"],
		}
		if group.GameName == "mariokartwii" {
			eventData["mario_kart_wii_region"] = group.MKWRegion
		}
		logging.Event("group_created", eventData)
	}

	// Keep group ID updated
	group.GroupID = resvOK.GroupID

	// Set connecting
	sender.Data["+conn_"+destination.Data["+joinindex"]] = "1"
	destination.Data["+conn_"+sender.Data["+joinindex"]] = "1"

	if group.players[destination] {
		// Player is already in the group
		if matchLog, ok := panelMatchLogFromGroupLocked(group); ok {
			go recordPanelMatchLog(matchLog, true)
		}
		return true
	}

	logging.Notice(moduleName, "New player", aurora.BrightCyan(destination.Data["dwc_pid"]), "in group", aurora.Cyan(group.GroupName))

	logging.Event(
		"group_joined",
		map[string]any{
			"dwc_group_id": group.GroupID,
			"group_name":   group.GroupName,
			"profile_id":   destination.Data["dwc_pid"],
		},
	)

	group.LastJoinIndex++
	destination.Data["+joinindex"] = strconv.Itoa(group.LastJoinIndex)
	if matchVersion == 90 {
		destination.Data["+localplayers"] = strconv.FormatUint(uint64(reservation.LocalPlayerCount), 10)
	}

	if destination.groupPointer != nil && destination.groupPointer != group {
		destination.removeFromGroup()
	}

	group.players[destination] = true
	destination.groupPointer = group
	destination.GroupName = group.GroupName

	if matchLog, ok := panelMatchLogFromGroupLocked(group); ok {
		go recordPanelMatchLog(matchLog, true)
	}

	return true
}

func processTellAddr(moduleName string, sender *Session, destination *Session) {
	common.MaybeUnused(moduleName)
	if sender.groupPointer != nil && sender.groupPointer == destination.groupPointer {
		// Just assume the connection is successful if TELL_ADDR is used
		sender.Data["+conn_"+destination.Data["+joinindex"]] = "2"
		destination.Data["+conn_"+sender.Data["+joinindex"]] = "2"
	}
}

func ProcessGPResvOK(matchVersion int, reservation common.MatchCommandDataReservation, resvOK common.MatchCommandDataResvOK, senderIP uint64, senderPid uint32, destIP uint64, destPid uint32) bool {
	senderPidStr := strconv.FormatUint(uint64(senderPid), 10)
	destPidStr := strconv.FormatUint(uint64(destPid), 10)

	moduleName := "QR2:GPMsg:" + senderPidStr + "->" + destPidStr

	mutex.Lock()
	defer mutex.Unlock()

	from := sessions[senderIP]
	if from == nil {
		logging.Error(moduleName, "Sender IP does not exist:", aurora.Cyan(fmt.Sprintf("%012x", senderIP)))
		return false
	}

	to := sessions[destIP]
	if to == nil {
		logging.Error(moduleName, "Destination IP does not exist:", aurora.Cyan(fmt.Sprintf("%012x", destIP)))
		return false
	}

	// Validate dwc_pid values
	if !from.setProfileID(moduleName, senderPidStr, "") {
		return false
	}

	if !to.setProfileID(moduleName, destPidStr, "") {
		return false
	}

	return processResvOK(moduleName, matchVersion, reservation, resvOK, from, to)
}

func ProcessGPTellAddr(senderPid uint32, senderIP uint64, destPid uint32, destIP uint64) {
	senderPidStr := strconv.FormatUint(uint64(senderPid), 10)
	destPidStr := strconv.FormatUint(uint64(destPid), 10)

	moduleName := "QR2:GPMsg:" + senderPidStr + "->" + destPidStr

	mutex.Lock()
	defer mutex.Unlock()

	from := sessions[senderIP]
	if from == nil {
		logging.Error(moduleName, "Sender IP does not exist:", aurora.Cyan(fmt.Sprintf("%012x", senderIP)))
		return
	}

	to := sessions[destIP]
	if to == nil {
		logging.Error(moduleName, "Destination IP does not exist:", aurora.Cyan(fmt.Sprintf("%012x", destIP)))
		return
	}

	// Validate dwc_pid values
	if !from.setProfileID(moduleName, senderPidStr, "") {
		return
	}

	if !to.setProfileID(moduleName, destPidStr, "") {
		return
	}

	processTellAddr(moduleName, from, to)
}

func ProcessGPStatusUpdate(profileID uint32, senderIP uint64, status string) {
	moduleName := "QR2/GPStatus:" + strconv.FormatUint(uint64(profileID), 10)

	mutex.Lock()
	defer mutex.Unlock()

	login, exists := logins[profileID]
	if !exists || login == nil {
		logging.Info(moduleName, "Received status update for non-existent profile ID", aurora.Cyan(profileID))
		return
	}

	session := login.session
	if session == nil {
		if senderIP == 0 {
			logging.Info(moduleName, "Received status update for profile ID", aurora.Cyan(profileID), "but no session exists")
			return
		}

		// Login with this profile ID
		session, exists = sessions[senderIP]
		if !exists || session == nil {
			logging.Info(moduleName, "Received status update for profile ID", aurora.Cyan(profileID), "but no session exists")
			return
		}

		if !session.setProfileID(moduleName, strconv.FormatUint(uint64(profileID), 10), "") {
			return
		}
	}

	// Send the client message exploit if not received yet
	if status != "0" && status != "1" && !session.ExploitReceived && session.login != nil && session.login.NeedsExploit {
		sessionCopy := *session

		mutex.Unlock()
		logging.Notice(moduleName, "Sending SBCM exploit to DNS patcher client")
		sendClientExploit(moduleName, sessionCopy)
		mutex.Lock()
	}

	if status == "0" || status == "1" || status == "3" || status == "4" {
		session := sessions[senderIP]
		if session == nil || session.groupPointer == nil {
			return
		}
		if session.groupPointer.AdminCreated && session.groupPointer.server == session {
			return
		}

		session.removeFromGroup()
	}
}

func checkReservationAllowed(moduleName string, sender, destination *Session, joinType byte) string {
	common.MaybeUnused(moduleName)

	if sender.login == nil || destination.login == nil {
		return ""
	}

	if !sender.login.Restricted && !destination.login.Restricted {
		return "ok"
	}

	if joinType != 2 && joinType != 3 {
		return "restricted_join"
	}

	// Open Host makes the host effectively public for the sender (auto-friends
	// them), so a restricted user must not be able to slip in via friend invite
	// when the sender would be auto-authorized.
	if destination.login.IsOpenHostFor(sender.login.ProfileID) {
		return "restricted_join"
	}

	if destination.groupPointer == nil {
		// Destination is not in a group, check their dwc_mtype instead
		if destination.Data["dwc_mtype"] != "2" && destination.Data["dwc_mtype"] != "3" {
			return "restricted_join"
		}

		// This is fine
		return "ok"
	}

	if destination.groupPointer.MatchType != "private" {
		return "restricted_join"
	}

	return "ok"
}

func CheckGPReservationAllowed(senderIP uint64, senderPid uint32, destPid uint32, joinType byte) string {
	senderPidStr := strconv.FormatUint(uint64(senderPid), 10)
	destPidStr := strconv.FormatUint(uint64(destPid), 10)

	moduleName := "QR2:CheckReservation:" + senderPidStr + "->" + destPidStr

	mutex.Lock()
	defer mutex.Unlock()

	from := sessions[senderIP]
	if from == nil {
		logging.Error(moduleName, "Sender IP does not exist:", aurora.Cyan(fmt.Sprintf("%012x", senderIP)))
		return ""
	}

	toLogin := logins[destPid]
	if toLogin == nil {
		logging.Error(moduleName, "Destination profile ID does not exist:", aurora.Cyan(destPid))
		return ""
	}

	to := toLogin.session
	if to == nil {
		logging.Error(moduleName, "Destination profile ID does not have a session")
		return ""
	}

	// Validate dwc_pid value
	if !from.setProfileID(moduleName, senderPidStr, "") || from.login == nil {
		return ""
	}

	return checkReservationAllowed(moduleName, from, to, joinType)
}

func ProcessNATNEGReport(result byte, ip1 string, ip2 string) {
	moduleName := "QR2:NATNEGReport"

	ip1Lookup := makeLookupAddr(ip1)
	ip2Lookup := makeLookupAddr(ip2)

	mutex.Lock()
	defer mutex.Unlock()

	session1 := sessions[ip1Lookup]
	if session1 == nil {
		logging.Warn(moduleName, "Received NATNEG report for non-existent IP", aurora.Cyan(ip1))
		return
	}

	session2 := sessions[ip2Lookup]
	if session2 == nil {
		logging.Warn(moduleName, "Received NATNEG report for non-existent IP", aurora.Cyan(ip2))
		return
	}

	if session1.groupPointer == nil || session1.groupPointer != session2.groupPointer {
		logging.Error(moduleName, "Received NATNEG report for two IPs in different groups")
		return
	}

	resultString := "3"
	if result == 1 {
		// Success
		resultString = "2"
	}

	session1.Data["+conn_"+session2.Data["+joinindex"]] = resultString
	session2.Data["+conn_"+session1.Data["+joinindex"]] = resultString

	if result != 1 {
		// Increment +conn_fail
		connFail1, _ := strconv.Atoi(session1.Data["+conn_fail"])
		connFail2, _ := strconv.Atoi(session2.Data["+conn_fail"])
		connFail1++
		connFail2++
		session1.Data["+conn_fail"] = strconv.Itoa(connFail1)
		session2.Data["+conn_fail"] = strconv.Itoa(connFail2)
		logging.Event(
			"natneg_failed",
			map[string]any{
				"dwc_group_id": session1.groupPointer.GroupID,
				"group_name":   session1.groupPointer.GroupName,
				"peers": []map[string]string{
					{
						"profile_id":   session1.Data["dwc_pid"],
						"failed_count": strconv.Itoa(connFail1),
						"ip_address":   ip1,
					},
					{
						"profile_id":   session2.Data["dwc_pid"],
						"failed_count": strconv.Itoa(connFail2),
						"ip_address":   ip2,
					},
				},
			},
		)
	} else {
		logging.Event(
			"natneg_succeeded",
			map[string]any{
				"dwc_group_id": session1.groupPointer.GroupID,
				"group_name":   session1.groupPointer.GroupName,
				"peers": []map[string]string{
					{
						"profile_id": session1.Data["dwc_pid"],
						"ip_address": ip1,
					},
					{
						"profile_id": session2.Data["dwc_pid"],
						"ip_address": ip2,
					},
				},
			},
		)
	}
}

func ProcessUSER(senderPid uint32, senderIP uint64, packet []byte) {
	moduleName := "QR2:ProcessUSER/" + strconv.FormatUint(uint64(senderPid), 10)

	mutex.Lock()
	login := logins[senderPid]
	if login == nil {
		mutex.Unlock()
		logging.Warn(moduleName, "Received USER packet from non-existent profile ID", aurora.Cyan(senderPid))
		return
	}

	session := login.session
	if session == nil {
		mutex.Unlock()
		logging.Warn(moduleName, "Received USER packet from profile ID", aurora.Cyan(senderPid), "but no session exists")
		return
	}
	mutex.Unlock()

	miiGroupCount := binary.BigEndian.Uint16(packet[0x04:0x06])
	if miiGroupCount != 2 {
		logging.Error(moduleName, "Received USER packet with unexpected Mii group count", aurora.Cyan(miiGroupCount))
		// Kick the client
		gpErrorCallback(senderPid, "bad_packet")
		return
	}

	miiGroupBitflags := binary.BigEndian.Uint32(packet[0x00:0x04])

	var miiData []string
	var miiName []string
	for i := 0; i < int(miiGroupCount); i++ {
		if miiGroupBitflags&(1<<uint(i)) == 0 {
			continue
		}

		index := 0x08 + i*0x4C
		mii := common.RawMiiFromBytes(packet[index : index+0x4C])
		if mii.CalculateMiiCRC() != 0x0000 {
			logging.Error(moduleName, "Received USER packet with invalid Mii data CRC")
			gpErrorCallback(senderPid, "bad_packet")
			return
		}

		createId := binary.BigEndian.Uint64(packet[index+0x18 : index+0x20])
		official, _ := common.SearchOfficialMiiData(createId)
		if official {
			miiName = append(miiName, "Player")
		} else {
			decodedName, err := common.GetWideString(packet[index+0x2:index+0x2+20], binary.BigEndian)
			if err != nil {
				logging.Error(moduleName, "Failed to parse Mii name:", err)
				gpErrorCallback(senderPid, "bad_packet")
				return
			}

			miiName = append(miiName, decodedName)
		}

		miiData = append(miiData, base64.StdEncoding.EncodeToString(packet[index:index+0x4A]))
	}

	mutex.Lock()
	defer mutex.Unlock()

	for i, name := range miiName {
		session.Data["+mii"+strconv.Itoa(i)] = miiData[i]
		session.Data["+mii_name"+strconv.Itoa(i)] = name
	}
}

// findNewServer attempts to find the new server/host in the group when the current server goes down.
// If no server is found, the group's server pointer is set to nil.
// Expects the mutex to be locked.
func (g *Group) findNewServer() {
	server := (*Session)(nil)
	serverJoinIndex := -1
	for session := range g.players {
		if session.Data["dwc_hoststate"] != "2" {
			continue
		}

		joinIndex, err := strconv.Atoi(session.Data["+joinindex"])
		if err != nil {
			continue
		}

		if server == nil || joinIndex < serverJoinIndex {
			server = session
			serverJoinIndex = joinIndex
		}
	}

	eventData := map[string]any{
		"dwc_group_id": g.GroupID,
		"group_name":   g.GroupName,
	}
	if server != nil {
		eventData["new_host_id"] = server.Data["dwc_pid"]
	}
	if g.server != nil {
		eventData["old_host_id"] = g.server.Data["dwc_pid"]
	}

	g.server = server
	g.updateMatchType()

	logging.Event("group_host_changed", eventData)
	if matchLog, ok := panelMatchLogFromGroupLocked(g); ok {
		go recordPanelMatchLog(matchLog, true)
	}
}

// updateMatchType updates the match type of the group based on the host's dwc_mtype value.
// Expects the mutex to be locked.
func (g *Group) updateMatchType() {
	if g.server == nil || g.server.Data["dwc_mtype"] == "" {
		return
	}

	g.MatchType = g.server.Data["dwc_mtype"]
}

func ProcessMKWSelectRecord(profileId uint32, key string, value string) {
	moduleName := "QR2:MKWSelectRecord:" + strconv.FormatUint(uint64(profileId), 10)

	mutex.Lock()
	login := logins[profileId]
	if login == nil {
		mutex.Unlock()
		logging.Warn(moduleName, "Received SELECT record from non-existent profile ID", aurora.Cyan(profileId))
		return
	}

	session := login.session
	if session == nil {
		mutex.Unlock()
		logging.Warn(moduleName, "Received SELECT record  from profile ID", aurora.Cyan(profileId), "but no session exists")
		return
	}
	mutex.Unlock()

	keyColored := aurora.BrightCyan(key).String()

	switch key {
	case "wl:mkw_select_course":
		courseId, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			logging.Error(moduleName, "Error decoding", keyColored+":", err.Error())
			return
		}

		logging.Info(moduleName, "Selected course", aurora.BrightCyan(strconv.FormatUint(courseId, 10)))

		mutex.Lock()
		group := session.groupPointer
		var matchLog database.PanelMatchLog
		var shouldRecord bool
		if group != nil {
			group.MKWRaceNumber++
			group.MKWCourseID = int(courseId)
			group.MKWEngineClassID = -1
			// A new race started, so a previous race-end request is done.
			group.mkwRaceEnd = nil
			matchLog, shouldRecord = panelMatchLogFromGroupLocked(group)
		}
		mutex.Unlock()

		recordPanelMatchLog(matchLog, shouldRecord)
		return

	case "wl:mkw_select_cc":
		ccId, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			logging.Error(moduleName, "Error decoding", keyColored+":", err.Error())
			return
		}

		logging.Info(moduleName, "Selected CC", aurora.BrightCyan(strconv.FormatUint(ccId, 10)))

		mutex.Lock()
		group := session.groupPointer
		var matchLog database.PanelMatchLog
		var shouldRecord bool
		if group != nil && mkwRaceActive(group) {
			group.MKWEngineClassID = int(ccId)
			matchLog, shouldRecord = panelMatchLogFromGroupLocked(group)
		}
		mutex.Unlock()

		recordPanelMatchLog(matchLog, shouldRecord)
		return
	}

}

func ProcessMKWFinishTimeReport(profileID uint32, value string) {
	moduleName := "QR2:MKWFinishTime:" + strconv.FormatUint(uint64(profileID), 10)

	packet, err := common.Base64DwcEncoding.DecodeString(value)
	if err != nil {
		logging.Error(moduleName, "Error decoding finish time report:", err.Error())
		return
	}
	if len(packet) != 12 {
		logging.Error(moduleName, "Invalid finish time report length:", len(packet))
		return
	}

	inGameTime := binary.BigEndian.Uint32(packet[0:4])
	finishTime := binary.BigEndian.Uint32(packet[4:8])
	difference := binary.BigEndian.Uint32(packet[8:12])
	logging.Info(moduleName, "Finish time report", aurora.Cyan(finishTime), "in-game", aurora.Cyan(inGameTime), "difference", aurora.Cyan(difference))
}

// panelMatchLogFromGroupLocked snapshots the current Mario Kart Wii race while the QR2 mutex is held.
func panelMatchLogFromGroupLocked(group *Group) (database.PanelMatchLog, bool) {
	if group == nil || group.GameName != "mariokartwii" {
		return database.PanelMatchLog{}, false
	}
	raceNumber := group.MKWRaceNumber
	courseID := group.MKWCourseID
	engineClassID := group.MKWEngineClassID
	if !mkwRaceActive(group) {
		raceNumber = 0
		courseID = -1
		engineClassID = -1
	}

	matchLog := database.PanelMatchLog{
		SourceKey:     fmt.Sprintf("%s:%d:%d", group.GroupName, group.GroupID, raceNumber),
		RoomID:        group.GroupName,
		DWCGroupID:    uint64(group.GroupID),
		GameName:      group.GameName,
		MatchType:     matchTypeLabel(group.MatchType),
		MKWRegion:     group.MKWRegion,
		RaceNumber:    raceNumber,
		CourseID:      courseID,
		EngineClassID: engineClassID,
	}

	if group.server != nil {
		matchLog.HostName = sessionName(group.server)
		if profileID, err := strconv.ParseUint(group.server.Data["dwc_pid"], 10, 32); err == nil {
			matchLog.HostProfileID = uint32(profileID)
		}
	}

	players := make([]database.PanelMatchLogPlayer, 0, len(group.players))
	for session := range group.players {
		player := playerInfoFromSession(session, group)
		logPlayer := database.PanelMatchLogPlayer{
			JoinIndex:    session.Data["+joinindex"],
			FriendCode:   player.FriendCode,
			InGameName:   player.InGameName,
			LocalPlayers: player.Count,
			ConnFail:     player.ConnFail,
			VersusELO:    player.VersusELO,
			BattleELO:    player.BattleELO,
		}
		if profileID, err := strconv.ParseUint(player.ProfileID, 10, 32); err == nil {
			logPlayer.ProfileID = uint32(profileID)
		}
		players = append(players, logPlayer)
	}

	sort.Slice(players, func(i, j int) bool {
		left, leftErr := strconv.Atoi(players[i].JoinIndex)
		right, rightErr := strconv.Atoi(players[j].JoinIndex)
		if leftErr == nil && rightErr == nil {
			return left < right
		}
		return players[i].JoinIndex < players[j].JoinIndex
	})

	matchLog.Players = players
	matchLog.PlayerCount = len(players)
	return matchLog, true
}

func recordPanelMatchLog(matchLog database.PanelMatchLog, ok bool) {
	if !ok || !matchLogStoreReady.Load() {
		return
	}

	if err := db.RecordPanelMatchLog(matchLog); err != nil {
		logging.Error("QR2", "Failed to record match log:", err)
		return
	}

	logging.Event("panel_match_log_recorded", map[string]any{
		"room_id":    matchLog.RoomID,
		"game_name":  matchLog.GameName,
		"match_type": matchLog.MatchType,
	})
}

// saveGroups saves the current groups state to disk.
// Expects the mutex to be locked.
func saveGroups() error {
	file, err := os.OpenFile("state/qr2_groups.gob", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() {
		common.ShouldNotError(file.Close())
	}()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(groups)
}

// loadGroups loads the groups state from disk.
// Expects the mutex to be locked, and the sessions to already be loaded.
func loadGroups() error {
	file, err := os.Open("state/qr2_groups.gob")
	if err != nil {
		return err
	}
	defer func() {
		common.ShouldNotError(file.Close())
	}()

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&groups)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		if session.groupPointer != nil || session.GroupName == "" {
			continue
		}

		group := groups[session.GroupName]
		if group == nil {
			logging.Warn("QR2", "Session", aurora.BrightCyan(session.Addr.String()), "has a group name but the group does not exist")
			continue
		}

		if group.players == nil {
			group.players = map[*Session]bool{}
		}

		group.players[session] = true
		session.groupPointer = group
	}

	for _, group := range groups {
		if group.players == nil {
			group.players = map[*Session]bool{}
		}

		group.findNewServer()
	}

	return nil
}
