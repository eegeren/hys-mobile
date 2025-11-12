package checkin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Store persists last check-in timestamps to disk.
type Store struct {
	path string
	loc  *time.Location

	mu   sync.RWMutex
	data map[string]time.Time
}

// NewStore loads or creates a store backed by the provided file path.
func NewStore(path string, loc *time.Location) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("path is required")
	}
	if loc == nil {
		var err error
		loc, err = time.LoadLocation("Europe/Istanbul")
		if err != nil {
			return nil, err
		}
	}
	s := &Store{path: path, loc: loc, data: make(map[string]time.Time)}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Record stores the last check-in time for the provided TC synchronously on disk.
func (s *Store) Record(tc string, when time.Time) (time.Time, error) {
	tc = strings.TrimSpace(tc)
	if tc == "" {
		return time.Time{}, errors.New("tc is required")
	}
	normalized := when.In(s.loc)

	s.mu.Lock()
	s.data[tc] = normalized
	err := s.persistLocked()
	s.mu.Unlock()
	if err != nil {
		return time.Time{}, err
	}
	return normalized, nil
}

// Last returns the stored check-in timestamp for the TC.
func (s *Store) Last(tc string) (time.Time, bool) {
	tc = strings.TrimSpace(tc)
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.data[tc]
	return t, ok
}

// Flush forces the current state to be written to disk.
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.persistLocked()
}

func (s *Store) load() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var disk map[string]string
	if err := json.Unmarshal(raw, &disk); err != nil {
		return fmt.Errorf("decode %s: %w", s.path, err)
	}
	for tc, ts := range disk {
		parsed, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			return fmt.Errorf("parse checkin timestamp: %w", err)
		}
		s.data[tc] = parsed.In(s.loc)
	}
	return nil
}

func (s *Store) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	disk := make(map[string]string, len(s.data))
	for tc, ts := range s.data {
		disk[tc] = ts.Format(time.RFC3339)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "checkins-*.json")
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(disk); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}
