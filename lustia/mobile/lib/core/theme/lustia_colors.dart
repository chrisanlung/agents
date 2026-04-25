// Token warna Lustia — ditranslasikan dari DESIGN_SYSTEM.md + globals.css tenant-admin.
// Sumber: tenant-admin/app/globals.css → --primary: HSL(175, 84%, 32%) = Teal-600.
//
// OPEN QUESTION (ADR 0014a §16.2): ui-ux-expert perlu konfirmasi apakah
// palet ini berlaku untuk customer app, atau ada palet terpisah.
// Sementara pakai teal-600 (konsisten dengan tenant-admin).

import 'package:flutter/material.dart';

/// Warna brand Lustia — gunakan via LustiaColors, bukan hardcode hex di widget.
abstract final class LustiaColors {
  // --- Brand primary: Teal ---
  /// Teal-600: HSL(175, 84%, 32%) ≈ #0D9488
  static const Color primary = Color(0xFF0D9488);

  /// Teal-700 — pressed / hover state
  static const Color primaryDark = Color(0xFF0F766E);

  /// Teal-50 — permukaan ringan / chip background
  static const Color primarySurface = Color(0xFFF0FDFA);

  /// Teal-100 — secondary container
  static const Color primaryContainer = Color(0xFFCCFBF1);

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
    0xFF2DD4BF,
  ); // teal-400 (lebih terang di dark)
}
