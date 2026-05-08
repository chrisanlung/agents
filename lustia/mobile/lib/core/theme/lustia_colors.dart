// Token warna Lustia — palet customer mobile menyatu dengan brand logo coral.
// Logo: woman silhouette dengan warna coral salmon (~#F87171). Tenant-admin
// + ops portal masih pakai teal — palet ini khusus customer mobile (per
// permintaan user 2026-05-08: "ingat itu untuk menjadi warna primary di mobile").

import 'package:flutter/material.dart';

/// Warna brand Lustia — gunakan via LustiaColors, bukan hardcode hex di widget.
abstract final class LustiaColors {
  // --- Brand primary: Coral (matching lustia-logo.png) ---
  /// Coral utama — match logo. Tailwind red-400 ≈ #F87171
  static const Color primary = Color(0xFFF87171);

  /// Red-500 — pressed / hover state, sedikit lebih dalam dari primary
  static const Color primaryDark = Color(0xFFEF4444);

  /// Red-50 — permukaan ringan / chip background
  static const Color primarySurface = Color(0xFFFEF2F2);

  /// Red-200 — secondary container (badge, soft fill)
  static const Color primaryContainer = Color(0xFFFECACA);

  // --- Neutral / surface ---
  static const Color background = Color(0xFFFFFFFF);
  static const Color surfaceVariant = Color(0xFFF1F5F9); // slate-100
  static const Color onSurface = Color(0xFF0F172A); // slate-900

  // --- Text ---
  static const Color textPrimary = Color(0xFF0F172A); // slate-900
  static const Color textMuted = Color(0xFF64748B); // slate-500
  static const Color textOnPrimary = Color(0xFFFFFFFF);

  // --- Semantic ---
  /// Hijau — status selesai / sukses
  static const Color success = Color(0xFF16A34A); // green-600

  /// Merah — error / bahaya
  static const Color danger = Color(0xFFDC2626); // red-600

  /// Amber — peringatan
  static const Color warning = Color(0xFFD97706); // amber-600

  // --- Border ---
  static const Color borderSubtle = Color(0xFFE2E8F0); // slate-200

  // --- Dark mode overrides ---
  static const Color backgroundDark = Color(0xFF0F172A);
  static const Color surfaceDark = Color(0xFF1E293B);
  static const Color textPrimaryDark = Color(0xFFF8FAFC);
  static const Color textMutedDark = Color(0xFF94A3B8);
  static const Color primaryDarkMode = Color(
    0xFFFCA5A5,
  ); // red-300 (coral lebih terang untuk dark mode)
}
