package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// loadDotEnv reads a simple KEY=VALUE .env file and sets each variable via
// os.Setenv, but only if that variable isn't already set in the real
// environment — so production env vars (e.g. set on Render/Railway) always
// win over whatever is committed in .env for local dev.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no .env file — fine, rely on real env vars
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func main() {
	loadDotEnv(".env")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set (check your .env file or environment)")
	}

	if d := os.Getenv("UPLOAD_DIR"); d != "" {
		uploadDir = d
	}
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("could not create upload dir %q: %v", uploadDir, err)
	}

	// CYCLE_SECONDS defaults to a real 5-hour cycle (18000s) as the
	// assignment specifies. Override it (e.g. CYCLE_SECONDS=30) when demoing
	// locally so the loop-and-restart behavior is visible without waiting hours.
	cycleSeconds := getEnvInt("CYCLE_SECONDS", 5*3600)

	initDB(dsn)
	seedIfEmpty(cycleSeconds)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /windows", withCORS(listWindowsHandler))
	mux.HandleFunc("GET /windows/{id}", withCORS(getWindowHandler))
	mux.HandleFunc("POST /windows/{id}/items", withCORS(addItemHandler))
	mux.HandleFunc("PUT /items/{id}", withCORS(updateItemHandler))
	mux.HandleFunc("DELETE /items/{id}", withCORS(deleteItemHandler))
	mux.HandleFunc("POST /upload", withCORS(uploadHandler))
	mux.HandleFunc("POST /sync", withCORS(syncHandler))
	mux.HandleFunc("GET /sync/status", withCORS(syncStatusHandler))
	mux.HandleFunc("GET /events", withCORS(sseHandler))

	// Serve uploaded media files directly, e.g. GET /uploads/<uuid>.jpg
	fileServer := http.FileServer(http.Dir(uploadDir))
	mux.Handle("GET /uploads/", withCORS(http.StripPrefix("/uploads/", fileServer).ServeHTTP))

	// Explicit CORS preflight (OPTIONS) handlers — browsers send these before
	// any POST/PUT/DELETE with a JSON body or file, and Go's method-specific
	// routing above won't otherwise match an OPTIONS request to the same path.
	noop := func(w http.ResponseWriter, r *http.Request) {}
	mux.HandleFunc("OPTIONS /windows/{id}/items", withCORS(noop))
	mux.HandleFunc("OPTIONS /items/{id}", withCORS(noop))
	mux.HandleFunc("OPTIONS /upload", withCORS(noop))
	mux.HandleFunc("OPTIONS /sync", withCORS(noop))

	log.Printf("media sequencer backend listening on :%s (cycle=%ds, uploads=%s)", port, cycleSeconds, uploadDir)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
