package allow

import "testing"

func TestDetermineRoute(t *testing.T) {
	allowlist := NewAllowlist("25031519376")

	tests := []struct {
		name   string
		tc     string
		gorev  string
		expect string
	}{
		{"allowlist-admin", "25031519376", "Personel", "admin"},
		{"patron", "1", "Genel Patron", "patron"},
		{"admin", "2", "Bilgi Islem Sorumlusu", "admin"},
		{"manager", "3", "Bölge Müdürü", "manager"},
		{"default", "4", "Kasiyer", "personel"},
	}

	for _, tt := range tests {
		if got := DetermineRoute(tt.tc, tt.gorev, allowlist); got != tt.expect {
			t.Fatalf("%s: expected %s got %s", tt.name, tt.expect, got)
		}
	}
}
