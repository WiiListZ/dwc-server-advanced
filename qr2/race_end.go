package qr2

import (
	"dwc-server-advanced/common"
	"dwc-server-advanced/logging"
	"strconv"
	"time"

	"github.com/logrusorgru/aurora/v3"
)

// MKWRaceEndModeAuto sends the WWFC race-end command to clients that run the
// payload and SC_CLOSE_CL to the others. Payload clients that do not confirm
// the end within mkwRaceEndFallbackDelay get SC_CLOSE_CL too.
const MKWRaceEndModeAuto = "auto"

var (
	mkwRaceEndFallbackDelay = 20 * time.Second
	mkwRaceEndStatusTTL     = 3 * time.Minute
)

// MKWRaceEndStatus describes the last race-end request of a room.
type MKWRaceEndStatus struct {
	Requested  time.Time `json:"requested"`
	Mode       string    `json:"mode"`
	Payload    int       `json:"payload"`
	Disconnect int       `json:"disconnect"`
	Finished   int       `json:"finished"`
	Fallback   bool      `json:"fallback"`
	// Results holds the last result each payload client reported, by profile ID.
	Results map[string]string `json:"results,omitempty"`
}

type mkwRaceEndRequest struct {
	status MKWRaceEndStatus
	// pending holds the payload clients that have not confirmed the end yet.
	pending map[uint32]bool
}

func sessionHasPayload(profileID uint32) bool {
	login := logins[profileID]
	return login != nil && login.PayloadFeatures
}

// buildCloseLinksMessagesLocked tells each receiver to close its links to
// every other participant. A player left alone in a race has it ended by the
// game, which works on clients without the payload.
func buildCloseLinksMessagesLocked(sender *Session, participants []mkwRaceEndCommandTarget, receivers []mkwRaceEndCommandTarget) []queuedServerMatchMessage {
	messages := []queuedServerMatchMessage{}
	for _, receiver := range receivers {
		closeProfileIDs := make([]uint32, 0, len(participants))
		for _, participant := range participants {
			if participant.profileID != receiver.profileID {
				closeProfileIDs = append(closeProfileIDs, participant.profileID)
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
		messages = append(messages, queuedServerMatchMessage{receiver: receiver.session, message: message})
	}
	return messages
}

// buildMKWRaceAutoEndMessagesLocked splits the room into payload clients,
// which get the WWFC race-end command, and the others, which get SC_CLOSE_CL.
func buildMKWRaceAutoEndMessagesLocked(group *Group) ([]queuedServerMatchMessage, []uint32, []uint32) {
	if group == nil || group.GameName != "mariokartwii" {
		return nil, nil, nil
	}

	sender, targets := mkwRaceEndCommandParticipantsLocked(group)
	if sender == nil || len(targets) == 0 {
		return nil, nil, nil
	}

	payloadTargets := []mkwRaceEndCommandTarget{}
	vanillaTargets := []mkwRaceEndCommandTarget{}
	for _, target := range targets {
		if sessionHasPayload(target.profileID) {
			payloadTargets = append(payloadTargets, target)
		} else {
			vanillaTargets = append(vanillaTargets, target)
		}
	}

	messages := []queuedServerMatchMessage{}
	payloadIDs := []uint32{}
	if len(payloadTargets) > 0 {
		message, ok := buildServerMatchCommandMessageLocked(sender, common.MatchWWFCMKWRaceEnd, common.MatchCommandData{Version: 90})
		if ok {
			for _, target := range payloadTargets {
				messages = append(messages, queuedServerMatchMessage{receiver: target.session, message: message})
				payloadIDs = append(payloadIDs, target.profileID)
			}
		}
	}

	closeMessages := buildCloseLinksMessagesLocked(sender, targets, vanillaTargets)
	messages = append(messages, closeMessages...)
	vanillaIDs := []uint32{}
	for _, target := range vanillaTargets {
		vanillaIDs = append(vanillaIDs, target.profileID)
	}
	if len(closeMessages) == 0 {
		vanillaIDs = nil
	}

	return messages, payloadIDs, vanillaIDs
}

func startMKWRaceEndRequestLocked(group *Group, mode string, payloadIDs []uint32, disconnectIDs []uint32) *mkwRaceEndRequest {
	request := &mkwRaceEndRequest{
		status: MKWRaceEndStatus{
			Requested:  time.Now().UTC(),
			Mode:       mode,
			Payload:    len(payloadIDs),
			Disconnect: len(disconnectIDs),
			Results:    map[string]string{},
		},
		pending: map[uint32]bool{},
	}
	for _, profileID := range payloadIDs {
		request.pending[profileID] = true
	}
	group.mkwRaceEnd = request
	return request
}

func scheduleMKWRaceEndFallback(group *Group, request *mkwRaceEndRequest) {
	time.AfterFunc(mkwRaceEndFallbackDelay, func() {
		runMKWRaceEndFallback(group, request)
	})
}

// runMKWRaceEndFallback closes the links of payload clients that did not
// confirm the race end in time.
func runMKWRaceEndFallback(group *Group, request *mkwRaceEndRequest) {
	mutex.Lock()
	if groups[group.GroupName] != group || group.mkwRaceEnd != request || len(request.pending) == 0 {
		mutex.Unlock()
		return
	}

	sender, targets := mkwRaceEndCommandParticipantsLocked(group)
	receivers := []mkwRaceEndCommandTarget{}
	for _, target := range targets {
		if request.pending[target.profileID] {
			receivers = append(receivers, target)
		}
	}
	messages := []queuedServerMatchMessage{}
	if sender != nil {
		messages = buildCloseLinksMessagesLocked(sender, targets, receivers)
	}
	request.status.Fallback = true
	request.status.Disconnect += len(messages)
	request.pending = map[uint32]bool{}
	roomID := group.GroupName
	mutex.Unlock()

	if len(messages) == 0 {
		return
	}
	logging.Notice("QR2", "Race end not confirmed in room", aurora.Cyan(roomID), "- closing peer links of", aurora.Cyan(len(messages)), "players")
	logging.Event("matchmaking_room_game_end_fallback", map[string]any{
		"room_id":      roomID,
		"disconnected": len(messages),
	})
	sendQueuedServerMatchMessages(messages)
}

// ProcessMKWRaceEndReport records the result a payload client reports for a
// race-end request (wl:mkw_end_race).
func ProcessMKWRaceEndReport(profileID uint32, result string) {
	mutex.Lock()
	login := logins[profileID]
	if login == nil || login.session == nil || login.session.groupPointer == nil {
		mutex.Unlock()
		return
	}
	group := login.session.groupPointer
	request := group.mkwRaceEnd
	if request == nil {
		mutex.Unlock()
		return
	}

	request.status.Results[strconv.FormatUint(uint64(profileID), 10)] = result
	if result == "finished" && request.pending[profileID] {
		delete(request.pending, profileID)
		request.status.Finished++
	}
	roomID := group.GroupName
	mutex.Unlock()

	logging.Event("matchmaking_room_game_end_result", map[string]any{
		"room_id":    roomID,
		"profile_id": profileID,
		"result":     result,
	})
}

// mkwRaceEndStatusLocked returns a copy of the room's recent race-end status.
func mkwRaceEndStatusLocked(group *Group) *MKWRaceEndStatus {
	request := group.mkwRaceEnd
	if request == nil || time.Since(request.status.Requested) > mkwRaceEndStatusTTL {
		return nil
	}
	status := request.status
	status.Results = make(map[string]string, len(request.status.Results))
	for k, v := range request.status.Results {
		status.Results[k] = v
	}
	return &status
}

// SetPayloadFeatures marks a logged-in profile as running the payload.
func SetPayloadFeatures(profileID uint32) {
	mutex.Lock()
	defer mutex.Unlock()

	if login, exists := logins[profileID]; exists {
		login.PayloadFeatures = true
	}
}
