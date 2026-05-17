// Entry point untuk build UAT (Sumopod Tencent VPS HTTP backend).
// Flavor: uat → http://43.129.55.236:8080
// Jalankan dev:    flutter run -t lib/main_uat.dart
// Build APK:       flutter build apk --release -t lib/main_uat.dart

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app.dart';
import 'core/config/app_config.dart';

void main() {
  AppConfig.configure(flavor: Flavor.uat);
  runApp(const ProviderScope(child: LustiaApp()));
}
