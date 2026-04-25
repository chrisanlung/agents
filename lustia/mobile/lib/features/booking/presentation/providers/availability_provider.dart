// Provider slot ketersediaan — dipanggil saat user memilih tanggal pada wizard.

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../data/availability_model.dart';
import '../../data/booking_repository.dart';

part 'availability_provider.g.dart';

@riverpod
Future<List<AvailabilitySlot>> availability(
  AvailabilityRef ref, {
  required String branchId,
  required String serviceId,
  required String date, // YYYY-MM-DD
}) async {
  final repo = ref.watch(bookingRepositoryProvider);
  final response = await repo.getAvailability(
    branchId: branchId,
    serviceId: serviceId,
    date: date,
  );
  return response.slots;
}
