// Utilitas format mata uang Rupiah.
// Contoh: formatRupiah(150000) → "Rp 150.000"

final class CurrencyFormatter {
  CurrencyFormatter._();

  /// Format harga dalam IDR (integer) menjadi string Rupiah.
  /// [amount] dalam satuan rupiah penuh (bukan sen).
  static String formatRupiah(int amount) {
    final str = amount.toString();
    final buffer = StringBuffer();

    // Sisipkan titik setiap 3 digit dari kanan
    final len = str.length;
    for (var i = 0; i < len; i++) {
      if (i > 0 && (len - i) % 3 == 0) {
        buffer.write('.');
      }
      buffer.write(str[i]);
    }

    return 'Rp ${buffer.toString()}';
  }
}
