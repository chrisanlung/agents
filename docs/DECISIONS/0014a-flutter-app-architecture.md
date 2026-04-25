# ADR 0014a — Flutter App Architecture (Lustia Mobile)

- **Status:** Accepted
- **Date:** 2026-04-25
- **Deciders:** flutter-expert, orchestrator
- **Parent ADR:** 0014 (Phase 5 — Booking Engine + Customer Mobile App), §3.18

---

## 1. Context

Phase 5 introduces the first mobile client in the Lustia repository. The app is a guest-only
customer-facing marketplace: discover branches, book treatments, receive a QR code for
check-in. There is no user authentication in Phase 5. The architecture must accommodate
this today while being non-destructive to add auth in a future phase.

---

## 2. Project structure — feature-first

**Decision: feature-first folder layout with a `core/` + `shared/` split.**

Rationale: Lustia Mobile has 4–5 distinct user-facing features (branch discovery, booking
flow, favorites, my bookings, settings). Feature-first groups everything a developer needs
for one feature in one folder (data models, repository, providers, screens, widgets).
Layer-first (`screens/`, `repositories/`, `providers/` at root) forces cross-folder jumps for
every feature change and does not scale beyond 3–4 features without becoming a navigation
maze. Feature-first is the Riverpod 2 community standard.

```
lustia/mobile/
  lib/
    main.dart                  # runApp(ProviderScope(child: LustiaApp()))
    app.dart                   # MaterialApp.router + go_router + theme
    core/
      api/
        dio_client.dart        # Dio singleton factory + BaseOptions
        interceptors/
          auth_interceptor.dart     # placeholder — injects token if present (Phase 6+)
          retry_interceptor.dart    # 1 retry on 5xx / network error
          error_interceptor.dart    # maps DioException → AppException
      config/
        app_config.dart        # env-flavor-aware: base URL, flavor name
        flavor.dart            # enum Flavor { dev, prod }
      theme/
        lustia_theme.dart      # ThemeData + LustiaColors extension
        lustia_colors.dart     # const color tokens from DESIGN_SYSTEM.md
        lustia_text_styles.dart
      storage/
        favorites_storage.dart # SharedPreferences wrapper: List<String> branchIds
        recent_bookings_storage.dart # List<RecentBooking> (code, branchName, scheduledStart)
      location/
        location_service.dart  # geolocator wrapper: getCurrentPosition + permission flow
      exceptions/
        app_exception.dart     # sealed class: NetworkException, ServerException, etc.
    features/
      branch/
        data/
          branch_model.dart    # freezed model
          branch_repository.dart     # interface
          branch_repository_impl.dart
        presentation/
          providers/
            branch_list_provider.dart
            branch_detail_provider.dart
          screens/
            branch_list_screen.dart
            branch_detail_screen.dart
          widgets/
            branch_card.dart
      booking/
        data/
          booking_model.dart
          availability_model.dart
          booking_repository.dart
          booking_repository_impl.dart
        presentation/
          providers/
            booking_provider.dart
            availability_provider.dart
          screens/
            booking_wizard_screen.dart   # multi-step coordinator
            payment_screen.dart
            booking_confirmation_screen.dart
          widgets/
            booking_step_indicator.dart
      favorites/
        data/
          favorites_notifier.dart   # StateNotifier wrapping FavoritesStorage
        presentation/
          providers/
            favorites_provider.dart
      my_bookings/
        data/
          recent_bookings_notifier.dart
        presentation/
          providers/
            my_bookings_provider.dart
          screens/
            my_bookings_screen.dart
          widgets/
            booking_code_card.dart
      settings/
        presentation/
          screens/
            settings_screen.dart
    shared/
      widgets/
        lustia_button.dart
        branch_card.dart       # re-export from feature; or home here — decided per Phase 5
        qr_display.dart        # qr_flutter wrapper
        cached_branch_image.dart
        error_view.dart
        loading_view.dart
        empty_view.dart
      utils/
        currency_formatter.dart
        date_formatter.dart
        haversine.dart         # client-side km label (server sorts; client displays)
  test/
    core/
    features/
      branch/
      booking/
    shared/
```

---

## 3. State management — Riverpod 2 + riverpod_generator

**Decision: Riverpod 2 code-generation (`@riverpod` annotations) throughout.**

- `AsyncNotifierProvider` for paginated lists (branch list, my bookings).
- `NotifierProvider` for sync local state (favorites, booking wizard step state).
- `FutureProvider` for one-shot reads (branch detail, availability slots).
- No `StateProvider` (too loose — explicit Notifier or plain `NotifierProvider` preferred).
- `ref.watch(provider.select(...))` used wherever a widget only needs a sub-field.
- Generated code lives in `*.g.dart` files, excluded from review diffs but committed.

---

## 4. Navigation — go_router 14 + go_router_builder

**Decision: `go_router` with typed route classes via `go_router_builder`.**

Route table (all defined in `app.dart` → extracted to `core/router/app_router.dart`):

| Name | Path | Screen |
|---|---|---|
| `SplashRoute` | `/` | Splash + location onboarding |
| `BranchListRoute` | `/branches` | Branch list (home) |
| `BranchDetailRoute` | `/branches/:id` | Branch detail |
| `BookingRoute` | `/booking` | Booking wizard (wizard state in provider, not URL) |
| `PaymentRoute` | `/payment/:bookingId` | Payment (dummy Phase 5) |
| `ConfirmationRoute` | `/confirmation/:code` | QR + booking code display |
| `MyBookingsRoute` | `/my-bookings` | Recent codes from local storage |
| `SettingsRoute` | `/settings` | Settings + terms |

`ShellRoute` wraps `BranchListRoute`, `MyBookingsRoute`, and `SettingsRoute` for a
persistent `BottomNavigationBar`. Booking wizard and confirmation are full-screen routes
without the bottom bar.

---

## 5. HTTP — Dio + interceptors

**Decision: single `Dio` instance created in `core/api/dio_client.dart`, accessed via
`dioProvider` (Riverpod `Provider`).**

Interceptor stack (in order):
1. `AuthInterceptor` — reads token from secure storage if present; injects
   `Authorization: Bearer` header. Phase 5 always skips (no token). Phase 6 enables.
2. `RetryInterceptor` — retries once on 5xx or `SocketException`, with 500ms delay.
3. `ErrorInterceptor` — translates `DioException` into `AppException` sealed variants for
   consistent error handling in providers.

`RequestOptions.extra['noRetry'] = true` flag respected by `RetryInterceptor` for
non-idempotent endpoints (POST booking create — should not auto-retry).

---

## 6. Local storage — shared_preferences

Two keys:
- `favorites_branch_ids` — `List<String>` (branch UUIDs). Max 100 entries; oldest dropped
  on overflow.
- `recent_bookings` — JSON-encoded `List<RecentBooking>`. Max 20 entries; oldest dropped.
  `RecentBooking` fields: `code`, `branchName`, `scheduledStart` (ISO-8601 string).

No encryption needed (no PII, no tokens stored locally in Phase 5). Phase 6 adds
`flutter_secure_storage` for auth tokens and that decision will be reviewed by
`security-expert` at that time.

---

## 7. Geolocation

`LocationService` wraps `geolocator`:
1. Check `LocationPermission` — if denied, call `requestPermission()`.
2. If `deniedForever` — open app settings via `Geolocator.openAppSettings()`.
3. On success: return `Position` (lat, lng).
4. `locationServiceEnabled` checked before requesting position; graceful fallback
   (list shown without distance sort) if service disabled.

Exposed as `locationProvider` (`FutureProvider<Position?>`) — nullable; null = user
declined or service unavailable.

---

## 8. Build flavors — dev / prod

Two Dart entry points:
- `lib/main_dev.dart` → sets `AppConfig.flavor = Flavor.dev`
- `lib/main_prod.dart` → sets `AppConfig.flavor = Flavor.prod`

Both call `runApp(ProviderScope(child: LustiaApp()))`.

`AppConfig` static fields set before `runApp`:
- `Flavor.dev`: `baseUrl = 'http://10.0.2.2:8080'` (Android emulator) —
  `http://localhost:8080` injected at runtime via platform check (`Platform.isIOS`).
- `Flavor.prod`: `baseUrl = 'https://api.lustia.id'` (placeholder).

`flutter run --flavor dev -t lib/main_dev.dart`
`flutter run --flavor prod -t lib/main_prod.dart`

Android `build.gradle` flavor dimensions: `flavorDimensions = ["env"]`; productFlavors
`dev` and `prod` with `applicationIdSuffix ".dev"` on dev flavor.

iOS schemes: `Dev` and `Prod` added to `Runner.xcodeproj/xcshareddata/xcschemes/`.

---

## 9. Images — cached_network_image

`CachedBranchImage` widget in `shared/widgets/cached_branch_image.dart`:
- `maxWidthDiskCache: 800`, `maxHeightDiskCache: 800` (branch hero images).
- `fadeInDuration: Duration(milliseconds: 200)`.
- Placeholder: `LustiaColors.surfaceVariant` shimmer via `Container`.
- Error: `Icon(Icons.broken_image_outlined)` centered.

---

## 10. QR generation — qr_flutter

`QrDisplay` widget in `shared/widgets/qr_display.dart`:
- Renders `QrImageView` with `data: bookingCode` (the 9-char string, e.g. `B7K3-M2QF`).
- `size: 220`, `version: QrVersions.auto`, `errorCorrectionLevel: QrErrorCorrectLevel.M`.
- Wrapped in a `RepaintBoundary` for share functionality (Phase 6).

---

## 11. i18n

Phase 5: Indonesian only. All user-visible strings defined as `const` string constants
in `shared/utils/strings_id.dart` rather than ARB files, to avoid the `flutter gen-l10n`
overhead for a single locale. When a second locale is needed (Phase 6+), strings are
migrated to `l10n/app_id.arb` + `app_en.arb` and `flutter_localizations` is enabled.
This is the one deliberate shortcut for Phase 5 velocity; the migration path is clear.

---

## 12. App icon + splash

Source asset: `assets/icon/lustia_icon_source.png` (1024×1024, placeholder).
- `flutter_launcher_icons` config in `pubspec.yaml` under `flutter_launcher_icons:`.
- `flutter_native_splash` config in `pubspec.yaml` under `flutter_native_splash:`.
- Run `dart run flutter_launcher_icons` and `dart run flutter_native_splash` after the
  real brand asset arrives. Not run during scaffolding (asset is placeholder).

---

## 13. Testing strategy

| Layer | Tool | Target coverage |
|---|---|---|
| Models (freezed) | `dart test` | 100% (generated, trivial) |
| Repository (Dio) | `dart test` + `http_mock_adapter` | Happy path + 4xx/5xx/network |
| Providers (Riverpod) | `flutter_test` + `ProviderContainer` | State transitions, error state |
| Screens | `testWidgets` + `find.byType` + `tester.pumpAndSettle()` | Smoke: renders, taps |
| Golden | `matchesGoldenFile` | `BranchCard`, `QrDisplay`, `BookingConfirmationScreen` |
| E2E | `integration_test` + emulator (CI) | Full booking flow, favorites, my-bookings |

Mock strategy: `http_mock_adapter` (`DioAdapterMockito` or `HttpClientAdapter`) for
unit/widget tests. `Riverpod overrides` at `ProviderContainer` level — never mock
providers globally in widget tests.

---

## 14. Minimum SDK targets

- Android: `minSdkVersion 21` (Android 5.0 — ~99% global coverage at 2026).
  `targetSdkVersion` latest (API 35 as of Flutter stable 3.24+).
- iOS: deployment target `13.0` — required by several dependencies (geolocator,
  permission_handler). Covers 98%+ of active iOS devices.

---

## 15. Package versions (pubspec.yaml)

| Package | Version pin | Purpose |
|---|---|---|
| `flutter_riverpod` | `^2.6.1` | State management |
| `riverpod_annotation` | `^2.3.5` | Code-gen annotations |
| `riverpod_generator` | `^2.4.3` | `build_runner` code-gen |
| `go_router` | `^14.6.2` | Navigation |
| `go_router_builder` | `^2.7.1` | Typed routes |
| `dio` | `^5.7.0` | HTTP client |
| `shared_preferences` | `^2.3.2` | Local key-value store |
| `geolocator` | `^13.0.2` | Device GPS |
| `permission_handler` | `^11.3.1` | Permission request flow |
| `cached_network_image` | `^3.4.1` | Network image w/ disk cache |
| `qr_flutter` | `^4.1.0` | QR code widget |
| `freezed` | `^2.5.7` | Immutable data classes |
| `freezed_annotation` | `^2.4.4` | Annotations |
| `json_serializable` | `^6.8.0` | JSON serialization |
| `build_runner` | `^2.4.12` | Code generation |
| `flutter_launcher_icons` | `^0.14.1` | App icon generation |
| `flutter_native_splash` | `^2.4.1` | Splash screen generation |
| `flutter_lints` | `^4.0.0` | Lint rules |

---

## 16. Open questions (for orchestrator resolution before screen implementation)

1. **Brand asset**: `assets/icon/lustia_icon_source.png` is a placeholder. Real 1024×1024
   PNG required from `ui-ux-expert` before generating app icon / splash.
2. **DESIGN_SYSTEM.md color tokens are stubs** — the table rows exist but values are
   blank. The theme in `core/theme/lustia_theme.dart` currently uses teal-600 extracted
   from `tenant-admin/globals.css` (`HSL 175 84% 32%`). Confirm this is the correct
   customer-app primary, or request `ui-ux-expert` to define a distinct mobile palette.
3. **Typography**: `DESIGN_SYSTEM.md` type-scale tokens (`display-lg`, `heading-md`, etc.)
   have no size/weight values yet. Phase 5 scaffold uses Material 3 defaults; `ui-ux-expert`
   should fill the token table before screen implementation begins.
4. **Booking wizard step count**: ADR 0014 §3.18 lists 7 steps. The wizard screen
   implementation will need UX flow approval before coding begins (see
   `docs/DESIGN_SYSTEM.md` — Phase 5 screen specs not yet written by `ui-ux-expert`).
5. **go_router_builder typed routes**: requires `build_runner`. Confirm CI pipeline
   (`devops-expert`) runs `dart run build_runner build` before `flutter build`.
