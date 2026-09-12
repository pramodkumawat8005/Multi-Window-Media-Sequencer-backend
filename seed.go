package main

import (
	"log"
	"time"
)

// seedIfEmpty populates three example windows with a short mixed playlist each,
// only if the database is empty (so re-running the server doesn't duplicate data).
func seedIfEmpty(cycleSeconds int) {
	windows, err := getAllWindows()
	if err != nil {
		log.Fatalf("seed check failed: %v", err)
	}
	if len(windows) > 0 {
		return
	}

	now := time.Now().Unix()

	seedData := []struct {
		name  string
		items []MediaItem
	}{
		{
			name: "Window 1",
			items: []MediaItem{
				{Type: "image", URL: "https://picsum.photos/id/1015/800/450", DurationSeconds: 8},
				{Type: "video", URL: "https://www.w3schools.com/html/mov_bbb.mp4", DurationSeconds: 12},
				{Type: "image", URL: "https://picsum.photos/id/1025/800/450", DurationSeconds: 8},
			},
		},
		{
			name: "Window 2",
			items: []MediaItem{
				{Type: "image", URL: "https://picsum.photos/id/1035/800/450", DurationSeconds: 10},
				{Type: "blank", DurationSeconds: 5},
				{Type: "video", URL: "https://www.w3schools.com/html/movie.mp4", DurationSeconds: 12},
			},
		},
		{
			name: "Window 3",
			items: []MediaItem{
				{Type: "image", URL: "https://picsum.photos/id/1043/800/450", DurationSeconds: 8},
				{Type: "image", URL: "https://picsum.photos/id/1050/800/450", DurationSeconds: 8},
			},
		},
	}

	for _, sw := range seedData {
		windowID, err := insertWindow(sw.name, now, cycleSeconds)
		if err != nil {
			log.Fatalf("seed insert window failed: %v", err)
		}
		for _, it := range sw.items {
			if _, err := addMediaItem(windowID, it.Type, it.URL, it.DurationSeconds); err != nil {
				log.Fatalf("seed insert item failed: %v", err)
			}
		}
	}
	log.Println("seeded 3 windows with example playlists")
}
