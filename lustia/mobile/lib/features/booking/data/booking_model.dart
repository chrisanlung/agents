// Model booking — dipetakan dari response POST /api/v1/public/bookings
// dan GET /api/v1/public/bookings/:code.

import 'availability_model.dart';

/// Add-on yang dipilih dalam booking (snapshot harga saat booking).
final class BookingAddonItem {
  const BookingAddonItem({
    required this.addonId,
    required this.name,
    required this.priceIdr,
  });

  final String addonId;
  final String name;
  final int priceIdr;

  factory BookingAddonItem.fromJson(Map<String, dynamic> json) {
    return BookingAddonItem(
      addonId: json['addon_id'] as String,
      name: json['name'] as String,
      priceIdr: (json['price_idr'] as num?)?.toInt() ?? 0,
    );
  }
}

/// Response dari POST /api/v1/public/bookings (201 Created).
/// ADR 0015 §2.8 — Phase 6 shape: QR-based payment fields.
final class CreateBookingResponse {
  const CreateBookingResponse({
    required this.id,
    required this.code,
    required this.status,
    required this.totalPriceIdr,
    required this.scheduledStart,
    required this.scheduledEnd,
    this.qrString,
    this.qrImageUrl,
    this.qrExpiresAt,
    this.paymentReference,
    this.addons = const [],
  });

  final String id;
  final String code;
  final String status;
  final int totalPriceIdr;
  final String scheduledStart;
  final String scheduledEnd;

  /// QRIS string untuk di-render via qr_flutter.
  final String? qrString;

  /// URL gambar QR dari provider (opsional — fallback ke qr_flutter).
  final String? qrImageUrl;

  /// Batas waktu QR dalam ISO-8601; null jika provider tidak mengisi.
  final String? qrExpiresAt;

  /// Referensi transaksi di sisi provider (misal: ipaymu_trx_xxx).
  final String? paymentReference;
  final List<BookingAddonItem> addons;

  factory CreateBookingResponse.fromJson(Map<String, dynamic> json) {
    return CreateBookingResponse(
      id: json['id'] as String,
      code: json['code'] as String,
      status: json['status'] as String,
      totalPriceIdr: (json['total_price_idr'] as num?)?.toInt() ?? 0,
      scheduledStart: json['scheduled_start'] as String,
      scheduledEnd: json['scheduled_end'] as String,
      qrString: json['qr_string'] as String?,
      qrImageUrl: json['qr_image_url'] as String?,
      qrExpiresAt: json['qr_expires_at'] as String?,
      paymentReference: json['payment_reference'] as String?,
      addons:
          (json['addons'] as List<dynamic>?)
              ?.map((e) => BookingAddonItem.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }
}

/// Response dari GET /api/v1/public/bookings/:code/payment-status.
/// ADR 0015 §2.7 — polling endpoint.
final class PaymentStatusResponse {
  const PaymentStatusResponse({
    required this.status,
    this.paidAt,
    this.qrExpiresAt,
  });

  /// 'awaiting' | 'paid' | 'expired' | 'failed'
  final String status;
  final String? paidAt;
  final String? qrExpiresAt;

  bool get isTerminal =>
      status == 'paid' || status == 'expired' || status == 'failed';

  factory PaymentStatusResponse.fromJson(Map<String, dynamic> json) {
    return PaymentStatusResponse(
      status: json['status'] as String,
      paidAt: json['paid_at'] as String?,
      qrExpiresAt: json['qr_expires_at'] as String?,
    );
  }
}

/// Response dari GET /api/v1/public/bookings/:code.
final class PublicBookingDetail {
  const PublicBookingDetail({
    required this.code,
    required this.branchName,
    required this.serviceName,
    required this.scheduledStart,
    required this.scheduledEnd,
    required this.status,
    required this.totalPriceIdr,
    this.customerName,
    this.addons = const [],
  });

  final String code;
  final String branchName;
  final String serviceName;
  final String scheduledStart;
  final String scheduledEnd;
  final String status;
  final int totalPriceIdr;
  final String? customerName;
  final List<BookingAddonItem> addons;

  factory PublicBookingDetail.fromJson(Map<String, dynamic> json) {
    return PublicBookingDetail(
      code: json['code'] as String,
      branchName: json['branch_name'] as String,
      serviceName: json['service_name'] as String,
      scheduledStart: json['scheduled_start'] as String,
      scheduledEnd: json['scheduled_end'] as String,
      status: json['status'] as String,
      totalPriceIdr: (json['total_price_idr'] as num?)?.toInt() ?? 0,
      customerName: json['customer_name'] as String?,
      addons:
          (json['addons'] as List<dynamic>?)
              ?.map((e) => BookingAddonItem.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }
}

/// Request body untuk POST /api/v1/public/bookings.
final class CreateBookingRequest {
  const CreateBookingRequest({
    required this.branchId,
    required this.serviceId,
    required this.scheduledStart,
    required this.customerName,
    required this.customerPhone,
    required this.customerEmail,
    this.addonIds = const [],
    this.roomId,
    this.therapistId,
  });

  final String branchId;
  final String serviceId;
  final String scheduledStart; // ISO-8601
  final String customerName;
  final String customerPhone;
  final String customerEmail;
  final List<String> addonIds;
  final String? roomId;
  final String? therapistId;

  Map<String, dynamic> toJson() {
    final map = <String, dynamic>{
      'branch_id': branchId,
      'service_id': serviceId,
      'scheduled_start': scheduledStart,
      'customer_name': customerName,
      'customer_phone': customerPhone,
      'customer_email': customerEmail,
      'addon_ids': addonIds,
    };
    if (roomId != null) map['room_id'] = roomId;
    if (therapistId != null) map['therapist_id'] = therapistId;
    return map;
  }
}

/// State wizard booking — menyimpan pilihan user di setiap langkah.
///
/// [therapistConfirmed] distinguishes "user explicitly tapped Pilih Otomatis/Pilih X"
/// (true) from "page just loaded, nothing picked yet" (false even when
/// selectedTherapistId is null). The new BookingSelectionScreen relies on this
/// to keep Section ③ locked until an explicit therapist selection is made.
final class BookingWizardState {
  const BookingWizardState({
    required this.branchId,
    this.selectedService,
    this.selectedAddonIds = const [],
    this.selectedDate,
    this.selectedSlot,
    this.selectedTherapistId,
    this.therapistConfirmed = false,
    this.selectedRoomId,
    this.customerName = '',
    this.customerPhone = '',
    this.customerEmail = '',
  });

  final String branchId;
  final String? selectedService; // service ID
  final List<String> selectedAddonIds;
  final DateTime? selectedDate;
  final AvailabilitySlot? selectedSlot;

  /// null = "Otomatis" (auto-assign) when [therapistConfirmed] is true.
  /// null with [therapistConfirmed] false = not yet chosen.
  final String? selectedTherapistId;

  /// true once the user has explicitly tapped "Pilih Otomatis" or "Pilih [Name]".
  final bool therapistConfirmed;
  final String? selectedRoomId; // null = auto
  final String customerName;
  final String customerPhone;
  final String customerEmail;

  BookingWizardState copyWith({
    String? selectedService,
    List<String>? selectedAddonIds,
    DateTime? selectedDate,
    AvailabilitySlot? selectedSlot,
    String? selectedTherapistId,
    bool clearTherapist = false,
    bool? therapistConfirmed,
    String? selectedRoomId,
    bool clearRoom = false,
    String? customerName,
    String? customerPhone,
    String? customerEmail,
  }) {
    return BookingWizardState(
      branchId: branchId,
      selectedService: selectedService ?? this.selectedService,
      selectedAddonIds: selectedAddonIds ?? this.selectedAddonIds,
      selectedDate: selectedDate ?? this.selectedDate,
      selectedSlot: selectedSlot ?? this.selectedSlot,
      selectedTherapistId: clearTherapist
          ? null
          : (selectedTherapistId ?? this.selectedTherapistId),
      therapistConfirmed: clearTherapist
          ? false
          : (therapistConfirmed ?? this.therapistConfirmed),
      selectedRoomId: clearRoom
          ? null
          : (selectedRoomId ?? this.selectedRoomId),
      customerName: customerName ?? this.customerName,
      customerPhone: customerPhone ?? this.customerPhone,
      customerEmail: customerEmail ?? this.customerEmail,
    );
  }
}
