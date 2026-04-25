// FavoritesStorage — wrapper SharedPreferences untuk menyimpan daftar ID cabang favorit.
// Format: List<String> (UUID cabang), maks 100 entri, entri lama dibuang saat overflow.

import 'package:shared_preferences/shared_preferences.dart';

class FavoritesStorage {
  static const String _key = 'favorites_branch_ids';
  static const int _maxEntries = 100;

  // TODO Phase 5: inject SharedPreferences via constructor untuk kemudahan testing.
  Future<List<String>> getAll() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getStringList(_key) ?? [];
  }

  Future<void> add(String branchId) async {
    final prefs = await SharedPreferences.getInstance();
    final list = List<String>.from(prefs.getStringList(_key) ?? []);
    if (list.contains(branchId)) return;
    list.add(branchId);
    if (list.length > _maxEntries) {
      list.removeRange(0, list.length - _maxEntries);
    }
    await prefs.setStringList(_key, list);
  }

  Future<void> remove(String branchId) async {
    final prefs = await SharedPreferences.getInstance();
    final list = List<String>.from(prefs.getStringList(_key) ?? []);
    list.remove(branchId);
    await prefs.setStringList(_key, list);
  }

  Future<bool> contains(String branchId) async {
    final list = await getAll();
    return list.contains(branchId);
  }

  Future<void> clear() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_key);
  }
}
