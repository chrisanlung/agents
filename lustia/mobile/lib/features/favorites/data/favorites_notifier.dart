// FavoritesNotifier — Notifier membungkus FavoritesStorage.

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/storage/favorites_storage.dart';

part 'favorites_notifier.g.dart';

@riverpod
class Favorites extends _$Favorites {
  @override
  Future<List<String>> build() async {
    final storage = FavoritesStorage();
    return storage.getAll();
  }

  Future<void> toggle(String branchId) async {
    final storage = FavoritesStorage();
    final current = state.value ?? [];
    if (current.contains(branchId)) {
      await storage.remove(branchId);
      state = AsyncData(List<String>.from(current)..remove(branchId));
    } else {
      await storage.add(branchId);
      state = AsyncData([...current, branchId]);
    }
  }

  bool isFavorite(String branchId) {
    return state.value?.contains(branchId) ?? false;
  }

  Future<void> clearAll() async {
    final storage = FavoritesStorage();
    await storage.clear();
    state = const AsyncData([]);
  }
}
