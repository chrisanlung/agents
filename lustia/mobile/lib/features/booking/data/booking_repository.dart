// Repository booking — interface + implementasi Dio.

import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/api/dio_client.dart';
import 'availability_model.dart';
import 'booking_model.dart';

part 'booking_repository.g.dart';

abstract interface class BookingRepository {
  Future<AvailabilityResponse> getAvailability({
    required String branchId,
    required String serviceId,
    required String date, // YYYY-MM-DD
  });

  Future<CreateBookingResponse> createBooking(CreateBookingRequest request);

  /// Polling — ADR 0015 §2.7.
  /// Rate limited 12/min per IP; caller responsible for 5-second interval.
  Future<PaymentStatusResponse> getPaymentStatus(String bookingCode);

  /// DEV-only short-circuit — ADR 0015 §2.7.
  /// Triggers webhook handler internally; body: {code}.
  Future<void> triggerDummyPayment(String bookingCode);

  Future<PublicBookingDetail> getBookingByCode(String code);
}

class BookingRepositoryImpl implements BookingRepository {
  BookingRepositoryImpl(this._dio);
  final Dio _dio;

  @override
  Future<AvailabilityResponse> getAvailability({
    required String branchId,
    required String serviceId,
    required String date,
  }) async {
    final response = await _dio.get<Map<String, dynamic>>(
      '/api/v1/public/branches/$branchId/availability',
      queryParameters: {'service_id': serviceId, 'date': date},
    );
    return AvailabilityResponse.fromJson(response.data!);
  }

  @override
  Future<CreateBookingResponse> createBooking(
    CreateBookingRequest request,
  ) async {
    final response = await _dio.post<Map<String, dynamic>>(
      '/api/v1/public/bookings',
      data: request.toJson(),
      options: Options(extra: {'noRetry': true}),
    );
    return CreateBookingResponse.fromJson(response.data!);
  }

  @override
  Future<PaymentStatusResponse> getPaymentStatus(String bookingCode) async {
    final response = await _dio.get<Map<String, dynamic>>(
      '/api/v1/public/bookings/$bookingCode/payment-status',
    );
    return PaymentStatusResponse.fromJson(response.data!);
  }

  @override
  Future<void> triggerDummyPayment(String bookingCode) async {
    await _dio.post<dynamic>(
      '/api/v1/public/payments/dummy-trigger',
      data: {'code': bookingCode},
      options: Options(extra: {'noRetry': true}),
    );
  }

  @override
  Future<PublicBookingDetail> getBookingByCode(String code) async {
    final response = await _dio.get<Map<String, dynamic>>(
      '/api/v1/public/bookings/$code',
    );
    return PublicBookingDetail.fromJson(response.data!);
  }
}

@riverpod
BookingRepository bookingRepository(BookingRepositoryRef ref) {
  final dio = ref.watch(dioClientProvider);
  return BookingRepositoryImpl(dio);
}
