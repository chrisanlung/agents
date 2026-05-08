// Provider slot ketersediaan — dipanggil saat user memilih tanggal pada wizard.
// API Contract §14.1.2 — optional therapist_id filter now supported.

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
  String? therapistId, // optional — omit for aggregate availability
}) async {
  final repo = ref.watch(bookingRepositoryProvider);
  final response = await repo.getAvailability(
    branchId: branchId,
    serviceId: serviceId,
    date: date,
    therapistId: therapistId,
  );
  return response.slots;
}
