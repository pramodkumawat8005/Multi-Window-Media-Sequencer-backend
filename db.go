package main

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

var db *sql.DB

// initDB opens a Postgres connection using the given DSN (DATABASE_URL) and
// ensures the schema exists. Unlike SQLite, Postgres needs an actual running
// server and an existing database (the DB named in DATABASE_URL, e.g.
// "eva_bharat", must already exist on the Postgres server — see README
// "Postgres setup"; this code creates the TABLES, not the database itself).
func initDB(dsn string) {
	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to connect to postgres (check DATABASE_URL and that the server/db exist): %v", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS windows (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		created_at_unix BIGINT NOT NULL,
		cycle_seconds INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS media_items (
		id SERIAL PRIMARY KEY,
		window_id INTEGER NOT NULL REFERENCES windows(id),
		type TEXT NOT NULL,
		url TEXT,
		duration_seconds INTEGER NOT NULL,
		order_index INTEGER NOT NULL
	);
	`
	if _, err := db.Exec(schema); err != nil {
		log.Fatalf("failed to create schema: %v", err)
	}
}

func getAllWindows() ([]Window, error) {
	rows, err := db.Query(`SELECT id, name, created_at_unix, cycle_seconds FROM windows ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var windows []Window
	for rows.Next() {
		var w Window
		if err := rows.Scan(&w.ID, &w.Name, &w.CreatedAtUTC, &w.CycleSeconds); err != nil {
			return nil, err
		}
		items, err := getItemsForWindow(w.ID)
		if err != nil {
			return nil, err
		}
		w.Items = items
		windows = append(windows, w)
	}
	return windows, nil
}

func getWindowByID(id int64) (*Window, error) {
	var w Window
	row := db.QueryRow(`SELECT id, name, created_at_unix, cycle_seconds FROM windows WHERE id = $1`, id)
	if err := row.Scan(&w.ID, &w.Name, &w.CreatedAtUTC, &w.CycleSeconds); err != nil {
		return nil, err
	}
	items, err := getItemsForWindow(w.ID)
	if err != nil {
		return nil, err
	}
	w.Items = items
	return &w, nil
}

func getItemsForWindow(windowID int64) ([]MediaItem, error) {
	rows, err := db.Query(
		`SELECT id, window_id, type, url, duration_seconds, order_index
		 FROM media_items WHERE window_id = $1 ORDER BY order_index`, windowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []MediaItem
	for rows.Next() {
		var it MediaItem
		var url sql.NullString
		if err := rows.Scan(&it.ID, &it.WindowID, &it.Type, &url, &it.DurationSeconds, &it.OrderIndex); err != nil {
			return nil, err
		}
		it.URL = url.String
		items = append(items, it)
	}
	return items, nil
}

// insertWindow creates a window and returns its new ID.
// Postgres has no LastInsertId support via database/sql, so we use
// "RETURNING id" and QueryRow instead of Exec.
func insertWindow(name string, createdAtUnix int64, cycleSeconds int) (int64, error) {
	var id int64
	err := db.QueryRow(
		`INSERT INTO windows (name, created_at_unix, cycle_seconds) VALUES ($1, $2, $3) RETURNING id`,
		name, createdAtUnix, cycleSeconds,
	).Scan(&id)
	return id, err
}

// addMediaItem appends a new item to the end of a window's playlist.
func addMediaItem(windowID int64, mediaType, url string, durationSeconds int) (MediaItem, error) {
	var maxOrder sql.NullInt64
	row := db.QueryRow(`SELECT MAX(order_index) FROM media_items WHERE window_id = $1`, windowID)
	if err := row.Scan(&maxOrder); err != nil {
		return MediaItem{}, err
	}
	nextOrder := 0
	if maxOrder.Valid {
		nextOrder = int(maxOrder.Int64) + 1
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO media_items (window_id, type, url, duration_seconds, order_index)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		windowID, mediaType, url, durationSeconds, nextOrder,
	).Scan(&id)
	if err != nil {
		return MediaItem{}, err
	}
	return MediaItem{
		ID: id, WindowID: windowID, Type: mediaType, URL: url,
		DurationSeconds: durationSeconds, OrderIndex: nextOrder,
	}, nil
}

// updateMediaItem overwrites an existing item's type, url, and duration.
// order_index and window_id are left untouched — moving an item between
// windows isn't a requirement, so this only edits the item's own content.
func updateMediaItem(itemID int64, mediaType, url string, durationSeconds int) (*MediaItem, error) {
	res, err := db.Exec(
		`UPDATE media_items SET type = $1, url = $2, duration_seconds = $3 WHERE id = $4`,
		mediaType, url, durationSeconds, itemID,
	)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, sql.ErrNoRows
	}
	return getMediaItemByID(itemID)
}

// deleteMediaItem removes an item. Order indices of remaining items are left
// as-is (gaps in order_index are fine — playback order only depends on
// relative ordering, not contiguous numbering).
func deleteMediaItem(itemID int64) error {
	res, err := db.Exec(`DELETE FROM media_items WHERE id = $1`, itemID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// getMediaItemByID is used to resolve the item a sync request points at,
// and to return the fresh row after an update.
func getMediaItemByID(id int64) (*MediaItem, error) {
	var it MediaItem
	var url sql.NullString
	row := db.QueryRow(
		`SELECT id, window_id, type, url, duration_seconds, order_index FROM media_items WHERE id = $1`, id)
	if err := row.Scan(&it.ID, &it.WindowID, &it.Type, &url, &it.DurationSeconds, &it.OrderIndex); err != nil {
		return nil, err
	}
	it.URL = url.String
	return &it, nil
}
