// Widget card cabang — BranchListScreen dan tab Favorit (BK-A3).

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../shared/widgets/cached_branch_image.dart';
import '../../../favorites/data/favorites_notifier.dart';
import '../../data/branch_model.dart';

class BranchCard extends ConsumerWidget {
  const BranchCard({super.key, required this.branch});

  final BranchSummary branch;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    final favoritesAsync = ref.watch(favoritesProvider);
    final isFavorite = favoritesAsync.maybeWhen(
      data: (list) => list.contains(branch.id),
      orElse: () => false,
    );

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: () => context.push('/branches/${branch.id}'),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Foto thumbnail
              ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: CachedBranchImage(
                  imageUrl: branch.photoUrl,
                  width: 96,
                  height: 96,
                ),
              ),
              const SizedBox(width: 12),
              // Info
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Tenant name
                    Text(
                      branch.tenantName,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: cs.onSurfaceVariant,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 2),
                    // Branch name
                    Text(
                      branch.name,
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 4),
                    // Address
                    Row(
                      children: [
                        Icon(
                          Icons.location_on_outlined,
                          size: 14,
                          color: cs.onSurfaceVariant,
                        ),
                        const SizedBox(width: 2),
                        Expanded(
                          child: Text(
                            branch.shortAddress,
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: cs.onSurfaceVariant,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 4),
                    // Distance + favorite
                    Row(
                      children: [
                        if (branch.distanceLabel != null)
                          Text(
                            branch.distanceLabel!,
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: cs.primary,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        const Spacer(),
                        // Favorite toggle — 48dp tap target via IconButton
                        SizedBox(
                          width: 40,
                          height: 40,
                          child: IconButton(
                            padding: EdgeInsets.zero,
                            icon: Icon(
                              isFavorite
                                  ? Icons.favorite
                                  : Icons.favorite_outline,
                              size: 20,
                              color: isFavorite
                                  ? cs.error
                                  : cs.onSurfaceVariant,
                            ),
                            tooltip: isFavorite
                                ? 'Hapus dari favorit'
                                : 'Simpan ke favorit',
                            onPressed: () {
                              ref
                                  .read(favoritesProvider.notifier)
                                  .toggle(branch.id);
                            },
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
