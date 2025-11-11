# HYS Backend (Go)

Golang 1.22 ile yazılmış bu küçük servis, IK XML kaynağından personel verisini çekip REST API olarak sunar. Fixture dosyası desteği, bellek içi cache, allowlist tabanlı admin rotaları ve mobil uygulamalar için rol tayini içerir.

## Mimari

```
cmd/server/main.go        # Uygulama girişi
internal/http/routes.go   # Router + middleware
internal/handlers         # HTTP handler'ları
internal/repo             # XML kaynağı + cache
internal/model            # Veri modelleri
internal/allow            # Allowlist ve rol belirleme
```

## Çalıştırma

Önce modülleri çekin ve binayı ayağa kaldırın:

```bash
PERSONNEL_XML_URL="http://ik.hysavm.com.tr:8088/PersonelGuncellemeListesi.doms?MUSTERI_KODU=HYS&PAROLA=mxOTDjCAQvjMbdV" \
API_PORT=8080 \
go run ./cmd/server
```

Fixture ile offline test:

```bash
PERSONNEL_FIXTURE="/absolute/path/personeller.xml" \
PERSONNEL_XML_URL="http://placeholder" \
go run ./cmd/server
```

## Örnek İstekler

Personel listesi:

```bash
curl "http://localhost:8080/api/personel_detay?all=1&limit=100"
```

Giriş kontrolü:

```bash
curl -X POST http://localhost:8080/api/giris \
  -H "Content-Type: application/json" \
  -d '{"tc":"25031519376"}'
```

Allowlist yönetimi:

```bash
curl -X POST http://localhost:8080/api/admin/allowlist \
  -H "Content-Type: application/json" -H "X-Role: admin" \
  -d '{"tc":"25031519376"}'
```

## Testler

Parser ve rol karar testleri:

```bash
go test ./...
```

## Konfigürasyon

- `PERSONNEL_XML_URL` (zorunlu): IK XML endpoint'i.
- `PERSONNEL_FIXTURE`: var ise önce bu dosya okunur.
- `API_PORT` (varsayılan 8080)
- `ADMIN_ALLOWLIST`: virgülle ayrılmış TC listesi.
# hys-mobile
