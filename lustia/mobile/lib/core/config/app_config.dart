// Konfigurasi app berbasis flavor: dev / prod.
// AppConfig.configure() dipanggil sebelum runApp() di main_*.dart.

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
  /// - dev: ngrok tunnel publik supaya HP fisik, emulator, web, dan iOS sim
  ///   semua bisa reach backend tanpa ADB reverse / laptop LAN IP.
  ///   Update URL ini kalau tunnel ngrok rotate.
  /// - prod: https://api.lustia.id (placeholder)
  static String get baseUrl {
    switch (_flavor) {
      case Flavor.dev:
        return 'https://darkish-trifle-kept.ngrok-free.dev';
      case Flavor.prod:
        return 'https://api.lustia.id'; // TODO: ganti saat URL prod tersedia
    }
  }

  static bool get isDev => _flavor == Flavor.dev;
}
