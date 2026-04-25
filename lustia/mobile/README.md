# Lustia Mobile

Aplikasi customer-facing Lustia — temukan cabang spa & wellness terdekat, buat booking,
dan terima kode QR untuk check-in.

- **Platform:** Android (minSdk 21) + iOS (target 13+)
- **Flutter channel:** stable (3.35+, Dart 3.9+)
- **State management:** Riverpod 2 + riverpod_generator
- **Navigasi:** go_router

---

## Prasyarat

```
flutter --version    # harus >= 3.35 stable
```

---

## Pengaturan awal

```bash
cd lustia/mobile
flutter pub get
dart run build_runner build --delete-conflicting-outputs
```

---

## Menjalankan aplikasi

### Flavor dev (Android emulator / iOS simulator)

```bash
# Android emulator (API URL: http://10.0.2.2:8080)
flutter run --flavor dev -t lib/main_dev.dart

# iOS simulator (API URL: http://localhost:8080)
flutter run --flavor dev -t lib/main_dev.dart
```

### Flavor prod

```bash
flutter run --flavor prod -t lib/main.dart
```

---

## Menjalankan tests

```bash
flutter test
```

---

## Build release

```bash
# Android AAB
flutter build appbundle --flavor prod -t lib/main.dart

# iOS
flutter build ipa --flavor prod -t lib/main.dart
```

---

## Generate app icon + splash screen

Lakukan SETELAH mengganti `assets/icon/lustia_icon_source.png` dengan aset brand asli
(1024x1024 PNG).

```bash
dart run flutter_launcher_icons
dart run flutter_native_splash:create
```

---

## Phase 5 — layar yang diimplementasikan

| Layar | Route | Spesifikasi |
|---|---|---|
| Splash + onboarding lokasi | `/splash` | BK-A2 |
| Beranda (daftar cabang) | `/` | BK-A3 |
| Detail cabang | `/branches/:id` | BK-A4 |
| Wizard booking (7 langkah) | `/branches/:id/book` | BK-A5..A9 |
| Pembayaran (DUMMY) | `/branches/:id/book/payment` | BK-A10 |
| Konfirmasi booking + QR | `/confirmation/:code` | BK-A11 |
| Booking Saya | `/my-bookings` | BK-A12 |
| Detail booking (re-show QR) | `/my-bookings/:code` | BK-A12 |
| Favorit | `/favorites` | BK-A1 |
| Pengaturan | `/settings` | BK-A1 |

**Catatan Phase 5:**
- Pembayaran berjalan di mode DUMMY — klik "Bayar" akan memanggil `POST /public/bookings`
  lalu `POST /public/payments/webhook` dengan payload dummy. Tidak membutuhkan Midtrans SDK.
- Tidak ada deep linking (phase 5 scope).
- Favorit disimpan di `shared_preferences` (lokal, hapus saat uninstall).
- Booking Saya disimpan di `shared_preferences` dengan key `lustia_recent_booking_codes`.

---

## Struktur folder

```
lib/
  main.dart          # entry point prod
  main_dev.dart      # entry point dev
  app.dart           # root widget + router + theme
  core/              # shared infrastructure (api, config, theme, storage, location)
  features/          # fitur per domain (branch, booking, favorites, my_bookings, settings)
  shared/            # widget + utilitas reusable
test/
assets/
  icon/              # sumber app icon
  images/            # aset statis lain
```

Lihat `docs/DECISIONS/0014a-flutter-app-architecture.md` untuk keputusan arsitektur lengkap.
