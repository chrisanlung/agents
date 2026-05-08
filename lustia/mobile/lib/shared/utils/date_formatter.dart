// Utilitas format tanggal dan waktu untuk locale Indonesia.
// Contoh: formatSlot('2026-05-01T10:00:00Z') → "Kamis, 1 Mei 2026 • 10:00"

final class DateFormatter {
  DateFormatter._();

  static const _months = [
    '',
    'Januari',
    'Februari',
    'Maret',
    'April',
    'Mei',
    'Juni',
    'Juli',
    'Agustus',
    'September',
    'Oktober',
    'November',
    'Desember',
  ];

  static const _days = [
    'Senin',
    'Selasa',
    'Rabu',
    'Kamis',
    'Jumat',
    'Sabtu',
    'Minggu',
  ];

  /// Format ISO-8601 string menjadi "Kamis, 1 Mei 2026"
  static String formatDate(String isoString) {
    final dt = DateTime.parse(isoString).toLocal();
    return formatFullDate(dt);
  }

  /// Format [DateTime] menjadi "Kamis, 1 Mei 2026" (Indonesian long format).
  static String formatFullDate(DateTime dt) {
    final dayName = _days[dt.weekday - 1];
    final monthName = _months[dt.month];
    return '$dayName, ${dt.day} $monthName ${dt.year}';
  }

  /// Format ISO-8601 string menjadi "10:00"
  static String formatTime(String isoString) {
    final dt = DateTime.parse(isoString).toLocal();
    final hh = dt.hour.toString().padLeft(2, '0');
    final mm = dt.minute.toString().padLeft(2, '0');
    return '$hh:$mm';
  }

  /// Format slot "10:00 – 11:00"
  static String formatSlotRange(String startIso, String endIso) {
    return '${formatTime(startIso)} – ${formatTime(endIso)}';
  }
}
