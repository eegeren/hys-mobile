package checkin

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkins.json")

	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	store, err := NewStore(path, loc)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	when := time.Date(2024, time.January, 2, 9, 30, 0, 0, loc)
	recorded, err := store.Record("25031519376", when)
	if err != nil {
		t.Fatalf("record: %v", err)
	}

	reloaded, err := NewStore(path, loc)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}

	got, ok := reloaded.Last("25031519376")
	if !ok {
		t.Fatalf("expected record to exist after reload")
	}
	if !got.Equal(recorded) {
		t.Fatalf("expected %s, got %s", recorded, got)
	}
}
