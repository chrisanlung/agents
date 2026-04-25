// Widget gambar cabang dengan disk cache.
// Digunakan di BranchCard dan BranchDetailScreen.

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';

import '../../core/theme/lustia_colors.dart';

/// Menampilkan gambar cabang dari URL dengan fade-in + placeholder shimmer.
/// [imageUrl] boleh null — ditampilkan placeholder ikon spa.
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
          child: const ColoredBox(
            color: LustiaColors.surfaceVariant,
            child: Center(child: Icon(Icons.spa, color: LustiaColors.primary)),
          ),
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
          child: const ColoredBox(
            color: LustiaColors.surfaceVariant,
            child: Center(
              child: Icon(
                Icons.broken_image_outlined,
                color: LustiaColors.textMuted,
              ),
            ),
          ),
        ),
      ),
    );
  }
}
