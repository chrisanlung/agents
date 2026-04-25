// Definisi semua route aplikasi Lustia menggunakan go_router.
// ShellRoute membungkus tab bottom-nav (Beranda, Favorit, Booking Saya, Pengaturan).
// Wizard booking + konfirmasi adalah full-screen route di luar shell.

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../features/booking/presentation/screens/booking_confirmation_screen.dart';
import '../../features/booking/presentation/screens/booking_wizard_screen.dart';
import '../../features/booking/presentation/screens/payment_screen.dart';
import '../../features/branch/presentation/screens/branch_detail_screen.dart';
import '../../features/branch/presentation/screens/branch_list_screen.dart';
import '../../features/favorites/presentation/screens/favorites_screen.dart';
import '../../features/my_bookings/presentation/screens/booking_detail_screen.dart';
import '../../features/my_bookings/presentation/screens/my_bookings_screen.dart';
import '../../features/settings/presentation/screens/settings_screen.dart';
import '../../features/splash/presentation/screens/splash_screen.dart';

part 'app_router.g.dart';

// ---------------------------------------------------------------------------
// Route path constants
// ---------------------------------------------------------------------------
abstract final class AppRoutes {
  static const String splash = '/splash';
  static const String home = '/';
  static const String favorites = '/favorites';
  static const String branchDetail = '/branches/:id';
  static const String bookingWizard = '/branches/:id/book';
  static const String payment = '/branches/:id/book/payment';
  static const String confirmation = '/confirmation/:code';
  static const String myBookings = '/my-bookings';
  static const String bookingDetail = '/my-bookings/:code';
  static const String settings = '/settings';
}

// ---------------------------------------------------------------------------
// Shell scaffold with NavigationBar
// ---------------------------------------------------------------------------
class _AppShell extends StatelessWidget {
  const _AppShell({required this.child, required this.navigationShell});
  final Widget child;
  final StatefulNavigationShell navigationShell;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: child,
      bottomNavigationBar: NavigationBar(
        selectedIndex: navigationShell.currentIndex,
        onDestinationSelected: (index) {
          navigationShell.goBranch(
            index,
            initialLocation: index == navigationShell.currentIndex,
          );
        },
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.home_outlined),
            selectedIcon: Icon(Icons.home),
            label: 'Beranda',
          ),
          NavigationDestination(
            icon: Icon(Icons.favorite_outline),
            selectedIcon: Icon(Icons.favorite),
            label: 'Favorit',
          ),
          NavigationDestination(
            icon: Icon(Icons.receipt_long_outlined),
            selectedIcon: Icon(Icons.receipt_long),
            label: 'Booking Saya',
          ),
          NavigationDestination(
            icon: Icon(Icons.settings_outlined),
            selectedIcon: Icon(Icons.settings),
            label: 'Pengaturan',
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Router provider
// ---------------------------------------------------------------------------
@riverpod
GoRouter appRouter(AppRouterRef ref) {
  return GoRouter(
    initialLocation: AppRoutes.splash,
    routes: [
      // Splash + onboarding — tidak masuk shell
      GoRoute(
        path: AppRoutes.splash,
        builder: (context, state) => const SplashScreen(),
      ),

      // Konfirmasi booking — full-screen, tidak bisa kembali ke payment
      GoRoute(
        path: '/confirmation/:code',
        builder: (context, state) {
          final code = state.pathParameters['code']!;
          final extra = state.extra;
          if (extra is BookingConfirmationData) {
            return BookingConfirmationScreen(data: extra);
          }
          // Fallback: minimal data from path
          return BookingConfirmationScreen(
            data: BookingConfirmationData(
              code: code,
              branchName: '',
              serviceName: '',
              scheduledStart: DateTime.now().toIso8601String(),
              scheduledEnd: DateTime.now()
                  .add(const Duration(hours: 1))
                  .toIso8601String(),
              totalPriceIdr: 0,
            ),
          );
        },
      ),

      // StatefulShellRoute: bottom nav
      StatefulShellRoute.indexedStack(
        builder: (context, state, navigationShell) =>
            _AppShell(navigationShell: navigationShell, child: navigationShell),
        branches: [
          // Branch 0: Beranda
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: AppRoutes.home,
                builder: (context, state) => const BranchListScreen(),
              ),
            ],
          ),
          // Branch 1: Favorit
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: AppRoutes.favorites,
                builder: (context, state) => const FavoritesScreen(),
              ),
            ],
          ),
          // Branch 2: Booking Saya
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: AppRoutes.myBookings,
                builder: (context, state) => const MyBookingsScreen(),
                routes: [
                  GoRoute(
                    path: ':code',
                    builder: (context, state) => BookingDetailScreen(
                      code: state.pathParameters['code']!,
                    ),
                  ),
                ],
              ),
            ],
          ),
          // Branch 3: Pengaturan
          StatefulShellBranch(
            routes: [
              GoRoute(
                path: AppRoutes.settings,
                builder: (context, state) => const SettingsScreen(),
              ),
            ],
          ),
        ],
      ),

      // Branch detail — push di atas shell
      GoRoute(
        path: '/branches/:id',
        builder: (context, state) =>
            BranchDetailScreen(branchId: state.pathParameters['id']!),
        routes: [
          // Booking wizard — full-screen
          GoRoute(
            path: 'book',
            builder: (context, state) =>
                BookingWizardScreen(branchId: state.pathParameters['id']!),
            routes: [
              // Payment
              GoRoute(
                path: 'payment',
                builder: (context, state) =>
                    PaymentScreen(branchId: state.pathParameters['id']!),
              ),
            ],
          ),
        ],
      ),
    ],
  );
}
