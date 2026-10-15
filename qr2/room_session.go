package qr2

import (
	"strconv"
	"strings"
)

const standaloneRoomPrefix = "session:"

func standaloneRoomID(session *Session) string {
	if session == nil || session.SearchID == 0 {
		return ""
	}

	return standaloneRoomPrefix + strconv.FormatUint(session.SearchID, 10)
}

// standaloneRoomSession expects the QR2 mutex to be locked.
func standaloneRoomSession(roomID string, currentTime int64) *Session {
	if !strings.HasPrefix(roomID, standaloneRoomPrefix) {
		return nil
	}

	searchID, err := strconv.ParseUint(strings.TrimPrefix(roomID, standaloneRoomPrefix), 10, 64)
	if err != nil || searchID == 0 {
		return nil
	}

	session := sessionBySearchID[searchID]
	if !isStandaloneRoomSession(session, currentTime) {
		return nil
	}

	return session
}

func isStandaloneRoomSession(session *Session, currentTime int64) bool {
	return session != nil && session.groupPointer == nil && isVisibleRoomSession(session, currentTime)
}

func isVisibleRoomSession(session *Session, currentTime int64) bool {
	if session == nil || !session.Authenticated || session.LastKeepAlive < currentTime-60 {
		return false
	}
	if session.Data["gamename"] == "" || session.Data["dwc_pid"] == "" {
		return false
	}
	if session.Data["+deviceauth"] != "1" {
		return false
	}
	if session.Data["dwc_suspend"] == "1" {
		return false
	}
	if matchTypeLabel(session.Data["dwc_mtype"]) == "unknown" {
		return false
	}

	hostState := session.Data["dwc_hoststate"]
	return hostState == "0" || hostState == "2"
}

func clearSessionRoomState(session *Session) {
	for field := range session.Data {
		if strings.HasPrefix(field, "+conn_") {
			delete(session.Data, field)
		}
	}
	delete(session.Data, "+joinindex")
	session.Data["dwc_hoststate"] = "0"
	session.Data["dwc_suspend"] = "1"
	session.Data["numplayers"] = "0"
	session.groupPointer = nil
	session.GroupName = ""
}
