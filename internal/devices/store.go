package devices

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store keeps device tokens thread-safely and persists them to disk.
type Store struct {
	mu      sync.RWMutex
	path    string
	records map[string]record
}

type record struct {
	Token string `json:"token"`
}

// NewStore loads device data from path or creates a new store if the file does not exist.
func NewStore(path string) (*Store, error) {
	if path == "" {
		path = "./var/devices.json"
	}
	s := &Store{
		path:    path,
		records: make(map[string]record),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Upsert stores or updates the token value for the given tc.
func (s *Store) Upsert(tc, token string) error {
	if tc == "" {
		return errors.New("tc is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[tc] = record{Token: token}
	return s.persistLocked()
}

// GetToken returns the token for tc.
func (s *Store) GetToken(tc string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.records[tc]
	if !ok {
		return "", false
	}
	return rec.Token, true
}

// All returns a copy of all stored tokens.
func (s *Store) All() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make(map[string]string, len(s.records))
	for tc, rec := range s.records {
		items[tc] = rec.Token
	}
	return items
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read devices store: %w", err)
	}
	var payload struct {
		Devices map[string]record `json:"devices"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parse devices store: %w", err)
	}
	if payload.Devices == nil {
		payload.Devices = make(map[string]record)
	}
	s.records = payload.Devices
	return nil
}

func (s *Store) persistLocked() error {
	dir := filepath.Dir(s.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create devices directory: %w", err)
		}
	}
	tmp, err := os.CreateTemp(dirOrDefault(dir), "devices-*.json")
	if err != nil {
		return fmt.Errorf("create temp devices file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{"devices": s.records}); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode devices store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp devices file: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("replace devices store: %w", err)
	}
	return nil
}

func dirOrDefault(dir string) string {
	if dir == "" || dir == "." {
		return "."
	}
	return dir
}
