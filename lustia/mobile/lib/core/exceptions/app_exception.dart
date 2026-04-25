// Sealed class AppException — semua error domain direpresentasikan di sini.
// Provider mengekspos AsyncError(AppException) → widget menampilkan pesan yang tepat.

sealed class AppException implements Exception {
  const AppException(this.message);
  final String message;

  @override
  String toString() => '$runtimeType: $message';
}

/// Tidak ada koneksi internet atau timeout.
final class NetworkException extends AppException {
  const NetworkException(super.message);
}

/// HTTP 400 — validasi gagal.
final class ValidationException extends AppException {
  const ValidationException(super.message, {this.code});
  final String? code;
}

/// HTTP 404 — resource tidak ditemukan.
final class NotFoundException extends AppException {
  const NotFoundException(super.message);
}

/// HTTP 409 — konflik (misal: slot sudah dipesan).
final class ConflictException extends AppException {
  const ConflictException(super.message, {this.code});
  final String? code;
}

/// HTTP 429 — rate limit.
final class RateLimitException extends AppException {
  const RateLimitException(super.message);
}

/// HTTP 5xx — error server.
final class ServerException extends AppException {
  const ServerException(super.message, {this.statusCode});
  final int? statusCode;
}

/// Error tidak diketahui / tidak terpetakan.
final class UnknownException extends AppException {
  const UnknownException(super.message);
}
