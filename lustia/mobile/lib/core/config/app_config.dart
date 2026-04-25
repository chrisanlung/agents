// Konfigurasi app berbasis flavor: dev / prod.
// AppConfig.configure() dipanggil sebelum runApp() di main_*.dart.

import 'dart:io' show Platform;

/// Flavor build: dev atau prod.
enum Flavor { dev, prod }

/// Singleton konfigurasi app yang dibaca seluruh aplikasi.
class AppConfig {
  AppConfig._();

  static Flavor _flavor = Flavor.prod;
  static bool _configured = false;

  /// Wajib dipanggil satu kali di main() sebelum runApp().
  static void configure({required Flavor flavor}) {
    assert(!_configured, 'AppConfig.configure() sudah dipanggil sebelumnya.');
    _flavor = flavor;
    _configured = true;
  }

  static Flavor get flavor => _flavor;

  /// Base URL API.
  /// - dev Android emulator : http://10.0.2.2:8080
  /// - dev iOS simulator    : http://localhost:8080
  /// - prod                 : https://api.lustia.id (placeholder)
  static String get baseUrl {
    switch (_flavor) {
      case Flavor.dev:
        // Platform.isIOS tidak tersedia di web; guard dengan try-catch.
        try {
          return Platform.isIOS
              ? 'http://localhost:8080'
              : 'http://10.0.2.2:8080';
        } catch (_) {
          return 'http://localhost:8080';
        }
      case Flavor.prod:
        return 'https://api.lustia.id'; // TODO: ganti saat URL prod tersedia
    }
  }

  static bool get isDev => _flavor == Flavor.dev;
}
