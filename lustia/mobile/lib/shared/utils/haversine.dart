// Fungsi Haversine — menghitung jarak antara dua koordinat (km).
// Digunakan untuk menampilkan label jarak di BranchCard.
// Catatan: server yang melakukan sorting by distance; ini hanya untuk label tampilan.

import 'dart:math' as math;

/// Menghitung jarak dalam kilometer antara dua titik koordinat.
double haversineKm(double lat1, double lon1, double lat2, double lon2) {
  const r = 6371.0; // Radius bumi (km)
  final dLat = _deg2rad(lat2 - lat1);
  final dLon = _deg2rad(lon2 - lon1);
  final a =
      math.sin(dLat / 2) * math.sin(dLat / 2) +
      math.cos(_deg2rad(lat1)) *
          math.cos(_deg2rad(lat2)) *
          math.sin(dLon / 2) *
          math.sin(dLon / 2);
  final c = 2 * math.atan2(math.sqrt(a), math.sqrt(1 - a));
  return r * c;
}

double _deg2rad(double deg) => deg * (math.pi / 180);

/// Format jarak untuk tampilan.
/// < 1 km → "800 m", ≥ 1 km → "2,5 km"
String formatDistance(double km) {
  if (km < 1.0) {
    return '${(km * 1000).round()} m';
  }
  return '${km.toStringAsFixed(1)} km';
}
