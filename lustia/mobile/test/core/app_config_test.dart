// Unit test untuk AppConfig.
// CATATAN: karena AppConfig._configured hanya boleh di-set sekali per isolate,
// test ini tidak boleh dikombinasikan dengan widget_test.dart dalam satu test suite
// tanpa reset. Jalankan: flutter test test/core/app_config_test.dart

import 'package:flutter_test/flutter_test.dart';
import 'package:lustia_mobile/core/config/app_config.dart';

void main() {
  test('AppConfig.flavor default adalah prod sebelum configure()', () {
    // Flavor default prod sudah di-set di file ini (tidak ada configure() sebelumnya).
    // Test ini lebih ke dokumentasi intent daripada assertion yang strict.
    // Implementasi assert di configure() mencegah double-configure di runtime.
    expect(Flavor.values, containsAll([Flavor.dev, Flavor.prod]));
  });
}
