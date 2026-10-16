package qr2

import (
	"dwc-server-advanced/common"
	"reflect"
	"testing"
	"time"
)

func TestStandaloneRoomKeepsCreationTime(t *testing.T) {
	created := time.Date(2026, time.October, 16, 12, 0, 0, 0, time.UTC)
	session := &Session{CreatedAt: created, LastKeepAlive: created.Add(time.Hour).Unix(), Data: map[string]string{"gamename": "mariokartwii", "dwc_pid": "1"}}
	room := roomInfoFromStandaloneSession(session)
	if !room.CreateTime.Equal(created) {
		t.Fatalf("room creation time = %v, want %v", room.CreateTime, created)
	}
}

func TestMarioKartWiiActiveRacePlayersArePlaying(t *testing.T) {
	tests := []struct {
		name      string
		hostState string
		suspend   string
	}{
		{name: "host marked available", hostState: "2", suspend: "0"},
		{name: "host marked suspended", hostState: "2", suspend: "1"},
		{name: "peer marked suspended", hostState: "1", suspend: "1"},
		{name: "peer marked waiting", hostState: "1", suspend: "0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := playerStateFromValues("mariokartwii", test.hostState, test.suspend, true)
			if got != "playing" {
				t.Fatalf("playerStateFromValues() = %q, want playing", got)
			}
		})
	}
}

func TestRoomStateFromGroup(t *testing.T) {
	tests := []struct {
		name  string
		group *Group
		want  string
	}{
		{name: "nil group", group: nil, want: "matchmaking"},
		{name: "mkw lobby", group: &Group{GameName: "mariokartwii"}, want: "waiting_for_course_selection"},
		{name: "mkw race", group: &Group{GameName: "mariokartwii", MKWRaceNumber: 1, MKWCourseID: 8}, want: "playing"},
		{name: "mkw ended race", group: &Group{GameName: "mariokartwii", MKWRaceNumber: 1, MKWCourseID: -1}, want: "waiting_for_course_selection"},
		{name: "generic room", group: &Group{GameName: "othergame"}, want: "waiting_for_lane_selection"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := roomStateFromGroup(test.group)
			if got != test.want {
				t.Fatalf("roomStateFromGroup() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestEndMKWRaceReturnsRoomToCourseSelection(t *testing.T) {
	mutex.Lock()
	previousGroups := groups
	groups = map[string]*Group{
		"room1": {
			GroupName:        "room1",
			GameName:         "mariokartwii",
			MKWRaceNumber:    3,
			MKWCourseID:      12,
			MKWEngineClassID: 2,
			players:          map[*Session]bool{},
		},
	}
	mutex.Unlock()
	defer func() {
		mutex.Lock()
		groups = previousGroups
		mutex.Unlock()
	}()

	ended, ok, message := EndMKWRace("room1", MKWRaceEndModeTrackingOnly)
	if !ok {
		t.Fatalf("EndMKWRace() failed: %s", message)
	}
	if ended.RaceInfo.RaceNumber != 3 || ended.RaceInfo.CourseID != 12 || ended.RaceInfo.EngineClassID != 2 {
		t.Fatalf("EndMKWRace() race = %+v", ended.RaceInfo)
	}

	mutex.Lock()
	group := groups["room1"]
	state := roomStateFromGroup(group)
	active := mkwRaceActive(group)
	raceNumber := group.MKWRaceNumber
	courseID := group.MKWCourseID
	engineClassID := group.MKWEngineClassID
	mutex.Unlock()

	if active {
		t.Fatal("mkwRaceActive() = true after ending race")
	}
	if state != "waiting_for_course_selection" {
		t.Fatalf("roomStateFromGroup() = %q, want waiting_for_course_selection", state)
	}
	if raceNumber != 3 || courseID != -1 || engineClassID != -1 {
		t.Fatalf("group race state = race %d course %d cc %d", raceNumber, courseID, engineClassID)
	}
}

func TestEndMKWRaceForceDisconnectRequiresPeerLinks(t *testing.T) {
	mutex.Lock()
	previousGroups := groups
	groups = map[string]*Group{
		"room1": {
			GroupName:        "room1",
			GameName:         "mariokartwii",
			MKWRaceNumber:    3,
			MKWCourseID:      12,
			MKWEngineClassID: 2,
			players:          map[*Session]bool{},
		},
	}
	mutex.Unlock()
	defer func() {
		mutex.Lock()
		groups = previousGroups
		mutex.Unlock()
	}()

	_, ok, message := EndMKWRace("room1", MKWRaceEndModeForceDisconnect)
	if ok {
		t.Fatal("EndMKWRace() with force disconnect succeeded without peer links")
	}
	if message != "no eligible in-game peer links to force-close" {
		t.Fatalf("EndMKWRace() message = %q", message)
	}
}

func TestBuildMKWRacePayloadEndMessagesUsesWWFCCommand(t *testing.T) {
	host := &Session{
		Authenticated: true,
		Data:          map[string]string{"dwc_pid": "1001"},
		SearchID:      (uint64(29900) << 32) | 0x7f000001,
	}
	peer := &Session{
		Authenticated: true,
		Data:          map[string]string{"dwc_pid": "1002"},
		SearchID:      (uint64(29901) << 32) | 0x7f000002,
	}
	group := &Group{
		GameName: "mariokartwii",
		server:   host,
		players: map[*Session]bool{
			host: true,
			peer: true,
		},
	}

	mutex.Lock()
	messages := buildMKWRacePayloadEndMessagesLocked(group)
	mutex.Unlock()

	if len(messages) != 2 {
		t.Fatalf("buildMKWRacePayloadEndMessagesLocked() made %d messages, want 2", len(messages))
	}
	if messages[0].receiver != host {
		t.Fatal("first message receiver is not host")
	}
	if messages[1].receiver != peer {
		t.Fatal("second message receiver is not peer")
	}

	for _, msg := range messages {
		message := msg.message
		if len(message) != 0x14 {
			t.Fatalf("message length = %d, want 20", len(message))
		}
		if message[8] != common.MatchWWFCMKWRaceEnd {
			t.Fatalf("message command = 0x%02x, want WWFC_MKW_RACE_END", message[8])
		}
		data, ok := common.DecodeMatchCommand(message[8], message[0x14:], 90)
		if !ok {
			t.Fatal("failed to decode generated WWFC_MKW_RACE_END message")
		}
		if data.Command != common.MatchWWFCMKWRaceEnd {
			t.Fatalf("decoded command = 0x%02x, want WWFC_MKW_RACE_END", data.Command)
		}
	}
}

func TestBuildMKWRaceForceDisconnectMessagesUsesServerCloseClient(t *testing.T) {
	host := &Session{
		Authenticated: true,
		Data:          map[string]string{"dwc_pid": "1001"},
		SearchID:      (uint64(29900) << 32) | 0x7f000001,
	}
	peer := &Session{
		Authenticated: true,
		Data:          map[string]string{"dwc_pid": "1002"},
		SearchID:      (uint64(29901) << 32) | 0x7f000002,
	}
	group := &Group{
		GameName: "mariokartwii",
		server:   host,
		players: map[*Session]bool{
			host: true,
			peer: true,
		},
	}

	mutex.Lock()
	messages := buildMKWRaceForceDisconnectMessagesLocked(group)
	mutex.Unlock()

	if len(messages) != 2 {
		t.Fatalf("buildMKWRaceForceDisconnectMessagesLocked() made %d messages, want 2", len(messages))
	}

	if messages[0].receiver != host {
		t.Fatal("first message receiver is not host")
	}
	if messages[1].receiver != peer {
		t.Fatal("second message receiver is not peer")
	}

	for _, test := range []struct {
		name string
		msg  queuedServerMatchMessage
		want []uint32
	}{
		{name: "host closes peer", msg: messages[0], want: []uint32{1002}},
		{name: "peer closes host", msg: messages[1], want: []uint32{1001}},
	} {
		t.Run(test.name, func(t *testing.T) {
			message := test.msg.message
			if len(message) < 0x14 || message[8] != common.MatchServerCloseClient {
				t.Fatalf("message command = 0x%02x, want SC_CLOSE_CL", message[8])
			}
			data, ok := common.DecodeMatchCommand(message[8], message[0x14:], 90)
			if !ok || data.ServerCloseClient == nil {
				t.Fatal("failed to decode generated SC_CLOSE_CL message")
			}
			if !reflect.DeepEqual(data.ServerCloseClient.ProfileIDs, test.want) {
				t.Fatalf("SC_CLOSE_CL profile IDs = %v, want %v", data.ServerCloseClient.ProfileIDs, test.want)
			}
		})
	}
}
