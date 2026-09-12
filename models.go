package main

// MediaItem is a single entry in a window's playlist.
// Type is one of "image", "video", or "blank".
type MediaItem struct {
	ID              int64  `json:"id"`
	WindowID        int64  `json:"window_id"`
	Type            string `json:"type"`
	URL             string `json:"url,omitempty"`
	DurationSeconds int    `json:"duration_seconds"`
	OrderIndex      int    `json:"order_index"`
}

// Window is one display window with its own looping playlist.
type Window struct {
	ID           int64       `json:"id"`
	Name         string      `json:"name"`
	CreatedAtUTC int64       `json:"created_at_unix"` // epoch seconds, used as the cycle's anchor point
	CycleSeconds int         `json:"cycle_seconds"`   // the fixed 5-hour (or overridden) cycle length
	Items        []MediaItem `json:"items"`
}

// SyncState describes an in-progress (or absent) sync-to-all-windows event.
type SyncState struct {
	Active          bool   `json:"active"`
	ItemID          int64  `json:"item_id,omitempty"`
	Type            string `json:"type,omitempty"`
	URL             string `json:"url,omitempty"`
	StartedAtUnix   int64  `json:"started_at_unix,omitempty"`
	EndsAtUnix      int64  `json:"ends_at_unix,omitempty"`
}
