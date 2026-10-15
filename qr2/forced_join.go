package qr2

import (
	"strings"
	"time"
)

type ForcedRoomJoin struct {
	ProfileID uint32    `json:"pid"`
	RoomID    string    `json:"room_id"`
	GameName  string    `json:"game"`
	ExpiresAt time.Time `json:"expires"`
}

var forcedRoomJoins = map[uint32]ForcedRoomJoin{}

func ForceJoinRoom(profileID uint32, roomID string, ttl time.Duration) (ForcedRoomJoin, bool) {
	roomID = strings.TrimSpace(roomID)
	if profileID == 0 || roomID == "" || ttl <= 0 {
		return ForcedRoomJoin{}, false
	}

	now := time.Now().UTC()

	mutex.Lock()
	defer mutex.Unlock()

	cleanupExpiredForcedRoomJoins(now)

	serverSession := forcedRoomSession(roomID, now)
	if serverSession == nil {
		return ForcedRoomJoin{}, false
	}

	assignment := ForcedRoomJoin{
		ProfileID: profileID,
		RoomID:    roomID,
		GameName:  serverSession.Data["gamename"],
		ExpiresAt: now.Add(ttl),
	}
	forcedRoomJoins[profileID] = assignment

	return assignment, true
}

func GetForcedRoomServer(profileID uint32, queryGame string) (map[string]string, ForcedRoomJoin, bool) {
	if profileID == 0 {
		return nil, ForcedRoomJoin{}, false
	}

	now := time.Now().UTC()

	mutex.Lock()
	defer mutex.Unlock()

	cleanupExpiredForcedRoomJoins(now)

	assignment, exists := forcedRoomJoins[profileID]
	if !exists || !assignment.ExpiresAt.After(now) {
		return nil, ForcedRoomJoin{}, false
	}
	if assignment.GameName != queryGame {
		return nil, ForcedRoomJoin{}, false
	}

	serverSession := forcedRoomSession(assignment.RoomID, now)
	if serverSession == nil {
		delete(forcedRoomJoins, profileID)
		return nil, ForcedRoomJoin{}, false
	}

	server := make(map[string]string, len(serverSession.Data))
	for key, value := range serverSession.Data {
		server[key] = value
	}

	return server, assignment, true
}

func forcedRoomSession(roomID string, now time.Time) *Session {
	if group := groups[roomID]; group != nil {
		return forcedRoomServer(group, now)
	}

	return standaloneRoomSession(roomID, now.Unix())
}

func forcedRoomServer(group *Group, now time.Time) *Session {
	if group.server == nil || !group.server.Authenticated || group.server.LastKeepAlive < now.Unix()-60 {
		group.findNewServer()
	}
	if group.server == nil || !group.server.Authenticated || group.server.LastKeepAlive < now.Unix()-60 || group.server.Data["+deviceauth"] != "1" {
		return nil
	}

	return group.server
}

func cleanupExpiredForcedRoomJoins(now time.Time) {
	for profileID, assignment := range forcedRoomJoins {
		if !assignment.ExpiresAt.After(now) {
			delete(forcedRoomJoins, profileID)
		}
	}
}
