package repo

import "testing"

func TestParsePersonelXML(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<PERSONELLER>
    <PERSONEL>
        <INSAN_ID> 123 </INSAN_ID>
        <TC>25031519376</TC>
        <AD>Ali</AD>
        <SOYAD>Veli</SOYAD>
        <GOREV>Bilgi Islem Uzmani</GOREV>
        <UNVAN>Uzman</UNVAN>
        <SUBE>Merkez</SUBE>
        <TELEFON>5550001111</TELEFON>
    </PERSONEL>
</PERSONELLER>`)

	persons, err := parsePersonelXML(xmlData)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(persons) != 1 {
		t.Fatalf("expected 1 person, got %d", len(persons))
	}
	got := persons[0]
	if got.TC != "25031519376" || got.Gorev != "Bilgi Islem Uzmani" {
		t.Fatalf("unexpected person parsed: %+v", got)
	}
}

func TestParsePersonelJSON(t *testing.T) {
	jsonData := []byte(`{
  "SONUC_KODU": "0",
  "SONUC_MESAJI": [
    {
      "INSAN_ID": 134236232,
      "TC_KIMLIK_NO": 25031519376,
      "ADI": "Ali",
      "SOYADI": "Veli",
      "UNVAN": "Bilgi Islem Uzmani",
      "BOLUM": "IT",
      "GOREV_YERI": "Merkez",
      "TELEFON": "5550001111"
    }
  ]
}`)

	persons, err := parsePersonelPayload(jsonData)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(persons) != 1 {
		t.Fatalf("expected 1 person, got %d", len(persons))
	}
	got := persons[0]
	if got.TC != "25031519376" {
		t.Fatalf("unexpected TC parsed: %+v", got)
	}
	if got.Gorev != "Bilgi Islem Uzmani" {
		t.Fatalf("unexpected Gorev parsed: %+v", got)
	}
	if got.Sube != "Merkez" {
		t.Fatalf("unexpected Sube parsed: %+v", got)
	}
}
