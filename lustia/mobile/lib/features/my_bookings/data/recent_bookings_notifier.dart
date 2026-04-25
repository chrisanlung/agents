// RecentBookingsNotifier — Notifier untuk daftar booking terakhir dari local storage.

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/storage/recent_bookings_storage.dart';

part 'recent_bookings_notifier.g.dart';

@riverpod
class RecentBookings extends _$RecentBookings {
  @override
  Future<List<RecentBooking>> build() async {
    final storage = RecentBookingsStorage();
    final all = await storage.getAll();
    // Newest first
    return all.reversed.toList();
  }

  Future<void> add(RecentBooking booking) async {
    final storage = RecentBookingsStorage();
    await storage.add(booking);
    // Refresh state
    ref.invalidateSelf();
  }
}
