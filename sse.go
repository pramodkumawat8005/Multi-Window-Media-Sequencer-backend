package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// hub keeps track of connected SSE clients and the current sync state.
// A window's frontend only needs to know: is a sync active, and if so, what item and until when.
type hub struct {
	mu        sync.Mutex
	clients   map[chan []byte]bool
	current   SyncState
	endTimer  *time.Timer
}

var syncHub = &hub{
	clients: make(map[chan []byte]bool),
}

func (h *hub) subscribe() chan []byte {
	ch := make(chan []byte, 8)
	h.mu.Lock()
	h.clients[ch] = true
	h.mu.Unlock()
	return ch
}

func (h *hub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *hub) broadcast(state SyncState) {
	h.mu.Lock()
	h.current = state
	payload, _ := json.Marshal(state)
	for ch := range h.clients {
		select {
		case ch <- payload:
		default:
			// slow client, drop the message rather than block the broadcaster
		}
	}
	h.mu.Unlock()
}

// startSync begins a sync-to-all-windows event and schedules its automatic end.
// Each window's own playback clock is untouched by this — see cycle logic on the frontend.
func (h *hub) startSync(item MediaItem, durationSeconds int) {
	h.mu.Lock()
	if h.endTimer != nil {
		h.endTimer.Stop()
	}
	h.mu.Unlock()

	now := time.Now().Unix()
	state := SyncState{
		Active:        true,
		ItemID:        item.ID,
		Type:          item.Type,
		URL:           item.URL,
		StartedAtUnix: now,
		EndsAtUnix:    now + int64(durationSeconds),
	}
	h.broadcast(state)

	timer := time.AfterFunc(time.Duration(durationSeconds)*time.Second, func() {
		h.broadcast(SyncState{Active: false})
	})
	h.mu.Lock()
	h.endTimer = timer
	h.mu.Unlock()
}

// sseHandler streams sync state changes to a connected browser.
// On connect, it immediately sends the current state so late joiners don't miss an in-progress sync.
func sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := syncHub.subscribe()
	defer syncHub.unsubscribe(ch)

	syncHub.mu.Lock()
	initial, _ := json.Marshal(syncHub.current)
	syncHub.mu.Unlock()
	fmt.Fprintf(w, "data: %s\n\n", initial)
	flusher.Flush()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
