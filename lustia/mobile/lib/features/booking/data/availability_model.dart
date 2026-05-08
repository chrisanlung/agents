// Model slot ketersediaan — dipetakan dari response
// GET /api/v1/public/branches/:id/availability?service_id=&date=[&therapist_id=]
// API Contract §14.1.2 — therapist_available and available_room_ids added.

/// Satu slot waktu yang tersedia.
final class AvailabilitySlot {
  const AvailabilitySlot({
    required this.start,
    required this.end,
    required this.therapistsAvailableCount,
    required this.roomsAvailableCount,
    required this.availableRoomIds,
    this.therapistAvailable,
  });

  final String start; // ISO-8601 UTC
  final String end; // ISO-8601 UTC
  final int therapistsAvailableCount;
  final int roomsAvailableCount;

  /// UUIDs of rooms not booked at this slot. Always present (can be empty).
  /// Use this to grey-out specific rooms after customer picks a slot.
  final List<String> availableRoomIds;

  /// Non-null only when the availability request included a ?therapist_id= param.
  /// true  = therapist is free at this slot (no conflict + schedule covers it).
  /// false = therapist is booked or their schedule does not cover this slot.
  /// null  = no therapist filter was applied — no per-therapist info available.
  final bool? therapistAvailable;

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
      availableRoomIds:
          (json['available_room_ids'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
      // Field is absent when no therapist_id was supplied — keep null, not false.
      therapistAvailable: json.containsKey('therapist_available')
          ? (json['therapist_available'] as bool?)
          : null,
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
