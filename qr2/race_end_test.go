package qr2

import (
	"dwc-server-advanced/common"
	"testing"
	"time"
)

// raceEndTestRoom builds a Mario Kart Wii room in the middle of a race with a
// payload host (1001) and a vanilla peer (1002).
func raceEndTestRoom(t *testing.T) (*Group, *Session, *Session) {
	t.Helper()
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
		GroupName:        "room1",
		GameName:         "mariokartwii",
		MKWRaceNumber:    2,
		MKWCourseID:      8,
		MKWEngineClassID: 1,
		server:           host,
		players:          map[*Session]bool{host: true, peer: true},
	}
	host.groupPointer = group
	peer.groupPointer = group

	mutex.Lock()
	previousGroups, previousLogins := groups, logins
	groups = map[string]*Group{"room1": group}
	logins = map[uint32]*LoginInfo{
		1001: {ProfileID: 1001, PayloadFeatures: true, session: host},
		1002: {ProfileID: 1002, session: peer},
	}
	host.login, peer.login = logins[1001], logins[1002]
	mutex.Unlock()

	t.Cleanup(func() {
		mutex.Lock()
		groups, logins = previousGroups, previousLogins
		mutex.Unlock()
	})
	return group, host, peer
}

func TestAutoRaceEndSplitsPayloadAndVanillaClients(t *testing.T) {
	group, host, peer := raceEndTestRoom(t)

	mutex.Lock()
	messages, payloadIDs, vanillaIDs := buildMKWRaceAutoEndMessagesLocked(group)
	mutex.Unlock()

	if len(messages) != 2 || len(payloadIDs) != 1 || payloadIDs[0] != 1001 || len(vanillaIDs) != 1 || vanillaIDs[0] != 1002 {
		t.Fatalf("got %d messages, payload %v, vanilla %v", len(messages), payloadIDs, vanillaIDs)
	}
	for _, msg := range messages {
		switch msg.receiver {
		case host:
			if msg.message[8] != common.MatchWWFCMKWRaceEnd {
				t.Fatalf("payload client got command 0x%02x", msg.message[8])
			}
		case peer:
			if msg.message[8] != common.MatchServerCloseClient {
				t.Fatalf("vanilla client got command 0x%02x", msg.message[8])
			}
			data, ok := common.DecodeMatchCommand(msg.message[8], msg.message[0x14:], 90)
			if !ok || data.ServerCloseClient == nil || len(data.ServerCloseClient.ProfileIDs) != 1 || data.ServerCloseClient.ProfileIDs[0] != 1001 {
				t.Fatalf("SC_CLOSE_CL data = %+v", data.ServerCloseClient)
			}
		default:
			t.Fatal("message sent to an unknown session")
		}
	}
}

func TestAutoRaceEndTracksResultsAndFallsBack(t *testing.T) {
	previousDelay := mkwRaceEndFallbackDelay
	mkwRaceEndFallbackDelay = time.Hour // the test runs the fallback by hand
	t.Cleanup(func() { mkwRaceEndFallbackDelay = previousDelay })

	group, _, _ := raceEndTestRoom(t)

	ended, ok, message := EndMKWRace("room1", MKWRaceEndModeAuto)
	if !ok {
		t.Fatalf("EndMKWRace() failed: %s", message)
	}
	if ended.PayloadTargets != 1 || ended.DisconnectTargets != 1 || ended.FallbackSeconds != 3600 {
		t.Fatalf("EndMKWRace() = %+v", ended)
	}

	mutex.Lock()
	request := group.mkwRaceEnd
	status := mkwRaceEndStatusLocked(group)
	mutex.Unlock()
	if request == nil || status == nil || status.Mode != MKWRaceEndModeAuto || !request.pending[1001] {
		t.Fatalf("race end request not tracked: %+v", status)
	}

	ProcessMKWRaceEndReport(1001, "native_timeout")
	runMKWRaceEndFallback(group, request)

	mutex.Lock()
	status = mkwRaceEndStatusLocked(group)
	mutex.Unlock()
	if !status.Fallback || status.Results["1001"] != "native_timeout" || status.Disconnect != 2 {
		t.Fatalf("after fallback status = %+v", status)
	}
}

func TestAutoRaceEndConfirmedSkipsFallback(t *testing.T) {
	previousDelay := mkwRaceEndFallbackDelay
	mkwRaceEndFallbackDelay = time.Hour
	t.Cleanup(func() { mkwRaceEndFallbackDelay = previousDelay })

	group, _, _ := raceEndTestRoom(t)
	if _, ok, message := EndMKWRace("room1", MKWRaceEndModeAuto); !ok {
		t.Fatalf("EndMKWRace() failed: %s", message)
	}

	mutex.Lock()
	request := group.mkwRaceEnd
	mutex.Unlock()

	ProcessMKWRaceEndReport(1001, "finished")
	runMKWRaceEndFallback(group, request)

	mutex.Lock()
	status := mkwRaceEndStatusLocked(group)
	mutex.Unlock()
	if status.Fallback || status.Finished != 1 {
		t.Fatalf("status = %+v, want finished without fallback", status)
	}
}
