// ThemeData Material 3 Lustia — menggunakan token dari lustia_colors.dart.
// TODO Phase 5: sesuaikan typography setelah ui-ux-expert mengisi DESIGN_SYSTEM.md.

import 'package:flutter/material.dart';

import 'lustia_colors.dart';

abstract final class LustiaTheme {
  /// Tema light — digunakan sebagai default di Phase 5.
  static ThemeData light() {
    const colorScheme = ColorScheme(
      brightness: Brightness.light,
      primary: LustiaColors.primary,
      onPrimary: LustiaColors.textOnPrimary,
      primaryContainer: LustiaColors.primaryContainer,
      onPrimaryContainer: LustiaColors.primaryDark,
      secondary: LustiaColors.primaryContainer,
      onSecondary: LustiaColors.primaryDark,
      secondaryContainer: LustiaColors.primarySurface,
      onSecondaryContainer: LustiaColors.primaryDark,
      error: LustiaColors.danger,
      onError: Colors.white,
      errorContainer: Color(0xFFFEE2E2), // red-100
      onErrorContainer: Color(0xFF991B1B), // red-800
      surface: LustiaColors.background,
      onSurface: LustiaColors.onSurface,
      surfaceContainerHighest: LustiaColors.surfaceVariant,
      outline: LustiaColors.borderSubtle,
      outlineVariant: LustiaColors.borderSubtle,
    );

    return ThemeData(
      useMaterial3: true,
      colorScheme: colorScheme,
      scaffoldBackgroundColor: LustiaColors.background,
      appBarTheme: const AppBarTheme(
        backgroundColor: LustiaColors.background,
        foregroundColor: LustiaColors.textPrimary,
        surfaceTintColor: Colors.transparent,
        elevation: 0,
        scrolledUnderElevation: 1,
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: LustiaColors.primary,
          foregroundColor: LustiaColors.textOnPrimary,
          minimumSize: const Size.fromHeight(48),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
          ),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: LustiaColors.primary,
          minimumSize: const Size.fromHeight(48),
          side: const BorderSide(color: LustiaColors.primary),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
          ),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: LustiaColors.surfaceVariant,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide.none,
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: LustiaColors.primary, width: 2),
        ),
        contentPadding: const EdgeInsets.symmetric(
          horizontal: 16,
          vertical: 14,
        ),
      ),
      cardTheme: CardThemeData(
        color: LustiaColors.background,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(16),
          side: const BorderSide(color: LustiaColors.borderSubtle),
        ),
      ),
      chipTheme: ChipThemeData(
        backgroundColor: LustiaColors.primarySurface,
        selectedColor: LustiaColors.primaryContainer,
        labelStyle: const TextStyle(fontSize: 12),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(999)),
      ),
      dividerTheme: const DividerThemeData(
        color: LustiaColors.borderSubtle,
        thickness: 1,
      ),
      bottomNavigationBarTheme: const BottomNavigationBarThemeData(
        backgroundColor: LustiaColors.background,
        selectedItemColor: LustiaColors.primary,
        unselectedItemColor: LustiaColors.textMuted,
        type: BottomNavigationBarType.fixed,
        elevation: 8,
      ),
    );
  }

  /// Tema dark — Phase 6+. Sementara dikembalikan ke light dengan brightness overridden.
  static ThemeData dark() {
    const colorScheme = ColorScheme(
      brightness: Brightness.dark,
      primary: LustiaColors.primaryDarkMode,
      onPrimary: LustiaColors.backgroundDark,
      primaryContainer: Color(0xFF134E4A), // teal-900
      onPrimaryContainer: LustiaColors.primaryDarkMode,
      secondary: Color(0xFF134E4A),
      onSecondary: LustiaColors.primaryDarkMode,
      secondaryContainer: Color(0xFF0F766E),
      onSecondaryContainer: Colors.white,
      error: Color(0xFFF87171), // red-400
      onError: LustiaColors.backgroundDark,
      errorContainer: Color(0xFF7F1D1D),
      onErrorContainer: Color(0xFFFECACA),
      surface: LustiaColors.backgroundDark,
      onSurface: LustiaColors.textPrimaryDark,
      surfaceContainerHighest: LustiaColors.surfaceDark,
      outline: Color(0xFF334155), // slate-700
      outlineVariant: Color(0xFF1E293B),
    );

    return ThemeData(
      useMaterial3: true,
      colorScheme: colorScheme,
      scaffoldBackgroundColor: LustiaColors.backgroundDark,
    );
  }
}
