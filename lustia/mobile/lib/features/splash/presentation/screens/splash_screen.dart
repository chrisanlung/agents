// Splash + onboarding lokasi (BK-A2).
// Ditampilkan di launch pertama; setelah prefetch selesai → navigasi ke /branches.

import 'dart:async' show unawaited;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:geolocator/geolocator.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../../../core/theme/lustia_colors.dart';
import '../../../branch/presentation/providers/branch_list_provider.dart';

const _onboardingKey = 'onboarding_location_asked';

class SplashScreen extends ConsumerStatefulWidget {
  const SplashScreen({super.key});

  @override
  ConsumerState<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends ConsumerState<SplashScreen> {
  bool _showOnboarding = false;
  bool _prefetchDone = false;
  bool _splashMinDone = false;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    // Minimum 400ms splash — just long enough to avoid flash, no slower.
    Future.delayed(const Duration(milliseconds: 400), () {
      if (mounted) {
        setState(() => _splashMinDone = true);
        _maybeNavigate();
      }
    });

    // Check onboarding flag
    final prefs = await SharedPreferences.getInstance();
    final asked = prefs.getBool(_onboardingKey) ?? false;

    if (!asked) {
      // Check current permission status. Wrap in try/catch — Geolocator throws
      // if AndroidManifest is missing permissions or the platform plugin
      // can't initialise (Flutter web Incognito, etc.). Treat any error as
      // "denied" → show onboarding card so the user can grant later.
      try {
        final permission = await Geolocator.checkPermission();
        if (permission == LocationPermission.denied ||
            permission == LocationPermission.unableToDetermine) {
          if (mounted) setState(() => _showOnboarding = true);
        }
      } catch (_) {
        if (mounted) setState(() => _showOnboarding = true);
      }
    }

    // Prefetch branch list in background — unawaited by design
    // (splash fires off the fetch, the .then callback updates state)
    unawaited(
      ref
          .read(branchListNotifierProvider.future)
          .then((_) {
            if (mounted) {
              setState(() => _prefetchDone = true);
              _maybeNavigate();
            }
          })
          .catchError((_) {
            if (mounted) {
              setState(() => _prefetchDone = true);
              _maybeNavigate();
            }
          }),
    );
  }

  void _maybeNavigate() {
    if (_splashMinDone && _prefetchDone && !_showOnboarding) {
      if (mounted) context.go('/');
    }
  }

  Future<void> _allowLocation() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_onboardingKey, true);

    final permission = await Geolocator.requestPermission();
    if (!mounted) return;

    if (permission == LocationPermission.deniedForever) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Aktifkan lokasi di Pengaturan perangkat'),
        ),
      );
    }

    setState(() => _showOnboarding = false);
    _maybeNavigate();
  }

  Future<void> _skipLocation() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_onboardingKey, true);
    if (mounted) {
      setState(() => _showOnboarding = false);
      _maybeNavigate();
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_showOnboarding && _splashMinDone) {
      return _OnboardingScreen(onAllow: _allowLocation, onSkip: _skipLocation);
    }
    return const _SplashContent();
  }
}

class _SplashContent extends StatelessWidget {
  const _SplashContent();

  @override
  Widget build(BuildContext context) {
    // Cream background matches the splash image edge color (#F8E5E1, same
    // as flutter_native_splash config in pubspec.yaml). BoxFit.contain so
    // the whole image is visible without cropping; letterbox blends with
    // the background so the seam is invisible at boot.
    return const Scaffold(
      backgroundColor: Color(0xFFF8E5E1),
      body: SafeArea(
        child: Center(
          child: Image(
            image: AssetImage('assets/images/splash.png'),
            fit: BoxFit.contain,
          ),
        ),
      ),
    );
  }
}

class _OnboardingScreen extends StatelessWidget {
  const _OnboardingScreen({required this.onAllow, required this.onSkip});

  final VoidCallback onAllow;
  final VoidCallback onSkip;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 24),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Icon(
                Icons.location_on_outlined,
                size: 80,
                color: LustiaColors.primary,
              ),
              const SizedBox(height: 32),
              Text(
                'Temukan cabang terdekat dari kamu',
                style: theme.textTheme.headlineMedium?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 16),
              ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 280),
                child: Text(
                  'Izinkan Lustia mengakses lokasimu agar kami bisa menampilkan cabang spa dan klinik yang paling dekat denganmu.',
                  style: theme.textTheme.bodyLarge,
                  textAlign: TextAlign.center,
                ),
              ),
              const SizedBox(height: 32),
              FilledButton(
                onPressed: onAllow,
                style: FilledButton.styleFrom(
                  minimumSize: const Size(double.infinity, 52),
                ),
                child: const Text('Izinkan Lokasi'),
              ),
              const SizedBox(height: 12),
              OutlinedButton(
                onPressed: onSkip,
                style: OutlinedButton.styleFrom(
                  minimumSize: const Size(double.infinity, 52),
                ),
                child: const Text('Lewati, cari manual'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
