// Unit test untuk CurrencyFormatter.

import 'package:flutter_test/flutter_test.dart';
import 'package:lustia_mobile/shared/utils/currency_formatter.dart';

void main() {
  group('CurrencyFormatter', () {
    test('format 150000 menjadi "Rp 150.000"', () {
      expect(CurrencyFormatter.formatRupiah(150000), 'Rp 150.000');
    });

    test('format 1000000 menjadi "Rp 1.000.000"', () {
      expect(CurrencyFormatter.formatRupiah(1000000), 'Rp 1.000.000');
    });

    test('format 500 menjadi "Rp 500"', () {
      expect(CurrencyFormatter.formatRupiah(500), 'Rp 500');
    });
  });
}
