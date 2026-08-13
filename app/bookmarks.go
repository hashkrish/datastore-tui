package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/krishnan/datastore-tui/datastore/model"
)

// bookmark is one saved entity key plus the display label shown in the
// bookmark list (ctrl+l).
type bookmark struct {
	Label string     `json:"label"`
	Key   *model.Key `json:"key"`
}

// bookmarksFilePath returns where bookmarks are persisted across sessions.
func bookmarksFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "datastore-tui", "bookmarks.json"), nil
}

// loadBookmarks reads the persisted bookmark list, returning nil (not an
// error) if none has been saved yet.
func loadBookmarks() []bookmark {
	path, err := bookmarksFilePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var bs []bookmark
	if err := json.Unmarshal(data, &bs); err != nil {
		return nil
	}
	return bs
}

// saveBookmarks persists bs, overwriting any previously saved list.
func saveBookmarks(bs []bookmark) error {
	path, err := bookmarksFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(bs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// bookmarkIndex returns the index of the bookmark for key in bs, or -1 if
// key isn't bookmarked.
func bookmarkIndex(bs []bookmark, key *model.Key) int {
	if key == nil {
		return -1
	}
	for i, b := range bs {
		if b.Key != nil && b.Key.String() == key.String() {
			return i
		}
	}
	return -1
}
