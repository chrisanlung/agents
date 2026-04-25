// Entry point untuk build dev.
// Flavor: dev → API URL emulator (10.0.2.2:8080 Android / localhost:8080 iOS).
// Jalankan: flutter run --flavor dev -t lib/main_dev.dart

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app.dart';
import 'core/config/app_config.dart';

void main() {
  AppConfig.configure(flavor: Flavor.dev);
  runApp(const ProviderScope(child: LustiaApp()));
}
