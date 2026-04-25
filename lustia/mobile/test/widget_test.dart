// Smoke test: app dapat di-mount tanpa crash.
// Menggunakan ProviderScope overrides untuk isolasi.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:lustia_mobile/app.dart';
import 'package:lustia_mobile/core/config/app_config.dart';
import 'package:lustia_mobile/core/router/app_router.dart';

/// Router stub — langsung ke MaterialApp tanpa async timer dari SplashScreen.
final _testRouter = GoRouter(
  initialLocation: '/',
  routes: [
    GoRoute(
      path: '/',
      builder: (_, __) =>
          const Scaffold(body: Center(child: Text('Test Home'))),
    ),
  ],
);

void main() {
  setUpAll(() {
    // AppConfig.configure hanya boleh dipanggil sekali; skip jika sudah dikonfigurasi.
    try {
      AppConfig.configure(flavor: Flavor.dev);
    } catch (_) {
      // Already configured — aman untuk dilanjutkan.
    }
  });

  testWidgets('App smoke test — mount tanpa crash', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          // Override router agar tidak menjalankan splash timer/async
          appRouterProvider.overrideWithValue(_testRouter),
        ],
        child: const LustiaApp(),
      ),
    );

    // Pump sekali untuk settle widget tree awal
    await tester.pump();

    // Pastikan MaterialApp.router ter-render tanpa crash
    expect(find.byType(MaterialApp), findsOneWidget);
    expect(find.text('Test Home'), findsOneWidget);
  });
}
