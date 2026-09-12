package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var uploadDir = "uploads"

// uploadHandler accepts a multipart/form-data POST with a "file" field,
// saves it under uploadDir with a random-prefixed name (to avoid collisions
// and overwriting), and returns {"url": "/uploads/<name>"} — a URL you can
// then pass straight into POST /windows/{id}/items or PUT /items/{id} exactly
// like a pasted external URL. This is what makes "paste a URL OR upload a
// file" both work through the same `url` field everywhere else in the API.
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// 32 MB in-memory parse limit; larger files spill to a temp file automatically.
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "could not parse upload: " + err.Error()})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing 'file' field"})
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	safeName := fmt.Sprintf("%s%s", uuid.NewString(), strings.ToLower(ext))
	destPath := filepath.Join(uploadDir, safeName)

	out, err := os.Create(destPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save file"})
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not write file"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"url": "/uploads/" + safeName})
}
