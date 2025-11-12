package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hys-backend-go/internal/allow"
	"hys-backend-go/internal/model"
	"hys-backend-go/internal/repo"
)

func TestPersonelListFromFixture(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "personel.xml")
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<PERSONELLER>
    <PERSONEL>
        <INSAN_ID>1</INSAN_ID>
        <TC>25031519370</TC>
        <AD>Ayse</AD>
        <SOYAD>Yilmaz</SOYAD>
        <GOREV>Mudur Yardimcisi</GOREV>
        <UNVAN>Mudur</UNVAN>
        <SUBE>03</SUBE>
        <TELEFON>5550001111</TELEFON>
    </PERSONEL>
    <PERSONEL>
        <INSAN_ID>2</INSAN_ID>
        <TC>25031519371</TC>
        <AD>Fatma</AD>
        <SOYAD>Kaya</SOYAD>
        <GOREV>Personel</GOREV>
        <UNVAN>Danisman</UNVAN>
        <SUBE>04</SUBE>
        <TELEFON>5550002222</TELEFON>
    </PERSONEL>
</PERSONELLER>`
	if err := os.WriteFile(fixture, []byte(xml), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	source := repo.NewSource("", fixture, 5*time.Minute, &http.Client{Timeout: time.Second})
	h := &Handler{source: source, allowlist: allow.NewAllowlist(""), loc: time.Local}

	req := httptest.NewRequest(http.MethodGet, "/api/personel_detay?all=1", nil)
	rr := httptest.NewRecorder()

	h.PersonelList(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp struct {
		Items []model.Personel `json:"items"`
		Total int              `json:"total"`
		Page  int              `json:"page"`
		Limit int              `json:"limit"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Total != 2 || len(resp.Items) != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Items[0].TC != "25031519370" {
		t.Fatalf("unexpected first TC: %+v", resp.Items[0])
	}
}
