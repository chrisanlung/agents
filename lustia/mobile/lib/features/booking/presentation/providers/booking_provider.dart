// Provider wizard booking — state multi-step form booking.

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

/// Provider untuk submit booking dan dummy payment.
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

    final result = await AsyncValue.guard(() async {
      final response = await repo.createBooking(request);
      // Dummy webhook — simulates Midtrans callback immediately
      await repo.sendDummyWebhook(response.code, response.totalPriceIdr);
      return response;
    });

    state = result;
    return result.valueOrNull;
  }
}
