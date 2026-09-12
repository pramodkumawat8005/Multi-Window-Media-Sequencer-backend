package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// GET /windows — every window with its playlist. The frontend computes each
// window's current playback position itself from created_at_unix + cycle_seconds,
// so the backend never needs to "push" the next item.
func listWindowsHandler(w http.ResponseWriter, r *http.Request) {
	windows, err := getAllWindows()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, windows)
}

// GET /windows/{id}
func getWindowHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid window id"})
		return
	}
	window, err := getWindowByID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "window not found"})
		return
	}
	writeJSON(w, http.StatusOK, window)
}

type addItemRequest struct {
	Type            string `json:"type"`
	URL             string `json:"url"`
	DurationSeconds int    `json:"duration_seconds"`
}

// POST /windows/{id}/items — dynamically append media to a window's list.
func addItemHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid window id"})
		return
	}

	var req addItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if req.Type != "image" && req.Type != "video" && req.Type != "blank" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type must be image, video, or blank"})
		return
	}
	if req.DurationSeconds <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "duration_seconds must be positive"})
		return
	}

	item, err := addMediaItem(id, req.Type, req.URL, req.DurationSeconds)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// PUT /items/{id} — edit an existing media item's type, url (pasted link OR
// an /uploads/... path from uploadHandler — both are just strings to this
// endpoint), and duration.
func updateItemHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item id"})
		return
	}

	var req addItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if req.Type != "image" && req.Type != "video" && req.Type != "blank" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type must be image, video, or blank"})
		return
	}
	if req.DurationSeconds <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "duration_seconds must be positive"})
		return
	}

	item, err := updateMediaItem(id, req.Type, req.URL, req.DurationSeconds)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// DELETE /items/{id} — remove a media item from whichever window's playlist it's in.
func deleteItemHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item id"})
		return
	}
	if err := deleteMediaItem(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type syncRequest struct {
	ItemID          int64 `json:"item_id"`
	DurationSeconds int   `json:"duration_seconds"`
}

// POST /sync — trigger a selected media item to display across every window
// simultaneously for the given duration. This does not modify any window's
// stored playlist or its underlying cycle clock; it only overrides what is
// rendered for the sync duration. See sse.go: startSync.
func syncHandler(w http.ResponseWriter, r *http.Request) {
	var req syncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if req.DurationSeconds <= 0 {
		req.DurationSeconds = 10 // sensible default if the caller omits it
	}

	item, err := getMediaItemByID(req.ItemID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "media item not found"})
		return
	}

	syncHub.startSync(*item, req.DurationSeconds)
	writeJSON(w, http.StatusOK, syncHub.current)
}

// GET /sync/status — current sync state, for a client that wants to poll
// instead of holding an SSE connection open (or to check state on first load).
func syncStatusHandler(w http.ResponseWriter, r *http.Request) {
	syncHub.mu.Lock()
	state := syncHub.current
	syncHub.mu.Unlock()
	writeJSON(w, http.StatusOK, state)
}
