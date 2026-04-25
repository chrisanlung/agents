// Model cabang — dipetakan dari response GET /api/v1/public/branches
// dan GET /api/v1/public/branches/:id (full detail).
// Menggunakan hand-written factory constructors karena freezed code-gen
// memerlukan build_runner di CI — model ini cukup sederhana untuk manual.

/// Model ringkas untuk daftar cabang (dari /public/branches).
final class BranchSummary {
  const BranchSummary({
    required this.id,
    required this.name,
    required this.tenantName,
    required this.city,
    required this.addressLine1,
    required this.contactPhone,
    required this.contactEmail,
    this.latitude,
    this.longitude,
    this.distanceMeters,
    this.photoUrl,
    this.categories = const [],
    this.operationalHours,
  });

  final String id;
  final String name;
  final String tenantName;
  final String city;
  final String addressLine1;
  final String contactPhone;
  final String contactEmail;
  final double? latitude;
  final double? longitude;
  final double? distanceMeters;
  final String? photoUrl;
  final List<String> categories;
  final Map<String, dynamic>? operationalHours;

  factory BranchSummary.fromJson(Map<String, dynamic> json) {
    return BranchSummary(
      id: json['id'] as String,
      name: json['name'] as String,
      tenantName: (json['tenant_name'] as String?) ?? '',
      city: (json['city'] as String?) ?? '',
      addressLine1: (json['address_line1'] as String?) ?? '',
      contactPhone: (json['contact_phone'] as String?) ?? '',
      contactEmail: (json['contact_email'] as String?) ?? '',
      latitude: (json['latitude'] as num?)?.toDouble(),
      longitude: (json['longitude'] as num?)?.toDouble(),
      distanceMeters: (json['distance_meters'] as num?)?.toDouble(),
      photoUrl: json['photo_url'] as String?,
      categories:
          (json['categories'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
      operationalHours: json['operational_hours'] as Map<String, dynamic>?,
    );
  }

  /// Singkatan alamat untuk card: "Kota" saja.
  String get shortAddress => city.isNotEmpty ? city : addressLine1;

  /// Jarak dalam km diformat "1.2 km".
  String? get distanceLabel {
    if (distanceMeters == null) return null;
    final km = distanceMeters! / 1000;
    return '${km.toStringAsFixed(1)} km';
  }
}

/// Service dalam daftar layanan cabang.
final class ServiceItem {
  const ServiceItem({
    required this.id,
    required this.name,
    required this.durationMinutes,
    required this.priceIdr,
    required this.category,
    this.description,
    this.addons = const [],
  });

  final String id;
  final String name;
  final int durationMinutes;
  final int priceIdr;
  final String category;
  final String? description;
  final List<AddonItem> addons;

  factory ServiceItem.fromJson(Map<String, dynamic> json) {
    return ServiceItem(
      id: json['id'] as String,
      name: json['name'] as String,
      durationMinutes: (json['duration_minutes'] as num?)?.toInt() ?? 60,
      priceIdr: (json['price_idr'] as num?)?.toInt() ?? 0,
      category: (json['category'] as String?) ?? '',
      description: json['description'] as String?,
      addons:
          (json['addons'] as List<dynamic>?)
              ?.map((e) => AddonItem.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }
}

/// Add-on untuk layanan.
final class AddonItem {
  const AddonItem({
    required this.id,
    required this.name,
    required this.priceIdr,
    this.description,
  });

  final String id;
  final String name;
  final int priceIdr;
  final String? description;

  factory AddonItem.fromJson(Map<String, dynamic> json) {
    return AddonItem(
      id: json['id'] as String,
      name: json['name'] as String,
      priceIdr: (json['price_idr'] as num?)?.toInt() ?? 0,
      description: json['description'] as String?,
    );
  }
}

/// Model terapis dalam cabang (public detail).
final class TherapistItem {
  const TherapistItem({
    required this.id,
    required this.fullName,
    this.photoUrl,
    this.heightCm,
    this.weightKg,
    this.build,
  });

  final String id;
  final String fullName;
  final String? photoUrl;
  final int? heightCm;
  final int? weightKg;
  final String? build;

  String get initials {
    final parts = fullName.trim().split(' ');
    if (parts.length >= 2) {
      return '${parts.first[0]}${parts.last[0]}'.toUpperCase();
    }
    return fullName.isNotEmpty ? fullName[0].toUpperCase() : '?';
  }

  factory TherapistItem.fromJson(Map<String, dynamic> json) {
    return TherapistItem(
      id: json['id'] as String,
      fullName: (json['full_name'] as String?) ?? '',
      photoUrl: json['photo_url'] as String?,
      heightCm: (json['height_cm'] as num?)?.toInt(),
      weightKg: (json['weight_kg'] as num?)?.toInt(),
      build: json['build'] as String?,
    );
  }
}

/// Model ruangan dalam cabang (public detail).
final class RoomItem {
  const RoomItem({
    required this.id,
    required this.name,
    required this.capacity,
    this.roomType,
    this.photoUrl,
  });

  final String id;
  final String name;
  final int capacity;
  final String? roomType;
  final String? photoUrl;

  factory RoomItem.fromJson(Map<String, dynamic> json) {
    return RoomItem(
      id: json['id'] as String,
      name: json['name'] as String,
      capacity: (json['capacity'] as num?)?.toInt() ?? 1,
      roomType: json['room_type'] as String?,
      photoUrl: json['photo_url'] as String?,
    );
  }
}

/// Detail penuh cabang dari /public/branches/:id.
final class BranchDetail {
  const BranchDetail({
    required this.id,
    required this.name,
    required this.tenantName,
    required this.city,
    required this.province,
    required this.addressLine1,
    required this.contactPhone,
    required this.contactEmail,
    required this.services,
    required this.therapists,
    required this.rooms,
    this.latitude,
    this.longitude,
    this.photoUrl,
    this.operationalHours,
  });

  final String id;
  final String name;
  final String tenantName;
  final String city;
  final String province;
  final String addressLine1;
  final String contactPhone;
  final String contactEmail;
  final List<ServiceItem> services;
  final List<TherapistItem> therapists;
  final List<RoomItem> rooms;
  final double? latitude;
  final double? longitude;
  final String? photoUrl;
  final Map<String, dynamic>? operationalHours;

  String get fullAddress {
    final parts = [
      addressLine1,
      city,
      province,
    ].where((s) => s.isNotEmpty).toList();
    return parts.join(', ');
  }

  /// Jam buka hari ini dari operational_hours JSON.
  /// Format operational_hours: {"1": {"open":"09:00","close":"21:00"}, ...}
  /// weekday: 1=Senin … 7=Minggu
  String get todayHours {
    if (operationalHours == null) return 'Lihat di cabang';
    final today = DateTime.now().weekday; // 1=Monday … 7=Sunday
    final key = today.toString();
    final dayData = operationalHours![key];
    if (dayData == null) return 'Tutup hari ini';
    if (dayData is Map) {
      final open = dayData['open'] ?? '';
      final close = dayData['close'] ?? '';
      if (open.isNotEmpty && close.isNotEmpty) return '$open – $close';
    }
    return 'Lihat di cabang';
  }

  bool get isOpenToday {
    if (operationalHours == null) return false;
    final today = DateTime.now().weekday.toString();
    return operationalHours!.containsKey(today);
  }

  /// Services grouped by category.
  Map<String, List<ServiceItem>> get servicesByCategory {
    final map = <String, List<ServiceItem>>{};
    for (final s in services) {
      (map[s.category] ??= []).add(s);
    }
    return map;
  }

  factory BranchDetail.fromJson(Map<String, dynamic> json) {
    return BranchDetail(
      id: json['id'] as String,
      name: json['name'] as String,
      tenantName: (json['tenant_name'] as String?) ?? '',
      city: (json['city'] as String?) ?? '',
      province: (json['province'] as String?) ?? '',
      addressLine1: (json['address_line1'] as String?) ?? '',
      contactPhone: (json['contact_phone'] as String?) ?? '',
      contactEmail: (json['contact_email'] as String?) ?? '',
      services:
          (json['services'] as List<dynamic>?)
              ?.map((e) => ServiceItem.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
      therapists:
          (json['therapists'] as List<dynamic>?)
              ?.map((e) => TherapistItem.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
      rooms:
          (json['rooms'] as List<dynamic>?)
              ?.map((e) => RoomItem.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
      latitude: (json['latitude'] as num?)?.toDouble(),
      longitude: (json['longitude'] as num?)?.toDouble(),
      photoUrl: json['photo_url'] as String?,
      operationalHours: json['operational_hours'] as Map<String, dynamic>?,
    );
  }
}

/// Paginated response untuk daftar cabang.
final class BranchListResponse {
  const BranchListResponse({
    required this.data,
    required this.page,
    required this.limit,
    required this.totalCount,
  });

  final List<BranchSummary> data;
  final int page;
  final int limit;
  final int totalCount;

  bool get hasMore => page * limit < totalCount;

  factory BranchListResponse.fromJson(Map<String, dynamic> json) {
    return BranchListResponse(
      data: (json['data'] as List<dynamic>)
          .map((e) => BranchSummary.fromJson(e as Map<String, dynamic>))
          .toList(),
      page: (json['page'] as num?)?.toInt() ?? 1,
      limit: (json['limit'] as num?)?.toInt() ?? 10,
      totalCount: (json['total_count'] as num?)?.toInt() ?? 0,
    );
  }
}
