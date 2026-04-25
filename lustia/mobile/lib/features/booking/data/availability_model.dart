// Model slot ketersediaan — dipetakan dari response
// GET /api/v1/public/branches/:id/availability?service_id=&date=

/// Satu slot waktu yang tersedia.
final class AvailabilitySlot {
  const AvailabilitySlot({
    required this.start,
    required this.end,
    required this.therapistsAvailableCount,
    required this.roomsAvailableCount,
  });

  final String start; // ISO-8601 UTC
  final String end; // ISO-8601 UTC
  final int therapistsAvailableCount;
  final int roomsAvailableCount;

  bool get isAvailable =>
      therapistsAvailableCount > 0 && roomsAvailableCount > 0;

  factory AvailabilitySlot.fromJson(Map<String, dynamic> json) {
    return AvailabilitySlot(
      start: json['start'] as String,
      end: json['end'] as String,
      therapistsAvailableCount:
          (json['therapists_available_count'] as num?)?.toInt() ?? 0,
      roomsAvailableCount:
          (json['rooms_available_count'] as num?)?.toInt() ?? 0,
    );
  }
}

/// Response penuh dari availability endpoint.
final class AvailabilityResponse {
  const AvailabilityResponse({
    required this.branchId,
    required this.serviceId,
    required this.date,
    required this.slots,
  });

  final String branchId;
  final String serviceId;
  final String date;
  final List<AvailabilitySlot> slots;

  factory AvailabilityResponse.fromJson(Map<String, dynamic> json) {
    return AvailabilityResponse(
      branchId: json['branch_id'] as String,
      serviceId: json['service_id'] as String,
      date: json['date'] as String,
      slots: (json['slots'] as List<dynamic>)
          .map((e) => AvailabilitySlot.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }
}
