// Factory Dio singleton — semua HTTP call di app menggunakan instance ini.
// Interceptor stack: auth → retry → error transform.
// ignore_for_file: deprecated_member_use_from_same_package

import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../config/app_config.dart';
import 'interceptors/auth_interceptor.dart';
import 'interceptors/error_interceptor.dart';
import 'interceptors/retry_interceptor.dart';

part 'dio_client.g.dart';

@riverpod
Dio dioClient(DioClientRef ref) {
  final dio = Dio(
    BaseOptions(
      baseUrl: AppConfig.baseUrl,
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 15),
      headers: const {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
        // Bypass ngrok-free.dev browser interstitial. ngrok serves an HTML
        // splash for any browser User-Agent unless this header is present;
        // without it Flutter web receives HTML instead of JSON. Harmless on
        // production / direct backend hits — the header is simply ignored.
        'ngrok-skip-browser-warning': 'true',
      },
    ),
  );

  dio.interceptors.addAll([
    AuthInterceptor(),
    RetryInterceptor(dio: dio),
    ErrorInterceptor(),
  ]);

  return dio;
}
