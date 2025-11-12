package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RegisterDevice handles POST /api/devices/register.
func (h *Handler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	if h.devices == nil {
		respondError(w, http.StatusServiceUnavailable, "device store unavailable")
		return
	}
	var payload struct {
		TC       string `json:"tc"`
		Token    string `json:"token"`
		Platform string `json:"platform"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	tc := strings.TrimSpace(payload.TC)
	token := strings.TrimSpace(payload.Token)

	if !isValidTC(tc) {
		respondError(w, http.StatusBadRequest, "tc must be 11 digits")
		return
	}
	if token == "" {
		respondError(w, http.StatusBadRequest, "token is required")
		return
	}
	if err := h.devices.Upsert(tc, token); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save device token")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// AdminPushTest handles POST /api/admin/push-test.
func (h *Handler) AdminPushTest(w http.ResponseWriter, r *http.Request) {
	if !isAdminRequest(r) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if h.devices == nil || h.notifier == nil {
		respondError(w, http.StatusServiceUnavailable, "push service unavailable")
		return
	}
	var payload struct {
		TC    string `json:"tc"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	tc := strings.TrimSpace(payload.TC)
	title := strings.TrimSpace(payload.Title)
	body := strings.TrimSpace(payload.Body)

	if !isValidTC(tc) {
		respondError(w, http.StatusBadRequest, "tc must be 11 digits")
		return
	}
	if title == "" {
		respondError(w, http.StatusBadRequest, "title is required")
		return
	}
	if body == "" {
		respondError(w, http.StatusBadRequest, "body is required")
		return
	}
	token, ok := h.devices.GetToken(tc)
	if !ok {
		respondError(w, http.StatusNotFound, "device token not found")
		return
	}
	custom := map[string]any{
		"tc":        tc,
		"push_type": "admin-test",
	}
	if err := h.notifier.Send(token, title, body, custom); err != nil {
		respondError(w, http.StatusBadGateway, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func isValidTC(tc string) bool {
	if len(tc) != 11 {
		return false
	}
	for _, r := range tc {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
