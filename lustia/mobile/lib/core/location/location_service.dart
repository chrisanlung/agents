// LocationService — wrapper geolocator untuk mendapatkan posisi device.
// ignore_for_file: deprecated_member_use_from_same_package
// Alur:
//   1. Periksa apakah location service aktif.
//   2. Periksa permission → minta jika belum diberikan.
//   3. Jika deniedForever → buka app settings.
//   4. Kembalikan Position atau null (graceful fallback).

import 'package:geolocator/geolocator.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'location_service.g.dart';

class LocationService {
  /// Mendapatkan posisi saat ini.
  /// Mengembalikan null jika permission ditolak / service mati / platform
  /// tidak mendukung (mis. Flutter web di Incognito tanpa permission browser).
  Future<Position?> getCurrentPosition() async {
    try {
      // 1. Periksa service aktif
      final serviceEnabled = await Geolocator.isLocationServiceEnabled();
      if (!serviceEnabled) return null;

      // 2. Periksa dan minta permission
      var permission = await Geolocator.checkPermission();
      if (permission == LocationPermission.denied) {
        permission = await Geolocator.requestPermission();
        if (permission == LocationPermission.denied) return null;
      }
      if (permission == LocationPermission.deniedForever) {
        // openAppSettings tidak diimplementasi di web — jangan panggil kalau
        // bisa throw. Cukup return null, biarkan backend kasih hasil tanpa
        // sort-by-distance.
        return null;
      }

      // 3. Dapatkan posisi
      return await Geolocator.getCurrentPosition(
        locationSettings: const LocationSettings(
          accuracy: LocationAccuracy.medium,
          timeLimit: Duration(seconds: 8),
        ),
      );
    } catch (_) {
      // Graceful fallback: any geolocation error (web platform, denied,
      // timeout, missing implementation) → no location, branches still load.
      return null;
    }
  }
}

/// Provider FutureProvider untuk posisi — nullable jika tidak tersedia.
@riverpod
Future<Position?> currentLocation(CurrentLocationRef ref) async {
  final service = LocationService();
  return service.getCurrentPosition();
}
