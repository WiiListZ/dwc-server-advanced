package api

import (
	"dwc-server-advanced/logging"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

var (
	panelEventsOnce sync.Once
	panelEventsHub  = newPanelEventHub()
)

type panelEventHub struct {
	mutex   sync.Mutex
	clients map[chan []byte]struct{}
}

type panelEventUpdate struct {
	Areas []string `json:"areas"`
	At    string   `json:"at"`
}

func newPanelEventHub() *panelEventHub {
	return &panelEventHub{
		clients: map[chan []byte]struct{}{},
	}
}

func startPanelEventStream() {
	panelEventsOnce.Do(func() {
		logging.RegisterEventCallback([]string{"all"}, func(eventType string, eventData map[string]any) {
			areas := panelUpdateAreas(eventType)
			if len(areas) == 0 {
				return
			}

			// Database event callbacks run asynchronously too; give them a short
			// head start so panel refreshes usually see the just-written row.
			time.Sleep(500 * time.Millisecond)
			panelEventsHub.broadcast(panelEventUpdate{
				Areas: areas,
				At:    time.Now().UTC().Format(time.RFC3339Nano),
			})
		})
	})
}

func (h *panelEventHub) subscribe() chan []byte {
	ch := make(chan []byte, 16)
	h.mutex.Lock()
	h.clients[ch] = struct{}{}
	h.mutex.Unlock()
	return ch
}

func (h *panelEventHub) unsubscribe(ch chan []byte) {
	h.mutex.Lock()
	delete(h.clients, ch)
	close(ch)
	h.mutex.Unlock()
}

func (h *panelEventHub) broadcast(update panelEventUpdate) {
	data, err := json.Marshal(update)
	if err != nil {
		return
	}

	h.mutex.Lock()
	defer h.mutex.Unlock()
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- data:
			default:
			}
		}
	}
}

func panelUpdateAreas(eventType string) []string {
	areas := map[string]struct{}{}
	add := func(names ...string) {
		for _, name := range names {
			areas[name] = struct{}{}
		}
	}

	if panelEventAffectsLogs(eventType) {
		add("logs")
	}

	switch eventType {
	case "profile_created", "logged_in", "logged_out", "received_login_info", "device_authenticated",
		"profile_banned", "profile_unbanned", "profile_kicked":
		add("dashboard")
	case "group_created", "group_deleted", "group_joined", "group_left", "group_host_changed",
		"natneg_succeeded", "natneg_failed", "matchmaking_room_created", "matchmaking_room_deleted",
		"matchmaking_room_game_end_attempted", "matchmaking_room_game_end_result", "matchmaking_room_game_end_fallback", "profile_forced_room_join", "panel_match_log_recorded":
		add("dashboard", "player_rooms")
	case "room_player_kicked":
		add("dashboard", "player_rooms")
	case "room_player_reported":
		add("reports", "logs")
	case "profile_link_requested", "profile_linked", "profile_unlinked":
		add("accounts")
	case "panel_mkw_ranking_deleted", "panel_mkw_profile_cleared", "panel_mkw_ghost_downloaded",
		"mkw_ghost_downloaded", "mkw_ghost_uploaded", "mkw_invalid_ghost_upload":
		add("ghosts")
	}

	if len(areas) == 0 {
		return nil
	}

	result := make([]string, 0, len(areas))
	for area := range areas {
		result = append(result, area)
	}
	sort.Strings(result)
	return result
}

func panelEventAffectsLogs(eventType string) bool {
	switch eventType {
	case "profile_created",
		"logged_in",
		"logged_out",
		"received_login_info",
		"device_authenticated",
		"login_message_delivered",
		"reported_bad_packet",
		"reported_stall",
		"gpcm_returned_error",
		"group_created",
		"group_deleted",
		"group_joined",
		"group_left",
		"group_host_changed",
		"natneg_succeeded",
		"natneg_failed",
		"profile_kicked",
		"profile_banned",
		"profile_unbanned",
		"room_player_kicked",
		"room_player_reported",
		"login_message_created",
		"profile_forced_room_join",
		"matchmaking_room_created",
		"matchmaking_room_deleted",
		"matchmaking_room_game_end_attempted",
		"matchmaking_room_game_end_result",
		"matchmaking_room_game_end_fallback",
		"profile_link_requested",
		"profile_linked",
		"profile_unlinked",
		"panel_mkw_ranking_deleted",
		"panel_mkw_profile_cleared",
		"panel_mkw_ghost_downloaded",
		"mkw_ghost_downloaded",
		"mkw_ghost_uploaded",
		"mkw_invalid_ghost_upload",
		"panel_match_log_recorded":
		return true
	default:
		return false
	}
}

func HandlePanelEvents(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	if _, ok := requirePanelAccount(w, r); !ok {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writePanelError(w, http.StatusInternalServerError, "event streaming unavailable")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := panelEventsHub.subscribe()
	defer panelEventsHub.unsubscribe(ch)

	_, _ = fmt.Fprint(w, "retry: 5000\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case data := <-ch:
			_, _ = fmt.Fprintf(w, "event: panel-update\ndata: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
