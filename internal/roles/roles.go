package roles

import (
	"strings"

	"hys-backend-go/internal/model"
)

var (
	// PatronTCs contains TC numbers that should be treated as patrons.
	PatronTCs []string
	// IKTCs contains TC numbers that should be treated as HR.
	IKTCs []string
)

// Determine returns the logical role label for a personnel record.
func Determine(p *model.Personel) string {
	if p == nil {
		return "personel"
	}
	tc := strings.TrimSpace(p.TC)
	for _, candidate := range PatronTCs {
		if strings.TrimSpace(candidate) == tc && tc != "" {
			return "patron"
		}
	}
	for _, candidate := range IKTCs {
		if strings.TrimSpace(candidate) == tc && tc != "" {
			return "ik"
		}
	}

	gorev := strings.ToLower(strings.TrimSpace(p.Gorev))
	switch {
	case strings.Contains(gorev, "bilgi"):
		return "bilgi_islem"
	case strings.Contains(gorev, "mudur") || strings.Contains(gorev, "müdür"):
		return "mudur"
	default:
		return "personel"
	}
}
