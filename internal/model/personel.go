package model

// Personel represents normalized personnel fields returned by the upstream feed.
type Personel struct {
	InsanID string `json:"insan_id"`
	TC      string `json:"tc"`
	Ad      string `json:"ad"`
	Soyad   string `json:"soyad"`
	Gorev   string `json:"gorev"`
	Unvan   string `json:"unvan"`
	Sube    string `json:"sube"`
	Telefon string `json:"telefon"`
}
