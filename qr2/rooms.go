package qr2

import (
	"dwc-server-advanced/common"
	"sort"
	"strconv"
	"time"
)

type RoomInfo struct {
	ID            string                `json:"id"`
	GameName      string                `json:"game"`
	CreateTime    time.Time             `json:"created"`
	MatchType     string                `json:"type"`
	State         string                `json:"state"`
	HostProfileID string                `json:"host_pid,omitempty"`
	HostName      string                `json:"host_name,omitempty"`
	HostState     string                `json:"host_state,omitempty"`
	MKWRegion     string                `json:"rk,omitempty"`
	Players       map[string]PlayerInfo `json:"players"`
	RaceInfo      *RaceInfo             `json:"race,omitempty"`
	RaceEnd       *MKWRaceEndStatus     `json:"race_end,omitempty"`
}

func GetRooms(sorted bool) []RoomInfo {
	currentTime := time.Now().UTC().Unix()

	mutex.Lock()
	defer mutex.Unlock()

	rooms := []RoomInfo{}

	for _, group := range groups {
		room := RoomInfo{
			ID:         group.GroupName,
			GameName:   group.GameName,
			CreateTime: group.CreateTime,
			MatchType:  matchTypeLabel(group.MatchType),
			State:      roomStateFromGroup(group),
			MKWRegion:  group.MKWRegion,
			Players:    map[string]PlayerInfo{},
		}

		if group.server != nil {
			room.HostProfileID = group.server.Data["dwc_pid"]
			room.HostName = sessionName(group.server)
			room.HostState = group.server.Data["dwc_hoststate"]
		}

		if mkwRaceActive(group) {
			room.RaceInfo = &RaceInfo{
				RaceNumber:    group.MKWRaceNumber,
				CourseID:      group.MKWCourseID,
				EngineClassID: group.MKWEngineClassID,
			}
		}

		room.RaceEnd = mkwRaceEndStatusLocked(group)

		for session := range group.players {
			if !session.Authenticated || session.LastKeepAlive < currentTime-60 {
				continue
			}

			joinIndex := session.Data["+joinindex"]
			if joinIndex == "" {
				joinIndex = strconv.Itoa(len(room.Players))
			}

			room.Players[joinIndex] = playerInfoFromSession(session, group)
		}

		if len(room.Players) > 0 {
			rooms = append(rooms, room)
		}
	}

	for _, session := range sessions {
		if !isStandaloneRoomSession(session, currentTime) {
			continue
		}

		room := roomInfoFromStandaloneSession(session)
		if room.ID != "" {
			rooms = append(rooms, room)
		}
	}

	if sorted {
		sort.Slice(rooms, func(i, j int) bool {
			if rooms[i].CreateTime.Equal(rooms[j].CreateTime) {
				return rooms[i].ID < rooms[j].ID
			}

			return rooms[i].CreateTime.Before(rooms[j].CreateTime)
		})
	}

	return rooms
}

func roomInfoFromStandaloneSession(session *Session) RoomInfo {
	joinIndex := session.Data["+joinindex"]
	if joinIndex == "" {
		joinIndex = "0"
	}

	room := RoomInfo{
		ID:            standaloneRoomID(session),
		GameName:      session.Data["gamename"],
		CreateTime:    time.Unix(session.LastKeepAlive, 0).UTC(),
		MatchType:     matchTypeLabel(session.Data["dwc_mtype"]),
		State:         "matchmaking",
		HostProfileID: session.Data["dwc_pid"],
		HostName:      sessionName(session),
		HostState:     session.Data["dwc_hoststate"],
		Players:       map[string]PlayerInfo{},
	}

	if room.GameName == "mariokartwii" {
		room.MKWRegion = normalizedMKWRegion(session.Data["rk"])
	}

	room.Players[joinIndex] = playerInfoFromSession(session, nil)
	return room
}

func playerInfoFromSession(session *Session, group *Group) PlayerInfo {
	player := PlayerInfo{
		Count:      session.Data["+localplayers"],
		ProfileID:  session.Data["dwc_pid"],
		InGameName: sessionName(session),
		State:      playerStateFromSession(session, group),
		ConnFail:   session.Data["+conn_fail"],
		Suspend:    session.Data["dwc_suspend"],
	}

	if player.Count == "" {
		player.Count = "1"
	}
	if player.ConnFail == "" {
		player.ConnFail = "0"
	}

	pid, err := strconv.ParseUint(session.Data["dwc_pid"], 10, 32)
	if err == nil {
		if fcGame := session.Data["+fcgameid"]; len(fcGame) == 4 {
			player.FriendCode = common.CalcFriendCodeString(uint32(pid), fcGame)
		}
	}

	if session.Data["gamename"] == "mariokartwii" {
		player.VersusELO = session.Data["ev"]
		player.BattleELO = session.Data["eb"]
	}

	for i := 0; i < 32; i++ {
		miiData := session.Data["+mii"+strconv.Itoa(i)]
		if miiData == "" {
			continue
		}

		player.Mii = append(player.Mii, MiiInfo{
			MiiData: miiData,
			MiiName: session.Data["+mii_name"+strconv.Itoa(i)],
		})
	}

	if group != nil {
		for other := range group.players {
			if other == session {
				continue
			}

			joinIndex := other.Data["+joinindex"]
			if joinIndex == "" {
				continue
			}

			if session.Data["+conn_"+joinIndex] == "" {
				player.ConnMap += "0"
				continue
			}

			player.ConnMap += session.Data["+conn_"+joinIndex]
		}
	}

	return player
}

func playerStateFromSession(session *Session, group *Group) string {
	if session == nil {
		return "waiting"
	}

	raceActive := mkwRaceActive(group)
	return playerStateFromValues(session.Data["gamename"], session.Data["dwc_hoststate"], session.Data["dwc_suspend"], raceActive)
}

func playerStateFromValues(gameName string, hostState string, suspend string, raceActive bool) string {
	if gameName == "mariokartwii" {
		if raceActive {
			return "playing"
		}
		return "waiting_for_course_selection"
	}

	if suspend == "1" {
		return "spectating"
	}
	if hostState == "2" {
		return "playing"
	}
	return "waiting_for_lane_selection"
}

func mkwRaceActive(group *Group) bool {
	return group != nil && group.GameName == "mariokartwii" && group.MKWRaceNumber > 0 && group.MKWCourseID >= 0
}

func roomStateFromGroup(group *Group) string {
	if group == nil {
		return "matchmaking"
	}
	if mkwRaceActive(group) {
		return "playing"
	}
	if group.GameName == "mariokartwii" {
		return "waiting_for_course_selection"
	}
	return "waiting_for_lane_selection"
}

func sessionName(session *Session) string {
	if session == nil {
		return ""
	}

	if session.login != nil && session.login.InGameName != "" {
		return session.login.InGameName
	}

	return session.Data["+ingamesn"]
}

func matchTypeLabel(value string) string {
	switch value {
	case "0", "1", "anybody", "public":
		return "public"
	case "2", "3", "private":
		return "private"
	default:
		return "unknown"
	}
}
