package repo

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"hys-backend-go/internal/model"
)

// Source knows how to retrieve personnel data from either a fixture file or the upstream endpoint.
type Source struct {
	url     string
	fixture string
	ttl     time.Duration
	client  *http.Client

	cacheMu      sync.RWMutex
	cached       []model.Personel
	cachedSource string
	expiresAt    time.Time
}

// NewSource builds a Source instance.
func NewSource(url, fixture string, ttl time.Duration, client *http.Client) *Source {
	return &Source{url: strings.TrimSpace(url), fixture: strings.TrimSpace(fixture), ttl: ttl, client: client}
}

// Fetch returns the cached personnel slice or refreshes it if stale.
func (s *Source) Fetch(ctx context.Context) ([]model.Personel, string, error) {
	now := time.Now()
	s.cacheMu.RLock()
	if len(s.cached) > 0 && now.Before(s.expiresAt) {
		data := clonePersons(s.cached)
		source := s.cachedSource
		s.cacheMu.RUnlock()
		log.Printf("repo: cache hit (%s, items=%d)", source, len(data))
		return data, source, nil
	}
	s.cacheMu.RUnlock()
	log.Printf("repo: cache miss (expired=%v)", now.After(s.expiresAt))

	raw, source, err := s.loadData(ctx)
	if err != nil {
		return nil, "", err
	}

	persons, err := parsePersonelPayload(raw)
	if err != nil {
		return nil, "", err
	}

	s.cacheMu.Lock()
	s.cached = clonePersons(persons)
	s.cachedSource = source
	s.expiresAt = time.Now().Add(s.ttl)
	s.cacheMu.Unlock()

	return persons, source, nil
}

func (s *Source) loadData(ctx context.Context) ([]byte, string, error) {
	if s.fixture != "" {
		if raw, err := os.ReadFile(s.fixture); err == nil {
			log.Printf("repo: loaded fixture %s (len=%d)", s.fixture, len(raw))
			return raw, "fixture", nil
		} else {
			log.Printf("repo: fixture read error: %v", err)
		}
	}

	if s.url == "" {
		return nil, "", fmt.Errorf("PERSONNEL_XML_URL is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "hys-backend/1.0")
	start := time.Now()
	log.Printf("repo: fetching %s", s.url)
	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("repo: upstream error: %v", err)
		return nil, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("repo: upstream read error: %v", err)
		return nil, "", err
	}

	log.Printf("repo: upstream status=%d len=%d elapsed=%s", resp.StatusCode, len(body), time.Since(start))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("upstream status %d", resp.StatusCode)
	}

	return body, "upstream", nil
}

func parsePersonelPayload(raw []byte) ([]model.Personel, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("upstream payload is empty")
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return parsePersonelJSON(trimmed)
	}
	return parsePersonelXML(trimmed)
}

// parsePersonelXML decodes PERSONELLER/PERSONEL structures into normalized model.Personel slices.
func parsePersonelXML(raw []byte) ([]model.Personel, error) {
	type xmlPerson struct {
		InsanID string `xml:"INSAN_ID"`
		TC      string `xml:"TC"`
		Ad      string `xml:"AD"`
		Soyad   string `xml:"SOYAD"`
		Gorev   string `xml:"GOREV"`
		Unvan   string `xml:"UNVAN"`
		Sube    string `xml:"SUBE"`
		Telefon string `xml:"TELEFON"`
	}

	type wrap struct {
		Items []xmlPerson `xml:"PERSONELLER>PERSONEL"`
	}

	var w wrap
	if err := xml.Unmarshal(raw, &w); err != nil {
		return nil, err
	}
	if len(w.Items) == 0 {
		type alt struct {
			Items []xmlPerson `xml:"PERSONEL"`
		}
		var a alt
		if err := xml.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		w.Items = a.Items
	}
	if len(w.Items) == 0 {
		return nil, fmt.Errorf("xml does not contain PERSONEL nodes")
	}

	out := make([]model.Personel, 0, len(w.Items))
	for _, item := range w.Items {
		out = append(out, model.Personel{
			InsanID: normalize(item.InsanID),
			TC:      normalize(item.TC),
			Ad:      normalize(item.Ad),
			Soyad:   normalize(item.Soyad),
			Gorev:   normalize(item.Gorev),
			Unvan:   normalize(item.Unvan),
			Sube:    normalize(item.Sube),
			Telefon: normalize(item.Telefon),
		})
	}
	return out, nil
}

func parsePersonelJSON(raw []byte) ([]model.Personel, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var resp struct {
		Code  string           `json:"SONUC_KODU"`
		Items []map[string]any `json:"SONUC_MESAJI"`
	}
	if err := dec.Decode(&resp); err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("json does not contain SONUC_MESAJI entries")
	}

	out := make([]model.Personel, 0, len(resp.Items))
	for _, item := range resp.Items {
		out = append(out, model.Personel{
			InsanID: normalize(stringFromAny(item["INSAN_ID"])),
			TC:      normalize(stringFromAny(item["TC_KIMLIK_NO"])),
			Ad:      normalize(stringFromAny(item["ADI"])),
			Soyad:   normalize(stringFromAny(item["SOYADI"])),
			Gorev: normalize(firstNonEmpty(
				stringFromAny(item["UNVAN"]),
				stringFromAny(item["BOLUM"]),
			)),
			Unvan: normalize(stringFromAny(item["UNVAN"])),
			Sube: normalize(firstNonEmpty(
				stringFromAny(item["GOREV_YERI"]),
				stringFromAny(item["ISYERI"]),
				stringFromAny(item["MASRAF_YERI"]),
			)),
			Telefon: normalize(stringFromAny(item["TELEFON"])),
		})
	}
	return out, nil
}

func normalize(v string) string {
	return strings.TrimSpace(v)
}

func clonePersons(in []model.Personel) []model.Personel {
	dup := make([]model.Personel, len(in))
	copy(dup, in)
	return dup
}

func stringFromAny(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		return val
	case json.Number:
		return val.String()
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
