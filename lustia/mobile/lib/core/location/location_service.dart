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
  /// Mengembalikan null jika permission ditolak atau service mati.
  Future<Position?> getCurrentPosition() async {
    // 1. Periksa service aktif
    final serviceEnabled = await Geolocator.isLocationServiceEnabled();
    if (!serviceEnabled) {
      // Tidak buka dialog service — biarkan user aktifkan sendiri.
      return null;
    }

    // 2. Periksa dan minta permission
    var permission = await Geolocator.checkPermission();
    if (permission == LocationPermission.denied) {
      permission = await Geolocator.requestPermission();
      if (permission == LocationPermission.denied) {
        return null;
      }
    }

    if (permission == LocationPermission.deniedForever) {
      // Arahkan ke app settings agar user bisa mengaktifkan manual
      await Geolocator.openAppSettings();
      return null;
    }

    // 3. Dapatkan posisi
    try {
      return await Geolocator.getCurrentPosition(
        locationSettings: const LocationSettings(
          accuracy: LocationAccuracy.medium, // cukup untuk sort by distance
          timeLimit: Duration(seconds: 8),
        ),
      );
    } catch (_) {
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
