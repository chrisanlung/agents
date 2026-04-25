// RetryInterceptor — 1 kali retry otomatis untuk error jaringan atau 5xx.
// Request POST booking (non-idempotent) ditandai options.extra['noRetry'] = true
// untuk mencegah double-booking.

import 'package:dio/dio.dart';

class RetryInterceptor extends Interceptor {
  RetryInterceptor({required this.dio});

  final Dio dio;
  static const int _maxRetries = 1;
  static const Duration _retryDelay = Duration(milliseconds: 500);

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final options = err.requestOptions;

    // Jangan retry jika ditandai noRetry (POST booking)
    if (options.extra['noRetry'] == true) {
      return handler.next(err);
    }

    final retryCount = (options.extra['retryCount'] as int?) ?? 0;

    final shouldRetry =
        retryCount < _maxRetries &&
        (err.type == DioExceptionType.connectionError ||
            err.type == DioExceptionType.receiveTimeout ||
            err.type == DioExceptionType.connectionTimeout ||
            (err.response?.statusCode != null &&
                err.response!.statusCode! >= 500));

    if (!shouldRetry) {
      return handler.next(err);
    }

    await Future<void>.delayed(_retryDelay);

    options.extra['retryCount'] = retryCount + 1;

    try {
      final response = await dio.fetch<dynamic>(options);
      handler.resolve(response);
    } on DioException catch (e) {
      handler.next(e);
    }
  }
}
