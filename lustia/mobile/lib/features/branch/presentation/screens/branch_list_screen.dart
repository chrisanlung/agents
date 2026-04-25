// Layar daftar cabang — halaman utama "Beranda" (BK-A3).

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../shared/widgets/error_view.dart';
import '../providers/branch_list_provider.dart';
import '../widgets/branch_card.dart';

/// Kategori tersedia untuk filter chip.
const _categories = ['', 'Pijat', 'Facial', 'Nail Art', 'Lainnya'];

class BranchListScreen extends ConsumerStatefulWidget {
  const BranchListScreen({super.key});

  @override
  ConsumerState<BranchListScreen> createState() => _BranchListScreenState();
}

class _BranchListScreenState extends ConsumerState<BranchListScreen> {
  final _scrollController = ScrollController();
  final _searchController = TextEditingController();
  BranchFilterState _filter = const BranchFilterState();
  bool _searchExpanded = false;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
  }

  @override
  void dispose() {
    _scrollController.dispose();
    _searchController.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 200) {
      ref.read(branchListNotifierProvider.notifier).loadNextPage();
    }
  }

  void _applyFilter(BranchFilterState newFilter) {
    setState(() => _filter = newFilter);
    ref.read(branchListNotifierProvider.notifier).applyFilter(newFilter);
  }

  void _onSearchChanged(String query) {
    final newFilter = _filter.copyWith(query: query);
    // Debounce via setState + re-apply
    setState(() => _filter = newFilter);
    Future.delayed(const Duration(milliseconds: 300), () {
      if (_filter.query == query && mounted) {
        ref.read(branchListNotifierProvider.notifier).applyFilter(_filter);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final async = ref.watch(branchListNotifierProvider);

    return Scaffold(
      appBar: AppBar(
        title: _searchExpanded
            ? TextField(
                controller: _searchController,
                autofocus: true,
                decoration: const InputDecoration(
                  hintText: 'Cari cabang atau area…',
                  border: InputBorder.none,
                  filled: false,
                  contentPadding: EdgeInsets.zero,
                ),
                onChanged: _onSearchChanged,
              )
            : Text('Lustia', style: theme.textTheme.titleLarge),
        actions: [
          IconButton(
            icon: Icon(_searchExpanded ? Icons.close : Icons.search),
            tooltip: _searchExpanded ? 'Tutup pencarian' : 'Cari',
            onPressed: () {
              setState(() {
                _searchExpanded = !_searchExpanded;
                if (!_searchExpanded) {
                  _searchController.clear();
                  _applyFilter(_filter.copyWith(query: ''));
                }
              });
            },
          ),
        ],
      ),
      body: Column(
        children: [
          // Filter chips
          SizedBox(
            height: 48,
            child: ListView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              children: [
                ..._categories.map((cat) {
                  final isAll = cat.isEmpty;
                  final label = isAll ? 'Semua Kategori' : cat;
                  final selected = isAll
                      ? _filter.category.isEmpty
                      : _filter.category == cat;
                  return Padding(
                    padding: const EdgeInsets.only(right: 8),
                    child: FilterChip(
                      label: Text(label),
                      selected: selected,
                      onSelected: (_) => _applyFilter(
                        _filter.copyWith(category: isAll ? '' : cat),
                      ),
                    ),
                  );
                }),
                FilterChip(
                  avatar: const Icon(Icons.access_time, size: 16),
                  label: const Text('Buka sekarang'),
                  selected: _filter.openNow,
                  onSelected: (v) => _applyFilter(_filter.copyWith(openNow: v)),
                ),
              ],
            ),
          ),
          // Branch list
          Expanded(
            child: async.when(
              loading: () => _SkeletonList(),
              error: (err, _) => ErrorView(
                message:
                    'Gagal memuat cabang. Periksa koneksi internetmu dan coba lagi.',
                onRetry: () => ref.invalidate(branchListNotifierProvider),
              ),
              data: (state) {
                final items = state.items;
                if (items.isEmpty && !state.isLoading) {
                  return _EmptyBranchState(
                    hasFilter:
                        _filter.query.isNotEmpty ||
                        _filter.category.isNotEmpty ||
                        _filter.openNow,
                    onClearFilter: () =>
                        _applyFilter(const BranchFilterState()),
                  );
                }
                return RefreshIndicator(
                  onRefresh: () =>
                      ref.read(branchListNotifierProvider.notifier).refresh(),
                  child: ListView.builder(
                    controller: _scrollController,
                    padding: const EdgeInsets.only(top: 8, bottom: 24),
                    itemCount: items.length + (state.hasMore ? 1 : 0),
                    itemBuilder: (context, index) {
                      if (index == items.length) {
                        return const Padding(
                          padding: EdgeInsets.symmetric(vertical: 24),
                          child: Center(
                            child: CircularProgressIndicator.adaptive(),
                          ),
                        );
                      }
                      return BranchCard(branch: items[index]);
                    },
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _EmptyBranchState extends StatelessWidget {
  const _EmptyBranchState({
    required this.hasFilter,
    required this.onClearFilter,
  });

  final bool hasFilter;
  final VoidCallback onClearFilter;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              hasFilter ? Icons.search_off : Icons.store_outlined,
              size: 56,
              color: theme.colorScheme.onSurfaceVariant,
            ),
            const SizedBox(height: 16),
            Text(
              hasFilter ? 'Tidak ditemukan' : 'Belum ada cabang',
              style: theme.textTheme.titleMedium,
            ),
            const SizedBox(height: 8),
            Text(
              hasFilter
                  ? 'Coba kata kunci atau filter yang berbeda.'
                  : 'Kami sedang berkembang. Cek lagi nanti!',
              style: theme.textTheme.bodyMedium,
              textAlign: TextAlign.center,
            ),
            if (hasFilter) ...[
              const SizedBox(height: 16),
              OutlinedButton(
                onPressed: onClearFilter,
                child: const Text('Hapus filter'),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

/// Skeleton loading list — 6 card placeholder.
class _SkeletonList extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: const EdgeInsets.only(top: 8),
      itemCount: 6,
      itemBuilder: (_, __) => const _SkeletonCard(),
    );
  }
}

class _SkeletonCard extends StatelessWidget {
  const _SkeletonCard();

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            Container(
              width: 96,
              height: 96,
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.surfaceContainerHighest,
                borderRadius: BorderRadius.circular(8),
              ),
            ),
            const SizedBox(width: 12),
            const Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _SkeletonBox(width: 80, height: 12),
                  SizedBox(height: 6),
                  _SkeletonBox(width: double.infinity, height: 16),
                  SizedBox(height: 6),
                  _SkeletonBox(width: 120, height: 12),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SkeletonBox extends StatelessWidget {
  const _SkeletonBox({required this.width, required this.height});
  final double width;
  final double height;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: width,
      height: height,
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(4),
      ),
    );
  }
}
