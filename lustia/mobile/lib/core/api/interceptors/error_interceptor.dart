// ErrorInterceptor — mengubah DioException menjadi AppException yang terstruktur.
// Provider dan widget cukup handle AppException, bukan DioException mentah.

import 'package:dio/dio.dart';

import '../../exceptions/app_exception.dart';

class ErrorInterceptor extends Interceptor {
  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    final appException = _mapDioException(err);
    handler.next(
      err.copyWith(
        // Simpan AppException di extra agar provider bisa mengaksesnya
        requestOptions: err.requestOptions
          ..extra['appException'] = appException,
      ),
    );
  }

  AppException _mapDioException(DioException err) {
    final response = err.response;

    if (err.type == DioExceptionType.connectionError ||
        err.type == DioExceptionType.connectionTimeout ||
        err.type == DioExceptionType.receiveTimeout) {
      return const NetworkException(
        'Tidak dapat terhubung ke server. Periksa koneksi internet Anda.',
      );
    }

    if (response != null) {
      final statusCode = response.statusCode ?? 0;
      final data = response.data;

      // Ekstrak error code dari envelope API: {"error": {"code": "...", "message": "..."}}
      String? apiCode;
      String? apiMessage;
      if (data is Map<String, dynamic>) {
        final error = data['error'];
        if (error is Map<String, dynamic>) {
          apiCode = error['code'] as String?;
          apiMessage = error['message'] as String?;
        }
      }

      if (statusCode == 400) {
        return ValidationException(
          apiMessage ?? 'Data tidak valid.',
          code: apiCode,
        );
      }
      if (statusCode == 404) {
        return const NotFoundException('Data tidak ditemukan.');
      }
      if (statusCode == 409) {
        return ConflictException(apiMessage ?? 'Konflik data.', code: apiCode);
      }
      if (statusCode == 429) {
        return const RateLimitException(
          'Terlalu banyak permintaan. Coba lagi nanti.',
        );
      }
      if (statusCode >= 500) {
        return ServerException(
          apiMessage ?? 'Terjadi kesalahan server.',
          statusCode: statusCode,
        );
      }
    }

    return UnknownException(
      err.message ?? 'Terjadi kesalahan tidak diketahui.',
    );
  }
}
