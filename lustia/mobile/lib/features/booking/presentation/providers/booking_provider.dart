// Provider wizard booking — state multi-step form booking.

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../data/availability_model.dart';
import '../../data/booking_model.dart';
import '../../data/booking_repository.dart';

part 'booking_provider.g.dart';

@riverpod
class BookingWizard extends _$BookingWizard {
  @override
  BookingWizardState build(String branchId) {
    return BookingWizardState(branchId: branchId);
  }

  void selectService(String serviceId) {
    state = state.copyWith(
      selectedService: serviceId,
      selectedAddonIds: [],
      selectedDate: null,
      selectedSlot: null,
      clearTherapist: true,
      clearRoom: true,
    );
  }

  void toggleAddon(String addonId) {
    final current = List<String>.from(state.selectedAddonIds);
    if (current.contains(addonId)) {
      current.remove(addonId);
    } else {
      current.add(addonId);
    }
    state = state.copyWith(selectedAddonIds: current);
  }

  void selectDate(DateTime date) {
    state = state.copyWith(selectedDate: date, selectedSlot: null);
  }

  void selectSlot(AvailabilitySlot slot) {
    state = state.copyWith(
      selectedSlot: slot,
      clearTherapist: true,
      clearRoom: true,
    );
  }

  void selectTherapist(String? therapistId) {
    if (therapistId == null) {
      state = state.copyWith(clearTherapist: true);
    } else {
      state = state.copyWith(selectedTherapistId: therapistId);
    }
  }

  void selectRoom(String? roomId) {
    if (roomId == null) {
      state = state.copyWith(clearRoom: true);
    } else {
      state = state.copyWith(selectedRoomId: roomId);
    }
  }

  void updateCustomerInfo({
    required String name,
    required String phone,
    required String email,
  }) {
    state = state.copyWith(
      customerName: name,
      customerPhone: phone,
      customerEmail: email,
    );
  }

  void reset() {
    state = BookingWizardState(branchId: state.branchId);
  }
}

/// Provider untuk submit booking — ADR 0015 §2.8.
/// Hanya membuat booking; tidak lagi memanggil dummy webhook.
/// Setelah submit berhasil, PaymentScreen menangani polling status.
@riverpod
class BookingSubmit extends _$BookingSubmit {
  @override
  AsyncValue<CreateBookingResponse?> build() => const AsyncData(null);

  Future<CreateBookingResponse?> submit(BookingWizardState wizard) async {
    state = const AsyncLoading();

    final repo = ref.read(bookingRepositoryProvider);
    final request = CreateBookingRequest(
      branchId: wizard.branchId,
      serviceId: wizard.selectedService!,
      scheduledStart: wizard.selectedSlot!.start,
      customerName: wizard.customerName,
      customerPhone: wizard.customerPhone,
      customerEmail: wizard.customerEmail,
      addonIds: wizard.selectedAddonIds,
      roomId: wizard.selectedRoomId,
      therapistId: wizard.selectedTherapistId,
    );

    final result = await AsyncValue.guard(() => repo.createBooking(request));

    state = result;
    return result.valueOrNull;
  }
}

/// Provider polling status pembayaran — ADR 0015 §2.7.
///
/// Emit stream [PaymentStatusResponse] setiap 5 detik.
/// Stream berhenti secara otomatis saat status terminal
/// (paid / expired / failed) atau saat provider di-dispose.
@riverpod
Stream<PaymentStatusResponse> paymentStatus(Ref ref, String bookingCode) {
  final repo = ref.read(bookingRepositoryProvider);
  return Stream.periodic(const Duration(seconds: 5))
      .asyncMap((_) => repo.getPaymentStatus(bookingCode))
      .takeWhile((status) => !status.isTerminal);
}
