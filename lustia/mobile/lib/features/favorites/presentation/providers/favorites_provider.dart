// Provider favorit — re-export FavoritesNotifier agar fitur bisa diakses
// dari manapun tanpa import path panjang.

// FavoritesNotifier sudah diekspos sebagai @riverpod di
// features/favorites/data/favorites_notifier.dart.
// File ini hanya sebagai barrel export agar konsistensi import di screen.

export '../../../favorites/data/favorites_notifier.dart';
