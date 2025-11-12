package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"hys-backend-go/internal/allow"
	"hys-backend-go/internal/checkin"
	"hys-backend-go/internal/devices"
	"hys-backend-go/internal/model"
	"hys-backend-go/internal/notify"
	"hys-backend-go/internal/reminder"
	"hys-backend-go/internal/repo"
	"hys-backend-go/internal/roles"
)

// Handler bundles HTTP endpoints.
type Handler struct {
	source    *repo.Source
	allowlist *allow.Allowlist
	checkins  *checkin.Store
	reminder  *reminder.Service
	devices   *devices.Store
	notifier  notify.Sender
	loc       *time.Location
}

// New constructs a Handler.
func New(source *repo.Source, allowlist *allow.Allowlist, checkins *checkin.Store, reminderSvc *reminder.Service, devicesStore *devices.Store, notifier notify.Sender, loc *time.Location) *Handler {
	if loc == nil {
		loc = time.Local
	}
	return &Handler{
		source:    source,
		allowlist: allowlist,
		checkins:  checkins,
		reminder:  reminderSvc,
		devices:   devicesStore,
		notifier:  notifier,
		loc:       loc,
	}
}

// Healthz responds with general status.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{"status": "ok", "ok": true})
}

// PersonelList handles GET /api/personel_detay.
func (h *Handler) PersonelList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	persons, _, err := h.source.Fetch(ctx)
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
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
		"ok":    true,
	})
}

// Login handles POST /api/giris.
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

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
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

	respondJSON(w, http.StatusOK, map[string]any{
		"personel": found,
		"role":     roles.Determine(found),
		"ok":       true,
	})
}

// Checkin handles POST /api/checkin.
func (h *Handler) Checkin(w http.ResponseWriter, r *http.Request) {
	if h.checkins == nil {
		respondError(w, http.StatusServiceUnavailable, "checkin store unavailable")
		return
	}

	var payload struct {
		TC         string `json:"tc"`
		GirisSaati string `json:"giris_saati"`
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

	target := time.Now().In(h.loc)
	if s := strings.TrimSpace(payload.GirisSaati); s != "" {
		parsed, err := time.ParseInLocation(time.RFC3339, s, h.loc)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid giris_saati format")
			return
		}
		target = parsed
	}

	recorded, err := h.checkins.Record(payload.TC, target)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"tc":            payload.TC,
		"recorded_at":   recorded.Format(time.RFC3339),
		"recorded_unix": recorded.Unix(),
		"ok":            true,
	})
}

// ShiftToday handles GET /api/shift-today.
func (h *Handler) ShiftToday(w http.ResponseWriter, r *http.Request) {
	if h.reminder == nil {
		respondError(w, http.StatusServiceUnavailable, "reminder service unavailable")
		return
	}
	sube := strings.TrimSpace(r.URL.Query().Get("sube"))
	if sube == "" {
		respondError(w, http.StatusBadRequest, "sube is required")
		return
	}
	labels, err := h.reminder.ShiftLabels(sube, time.Now().In(h.loc))
	if err != nil {
		if errors.Is(err, reminder.ErrNoSchedule) {
			respondError(w, http.StatusNotFound, "schedule not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"sube":   sube,
		"shifts": labels,
		"ok":     true,
	})
}

// AllowlistList handles GET /api/admin/allowlist.
func (h *Handler) AllowlistList(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if h.allowlist == nil {
		respondError(w, http.StatusServiceUnavailable, "allowlist unavailable")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": h.allowlist.List(), "ok": true})
}

// AllowlistAdd handles POST /api/admin/allowlist.
func (h *Handler) AllowlistAdd(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if h.allowlist == nil {
		respondError(w, http.StatusServiceUnavailable, "allowlist unavailable")
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

// AllowlistDelete handles DELETE /api/admin/allowlist/{tc}.
func (h *Handler) AllowlistDelete(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if h.allowlist == nil {
		respondError(w, http.StatusServiceUnavailable, "allowlist unavailable")
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

// ForceNotify handles POST /api/admin/force-notify.
func (h *Handler) ForceNotify(w http.ResponseWriter, r *http.Request) {
	if !hasRole(r, "personel", "admin") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if h.reminder == nil {
		respondError(w, http.StatusServiceUnavailable, "reminder service unavailable")
		return
	}
	tc := strings.TrimSpace(r.URL.Query().Get("tc"))
	if tc == "" {
		respondError(w, http.StatusBadRequest, "tc is required")
		return
	}
	note := strings.TrimSpace(r.URL.Query().Get("note"))
	if err := h.reminder.ForceNotify(tc, note); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]any{"error": msg, "ok": false})
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
	return hasRole(r, "admin")
}

func hasRole(r *http.Request, roles ...string) bool {
	role := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Role")))
	for _, allowed := range roles {
		if role == allowed {
			return true
		}
	}
	return false
}
