# HYS Backend (Go)

Go 1.21 ile yazılmış bu küçük servis, IK XML kaynağından personel verisini çekip REST API olarak sunar. 5 dakikalık bellek içi cache, kalıcı check-in deposu, vardiya hatırlatma döngüsü ve allowlist tabanlı admin rotaları içerir.

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

Önce modülleri çekin ve servisi konfigüre edip ayağa kaldırın:

```bash
PERSONNEL_XML_URL="http://ik.example.com/personel.xml" \
CHECKIN_DB_PATH="./data/checkins.json" \
ALLOWLIST_INIT="25031519370,25031519371" \
TZ="Europe/Istanbul" \
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

curl -s http://localhost:8080/api/shift-today?sube=03
```

Check-in kaydı:

```bash
curl -X POST http://localhost:8080/api/checkin \
  -H "Content-Type: application/json" \
  -d '{"tc":"25031519376"}'
```

Allowlist yönetimi:

```bash
curl -X POST http://localhost:8080/api/admin/allowlist \
  -H "Content-Type: application/json" -H "X-Role: admin" \
  -d '{"tc":"25031519376"}'

# test amaçlı bildirim tetikleme
curl -X POST "http://localhost:8080/api/admin/force-notify?tc=25031519376" \
  -H "X-Role: admin"
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
- `CHECKIN_DB_PATH` (varsayılan `./data/checkins.json`): check-in kalıcılığı.
- `ALLOWLIST_INIT`: virgülle ayrılmış TC listesi.
- `TZ` (varsayılan `Europe/Istanbul`)
