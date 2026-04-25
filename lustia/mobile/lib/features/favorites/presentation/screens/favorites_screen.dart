// Layar favorit — daftar cabang yang di-bookmark user.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../branch/presentation/providers/branch_list_provider.dart';
import '../../../branch/presentation/widgets/branch_card.dart';
import '../../data/favorites_notifier.dart';

class FavoritesScreen extends ConsumerWidget {
  const FavoritesScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final favoritesAsync = ref.watch(favoritesProvider);
    final branchListAsync = ref.watch(branchListNotifierProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text('Favorit', style: theme.textTheme.headlineMedium),
      ),
      body: favoritesAsync.when(
        loading: () =>
            const Center(child: CircularProgressIndicator.adaptive()),
        error: (_, __) => const Center(child: Text('Gagal memuat favorit.')),
        data: (favoriteIds) {
          if (favoriteIds.isEmpty) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.all(32),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      Icons.favorite_outline,
                      size: 64,
                      color: cs.onSurfaceVariant.withAlpha(102),
                    ),
                    const SizedBox(height: 16),
                    Text(
                      'Belum ada favorit',
                      style: theme.textTheme.titleMedium?.copyWith(
                        color: cs.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      'Ketuk ikon hati pada cabang untuk menyimpannya di sini.',
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: cs.onSurfaceVariant,
                      ),
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 24),
                    FilledButton(
                      onPressed: () => context.go('/'),
                      child: const Text('Cari Cabang'),
                    ),
                  ],
                ),
              ),
            );
          }

          // Filter branch list for favorites
          final branches =
              branchListAsync.valueOrNull?.items
                  .where((b) => favoriteIds.contains(b.id))
                  .toList() ??
              [];

          if (branches.isEmpty) {
            return const Center(
              child: Padding(
                padding: EdgeInsets.all(24),
                child: Text(
                  'Cabang favorit tidak ditemukan di daftar saat ini.',
                  textAlign: TextAlign.center,
                ),
              ),
            );
          }

          return ListView.builder(
            padding: const EdgeInsets.only(top: 8, bottom: 24),
            itemCount: branches.length,
            itemBuilder: (_, i) => BranchCard(branch: branches[i]),
          );
        },
      ),
    );
  }
}
