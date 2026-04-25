// Provider daftar cabang — paginated, filter, sort by distance.

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../../core/location/location_service.dart';
import '../../data/branch_model.dart';
import '../../data/branch_repository.dart';

part 'branch_list_provider.g.dart';

/// State filter daftar cabang.
final class BranchFilterState {
  const BranchFilterState({
    this.query = '',
    this.category = '',
    this.openNow = false,
  });

  final String query;
  final String category;
  final bool openNow;

  BranchFilterState copyWith({
    String? query,
    String? category,
    bool? openNow,
  }) => BranchFilterState(
    query: query ?? this.query,
    category: category ?? this.category,
    openNow: openNow ?? this.openNow,
  );
}

/// State halaman daftar cabang (paginated).
final class BranchListState {
  const BranchListState({
    required this.items,
    required this.isLoading,
    required this.hasMore,
    required this.currentPage,
    this.error,
  });

  final List<BranchSummary> items;
  final bool isLoading;
  final bool hasMore;
  final int currentPage;
  final Object? error;
}

@riverpod
class BranchListNotifier extends _$BranchListNotifier {
  static const int _pageSize = 10;

  @override
  Future<BranchListState> build() async {
    return _fetchPage(page: 1, reset: true);
  }

  Future<BranchListState> _fetchPage({
    required int page,
    required bool reset,
    BranchFilterState? filter,
  }) async {
    final currentFilter = filter ?? const BranchFilterState();
    final repo = ref.read(branchRepositoryProvider);
    final position = await ref.read(currentLocationProvider.future);

    final params = BranchListParams(
      page: page,
      limit: _pageSize,
      query: currentFilter.query.isEmpty ? null : currentFilter.query,
      lat: position?.latitude,
      lng: position?.longitude,
      category: currentFilter.category.isEmpty ? null : currentFilter.category,
      openNow: currentFilter.openNow ? true : null,
    );

    final response = await repo.listBranches(params);

    return BranchListState(
      items: reset
          ? response.data
          : [...(state.value?.items ?? []), ...response.data],
      isLoading: false,
      hasMore: response.hasMore,
      currentPage: page,
    );
  }

  /// Muat halaman berikutnya (infinite scroll).
  Future<void> loadNextPage() async {
    final current = state.value;
    if (current == null || current.isLoading || !current.hasMore) return;

    state = AsyncData(
      BranchListState(
        items: current.items,
        isLoading: true,
        hasMore: current.hasMore,
        currentPage: current.currentPage,
      ),
    );

    state = await AsyncValue.guard(
      () => _fetchPage(page: current.currentPage + 1, reset: false),
    );
  }

  /// Pull-to-refresh.
  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(() => _fetchPage(page: 1, reset: true));
  }

  /// Terapkan filter baru dan reset ke halaman 1.
  Future<void> applyFilter(BranchFilterState filter) async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => _fetchPage(page: 1, reset: true, filter: filter),
    );
  }
}
