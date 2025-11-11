package allow

import (
	"sort"
	"strings"
	"sync"
)

// Allowlist keeps an in-memory set of TCs granted admin access.
type Allowlist struct {
	mu     sync.RWMutex
	values map[string]struct{}
}

// NewAllowlist seeds the store with comma-separated TC values.
func NewAllowlist(csv string) *Allowlist {
	a := &Allowlist{values: make(map[string]struct{})}
	a.seed(csv)
	return a
}

func (a *Allowlist) seed(csv string) {
	for _, part := range strings.Split(csv, ",") {
		tc := strings.TrimSpace(part)
		if tc != "" {
			a.values[tc] = struct{}{}
		}
	}
}

// Add inserts a TC into the allowlist.
func (a *Allowlist) Add(tc string) {
	tc = strings.TrimSpace(tc)
	if tc == "" {
		return
	}
	a.mu.Lock()
	a.values[tc] = struct{}{}
	a.mu.Unlock()
}

// Remove deletes a TC from the set.
func (a *Allowlist) Remove(tc string) {
	tc = strings.TrimSpace(tc)
	if tc == "" {
		return
	}
	a.mu.Lock()
	delete(a.values, tc)
	a.mu.Unlock()
}

// Has returns true if the TC is on the allowlist.
func (a *Allowlist) Has(tc string) bool {
	tc = strings.TrimSpace(tc)
	a.mu.RLock()
	_, ok := a.values[tc]
	a.mu.RUnlock()
	return ok
}

// List returns a sorted slice of allowlisted TCs.
func (a *Allowlist) List() []string {
	a.mu.RLock()
	out := make([]string, 0, len(a.values))
	for tc := range a.values {
		out = append(out, tc)
	}
	a.mu.RUnlock()
	sort.Strings(out)
	return out
}

var replacer = strings.NewReplacer(
	"ı", "i",
	"İ", "i",
	"ğ", "g",
	"Ğ", "g",
	"ş", "s",
	"Ş", "s",
	"ö", "o",
	"Ö", "o",
	"ü", "u",
	"Ü", "u",
)

// DetermineRoute picks the mobile route for a personnel record.
func DetermineRoute(tc, gorev string, allowlist *Allowlist) string {
	if allowlist != nil && allowlist.Has(tc) {
		return "admin"
	}

	normalized := replacer.Replace(strings.ToLower(strings.TrimSpace(gorev)))

	switch {
	case strings.Contains(normalized, "patron"):
		return "patron"
	case strings.Contains(normalized, "bilgi islem"):
		return "admin"
	case strings.Contains(normalized, "mudur"):
		return "manager"
	default:
		return "personel"
	}
}
