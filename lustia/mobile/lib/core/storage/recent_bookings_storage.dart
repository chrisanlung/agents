// RecentBookingsStorage — menyimpan daftar kode booking terakhir di SharedPreferences.
// Digunakan oleh layar "Booking Saya" — tidak memerlukan akun customer.
// Maks 20 entri; entri terlama dibuang saat overflow.
// Storage key: lustia_recent_booking_codes (sesuai BK-A12)

import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

/// Model ringkas yang disimpan di local storage.
/// Fields: code, branchName, serviceName, scheduledStart, totalPriceIdr.
final class RecentBooking {
  const RecentBooking({
    required this.code,
    required this.branchName,
    required this.scheduledStart,
    this.serviceName = '',
    this.totalPriceIdr = 0,
  });

  final String code;
  final String branchName;
  final String scheduledStart; // ISO-8601 UTC string
  final String serviceName;
  final int totalPriceIdr;

  Map<String, dynamic> toJson() => {
    'code': code,
    'branch_name': branchName,
    'scheduled_start': scheduledStart,
    'service_name': serviceName,
    'total_price_idr': totalPriceIdr,
  };

  factory RecentBooking.fromJson(Map<String, dynamic> json) => RecentBooking(
    code: json['code'] as String,
    branchName: json['branch_name'] as String,
    scheduledStart: json['scheduled_start'] as String,
    serviceName: (json['service_name'] as String?) ?? '',
    totalPriceIdr: (json['total_price_idr'] as num?)?.toInt() ?? 0,
  );
}

class RecentBookingsStorage {
  static const String _key = 'lustia_recent_booking_codes';
  static const int _maxEntries = 20;

  Future<List<RecentBooking>> getAll() async {
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getStringList(_key) ?? [];
    final result = <RecentBooking>[];
    for (final e in raw) {
      try {
        result.add(
          RecentBooking.fromJson(jsonDecode(e) as Map<String, dynamic>),
        );
      } catch (_) {
        // Skip malformed entries
      }
    }
    return result;
  }

  Future<void> add(RecentBooking booking) async {
    final prefs = await SharedPreferences.getInstance();
    final raw = List<String>.from(prefs.getStringList(_key) ?? []);

    // Remove if code already exists (no duplicates)
    raw.removeWhere((e) {
      try {
        final decoded = jsonDecode(e) as Map<String, dynamic>;
        return decoded['code'] == booking.code;
      } catch (_) {
        return false;
      }
    });

    raw.add(jsonEncode(booking.toJson()));

    if (raw.length > _maxEntries) {
      raw.removeRange(0, raw.length - _maxEntries);
    }

    await prefs.setStringList(_key, raw);
  }

  Future<void> clear() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_key);
  }
}
