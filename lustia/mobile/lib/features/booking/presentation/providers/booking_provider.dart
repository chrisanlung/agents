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
    // Date change: clear slot + room but keep therapist preference (spec §3.①).
    state = state.copyWith(
      selectedDate: date,
      selectedSlot: null,
      clearRoom: true,
    );
  }

  void selectSlot(AvailabilitySlot slot) {
    state = state.copyWith(selectedSlot: slot, clearRoom: true);
  }

  /// Legacy wizard method — kept for backwards compatibility.
  void selectTherapist(String? therapistId) {
    if (therapistId == null) {
      state = state.copyWith(clearTherapist: true);
    } else {
      state = state.copyWith(selectedTherapistId: therapistId);
    }
  }

  /// Used by BookingSelectionScreen — records an explicit "Pilih" button tap.
  /// [therapistId] == null means the user tapped "Pilih Otomatis".
  /// Clears slot and room because a different therapist means different availability.
  void confirmTherapist(String? therapistId) {
    state = state.copyWith(
      selectedTherapistId: therapistId,
      therapistConfirmed: true,
      selectedSlot: null,
      clearRoom: true,
    );
    // If therapistId is null we use clearTherapist=false because we want
    // selectedTherapistId = null AND therapistConfirmed = true simultaneously.
    // The copyWith already sets selectedTherapistId = therapistId (null),
    // but clearTherapist=false keeps therapistConfirmed from being reset.
  }

  void selectRoom(String? roomId) {
    if (roomId == null) {
      state = state.copyWith(clearRoom: true);
    } else {
      state = state.copyWith(selectedRoomId: roomId);
    }
  }

  /// Set payment channel: 'qris' (default) or 'va_<bank>'.
  void selectPaymentChannel(String channel) {
    state = state.copyWith(paymentChannel: channel);
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

  void setCustomerName(String name) {
    state = state.copyWith(customerName: name);
  }

  void setCustomerPhone(String phone) {
    state = state.copyWith(customerPhone: phone);
  }

  void setCustomerEmail(String email) {
    state = state.copyWith(customerEmail: email);
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
      paymentChannel: wizard.paymentChannel,
    );

    final result = await AsyncValue.guard(() => repo.createBooking(request));

    state = result;
    return result.valueOrNull;
  }
}

/// Provider polling status pembayaran — ADR 0015 §2.7.
///
/// Emit pertama kali sekarang juga (tanpa tunggu 5 detik), lalu setiap 5 detik.
/// Stream emits status terminal (paid / expired / failed) lalu berhenti — UI
/// listener perlu lihat nilai terminal supaya bisa navigate ke konfirmasi.
/// (Bug fix: `takeWhile(!isTerminal)` membuang elemen terminal sehingga UI
/// tidak pernah lihat 'paid'.)
@riverpod
Stream<PaymentStatusResponse> paymentStatus(
  Ref ref,
  String bookingCode,
) async* {
  final repo = ref.read(bookingRepositoryProvider);
  while (true) {
    final status = await repo.getPaymentStatus(bookingCode);
    yield status;
    if (status.isTerminal) break;
    await Future<void>.delayed(const Duration(seconds: 5));
  }
}
