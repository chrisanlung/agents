// Semua string UI yang tampak ke user — Indonesian only (Phase 5).
// TODO Phase 6+: migrasi ke l10n/app_id.arb + flutter_localizations.

abstract final class StringsId {
  // --- Umum ---
  static const String appName = 'Lustia';
  static const String tryAgain = 'Coba Lagi';
  static const String save = 'Simpan';
  static const String cancel = 'Batal';
  static const String next = 'Lanjut';
  static const String back = 'Kembali';
  static const String loading = 'Memuat...';
  static const String noConnection =
      'Tidak dapat terhubung ke server. Periksa koneksi internet Anda.';
  static const String unknownError = 'Terjadi kesalahan. Coba lagi nanti.';

  // --- Splash / Onboarding ---
  static const String splashTagline = 'Spa & Wellness di ujung jari Anda';
  static const String locationPermissionTitle = 'Izin Lokasi';
  static const String locationPermissionBody =
      'Lustia menggunakan lokasi Anda untuk menampilkan cabang terdekat.';
  static const String locationPermissionAllow = 'Izinkan Lokasi';
  static const String locationPermissionSkip = 'Lewati';

  // --- Cabang ---
  static const String branchListTitle = 'Cabang Terdekat';
  static const String branchSearch = 'Cari nama cabang atau kota...';
  static const String branchFilterOpenNow = 'Buka Sekarang';
  static const String branchTabNearest = 'Terdekat';
  static const String branchTabFavorites = 'Favorit';
  static const String branchEmptyNearest = 'Tidak ada cabang ditemukan.';
  static const String branchEmptyFavorites = 'Belum ada cabang favorit.';
  static const String branchBook = 'Booking';

  // --- Booking wizard ---
  static const String bookingTitle = 'Buat Booking';
  static const String bookingStepService = 'Pilih Layanan';
  static const String bookingStepAddons = 'Pilih Add-on';
  static const String bookingStepDateTime = 'Pilih Tanggal & Waktu';
  static const String bookingStepTherapist = 'Pilih Terapis';
  static const String bookingStepRoom = 'Pilih Ruangan';
  static const String bookingStepCustomerInfo = 'Data Pemesan';
  static const String bookingStepSummary = 'Ringkasan Booking';
  static const String bookingAutoAssign = 'Pilihkan saja';
  static const String bookingPayCta = 'Bayar';
  static const String bookingTotal = 'Total';
  static const String bookingNoCancel =
      'Booking yang sudah dibayar tidak dapat dibatalkan atau dikembalikan.';

  // --- Payment ---
  static const String paymentTitle = 'Pembayaran';
  static const String paymentDummyCta = 'Bayar (DUMMY)';
  static const String paymentDummyNote =
      'Mode demo — pembayaran langsung dikonfirmasi.';

  // --- Konfirmasi ---
  static const String confirmationTitle = 'Booking Dikonfirmasi!';
  static const String confirmationShowAtBranch = 'Tunjukkan ini di cabang';
  static const String confirmationEmailSent =
      'Kode booking juga dikirim ke email Anda.';

  // --- Booking Saya ---
  static const String myBookingsTitle = 'Booking Saya';
  static const String myBookingsEmpty =
      'Belum ada booking. Yuk booking sekarang!';

  // --- Pengaturan ---
  static const String settingsTitle = 'Pengaturan';
  static const String settingsTerms = 'Syarat & Ketentuan';
  static const String settingsPrivacy = 'Kebijakan Privasi';
  static const String settingsAppVersion = 'Versi Aplikasi';
}
