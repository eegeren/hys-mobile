package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/hys/backend/internal/allow"
	"github.com/hys/backend/internal/model"
	"github.com/hys/backend/internal/repo"
)

// Handler bundles all HTTP endpoints.
type Handler struct {
	source    *repo.Source
	allowlist *allow.Allowlist
}

// New constructs a Handler.
func New(source *repo.Source, allowlist *allow.Allowlist) *Handler {
	return &Handler{source: source, allowlist: allowlist}
}

// Healthz responds with health information.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// PersonelList handles GET /api/personel_detay
func (h *Handler) PersonelList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	persons, src, err := h.source.Fetch(ctx)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	all := r.URL.Query().Get("all") == "1"
	page := clampInt(parseInt(r.URL.Query().Get("page"), 1), 1, 100000)
	limit := clampInt(parseInt(r.URL.Query().Get("limit"), 50), 1, 500)

	total := len(persons)
	items := persons

	if !all {
		start := (page - 1) * limit
		if start > total {
			start = total
		}
		end := start + limit
		if end > total {
			end = total
		}
		items = persons[start:end]
	} else {
		page = 1
		limit = total
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"source": src,
		"total":  total,
		"page":   page,
		"limit":  limit,
		"items":  items,
	})
}

// Login handles POST /api/giris
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		TC string `json:"tc"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	payload.TC = strings.TrimSpace(payload.TC)
	if payload.TC == "" {
		respondError(w, http.StatusBadRequest, "tc is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	persons, _, err := h.source.Fetch(ctx)
	if err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}

	var found *model.Personel
	for i := range persons {
		if strings.EqualFold(persons[i].TC, payload.TC) {
			found = &persons[i]
			break
		}
	}
	if found == nil {
		respondError(w, http.StatusNotFound, "personnel not found")
		return
	}

	route := allow.DetermineRoute(found.TC, found.Gorev, h.allowlist)
	respondJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"route":    route,
		"personel": found,
	})
}

// AllowlistList handles GET /api/admin/allowlist
func (h *Handler) AllowlistList(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"items": h.allowlist.List(),
	})
}

// AllowlistAdd handles POST /api/admin/allowlist
func (h *Handler) AllowlistAdd(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		TC string `json:"tc"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	payload.TC = strings.TrimSpace(payload.TC)
	if payload.TC == "" {
		respondError(w, http.StatusBadRequest, "tc is required")
		return
	}
	h.allowlist.Add(payload.TC)
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// AllowlistDelete handles DELETE /api/admin/allowlist/{tc}
func (h *Handler) AllowlistDelete(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	tc := strings.TrimSpace(mux.Vars(r)["tc"])
	if tc == "" {
		respondError(w, http.StatusBadRequest, "tc is required")
		return
	}
	h.allowlist.Remove(tc)
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]any{"ok": false, "error": msg})
}

func parseInt(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	if v, err := strconv.Atoi(raw); err == nil {
		return v
	}
	return fallback
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func isAdminRequest(r *http.Request) bool {
	role := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Role")))
	return role == "patron" || role == "admin"
}
