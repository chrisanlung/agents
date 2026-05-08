// Widget gambar cabang dengan disk cache.
// Digunakan di BranchCard dan BranchDetailScreen.

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';

import '../../core/theme/lustia_colors.dart';

/// Menampilkan gambar cabang dari URL dengan fade-in + placeholder shimmer.
/// [imageUrl] boleh null — ditampilkan logo Lustia dari assets sebagai fallback.
class CachedBranchImage extends StatelessWidget {
  const CachedBranchImage({
    super.key,
    required this.imageUrl,
    this.width,
    this.height,
    this.fit = BoxFit.cover,
    this.borderRadius = 12.0,
  });

  final String? imageUrl;
  final double? width;
  final double? height;
  final BoxFit fit;
  final double borderRadius;

  @override
  Widget build(BuildContext context) {
    final url = imageUrl;
    if (url == null || url.isEmpty) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(borderRadius),
        child: SizedBox(
          width: width,
          height: height,
          child: const _LustiaLogoFallback(),
        ),
      );
    }

    return ClipRRect(
      borderRadius: BorderRadius.circular(borderRadius),
      child: CachedNetworkImage(
        imageUrl: url,
        width: width,
        height: height,
        fit: fit,
        maxWidthDiskCache: 800,
        maxHeightDiskCache: 800,
        fadeInDuration: const Duration(milliseconds: 200),
        placeholder: (context, _) => SizedBox(
          width: width,
          height: height,
          child: const ColoredBox(color: LustiaColors.surfaceVariant),
        ),
        errorWidget: (context, _, __) => SizedBox(
          width: width,
          height: height,
          child: const _LustiaLogoFallback(),
        ),
      ),
    );
  }
}

/// Branding fallback ketika foto cabang belum di-upload atau gagal di-load.
/// Memakai aset `lustia-logo.png` (bukan ikon spa Material) supaya konsisten
/// dengan brand pada layar lain.
class _LustiaLogoFallback extends StatelessWidget {
  const _LustiaLogoFallback();

  @override
  Widget build(BuildContext context) {
    return ColoredBox(
      color: LustiaColors.surfaceVariant,
      child: Center(
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Image.asset('assets/images/lustia-logo.png'),
        ),
      ),
    );
  }
}
