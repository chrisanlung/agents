// Root widget: MaterialApp.router + tema Lustia + go_router.
// TODO Phase 5: hubungkan AppRouter setelah semua route screen diimplementasikan.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'core/router/app_router.dart';
import 'core/theme/lustia_theme.dart';

class LustiaApp extends ConsumerWidget {
  const LustiaApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(appRouterProvider);

    return MaterialApp.router(
      title: 'Lustia',
      debugShowCheckedModeBanner: false,
      theme: LustiaTheme.light(),
      darkTheme: LustiaTheme.dark(),
      themeMode: ThemeMode.light, // TODO Phase 6: ikuti system theme
      routerConfig: router,
    );
  }
}
