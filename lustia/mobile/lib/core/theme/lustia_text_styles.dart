// Token tipografi Lustia — dipetakan ke Flutter TextStyle.
//
// OPEN QUESTION (ADR 0014a §16.3): DESIGN_SYSTEM.md belum mengisi ukuran/weight
// untuk display-lg, heading-md, body-md, caption.
// Nilai di bawah menggunakan Material 3 defaults sambil menunggu konfirmasi ui-ux-expert.

import 'package:flutter/material.dart';

abstract final class LustiaTextStyles {
  // display-lg: ~28sp, w700
  static const TextStyle displayLg = TextStyle(
    fontSize: 28,
    fontWeight: FontWeight.w700,
    height: 1.25,
  );

  // heading-md: ~20sp, w600
  static const TextStyle headingMd = TextStyle(
    fontSize: 20,
    fontWeight: FontWeight.w600,
    height: 1.3,
  );

  // heading-sm: ~16sp, w600
  static const TextStyle headingSm = TextStyle(
    fontSize: 16,
    fontWeight: FontWeight.w600,
    height: 1.4,
  );

  // body-md: ~14sp, w400
  static const TextStyle bodyMd = TextStyle(
    fontSize: 14,
    fontWeight: FontWeight.w400,
    height: 1.5,
  );

  // body-sm: ~12sp, w400
  static const TextStyle bodySm = TextStyle(
    fontSize: 12,
    fontWeight: FontWeight.w400,
    height: 1.5,
  );

  // caption: ~11sp, w400
  static const TextStyle caption = TextStyle(
    fontSize: 11,
    fontWeight: FontWeight.w400,
    height: 1.4,
  );
}
