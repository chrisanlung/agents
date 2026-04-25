// AuthInterceptor — menyuntikkan token Bearer ke setiap request.
// Phase 5: tidak ada token (guest-only). Interceptor ini adalah placeholder
// untuk Phase 6+ ketika customer login ditambahkan.
// TODO Phase 6: baca token dari flutter_secure_storage dan inject di sini.

import 'package:dio/dio.dart';

class AuthInterceptor extends Interceptor {
  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    // Phase 5: tidak ada token, langsung lanjutkan.
    // Phase 6+: inject 'Authorization: Bearer <token>' di sini.
    handler.next(options);
  }
}
