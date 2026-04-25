// Entry point untuk build prod.
// Flavor: prod → API URL production (placeholder).
// Lihat juga: main_dev.dart untuk flavor dev.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app.dart';
import 'core/config/app_config.dart';

void main() {
  AppConfig.configure(flavor: Flavor.prod);
  runApp(const ProviderScope(child: LustiaApp()));
}
