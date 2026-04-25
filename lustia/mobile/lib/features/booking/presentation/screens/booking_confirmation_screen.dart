// Layar konfirmasi booking (BK-A11).
// QR code + kode booking + ringkasan.
// Route tidak bisa di-pop kembali ke payment.

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../shared/utils/currency_formatter.dart';
import '../../../../shared/utils/date_formatter.dart';
import '../../../../shared/widgets/qr_display.dart';

/// Data yang diteruskan ke layar ini via go_router extra atau query params.
class BookingConfirmationData {
  const BookingConfirmationData({
    required this.code,
    required this.branchName,
    required this.serviceName,
    required this.scheduledStart,
    required this.scheduledEnd,
    required this.totalPriceIdr,
    this.therapistName,
  });

  final String code;
  final String branchName;
  final String serviceName;
  final String scheduledStart;
  final String scheduledEnd;
  final int totalPriceIdr;
  final String? therapistName;
}

class BookingConfirmationScreen extends ConsumerWidget {
  const BookingConfirmationScreen({super.key, required this.data});

  final BookingConfirmationData data;

  /// Format kode menjadi "XXXX-XXXX".
  String get _formattedCode {
    final raw = data.code.replaceAll('-', '');
    if (raw.length >= 8) {
      return '${raw.substring(0, 4)}-${raw.substring(4, 8)}';
    }
    return data.code;
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false, // tidak bisa kembali ke payment
        title: const Text('Konfirmasi Booking'),
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            // Success icon
            Icon(Icons.check_circle_rounded, size: 72, color: cs.primary),
            const SizedBox(height: 16),
            Text(
              'Booking Berhasil!',
              style: theme.textTheme.headlineMedium?.copyWith(
                fontWeight: FontWeight.w600,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 24),

            // QR Code card
            Card(
              elevation: 2,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(16),
              ),
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  children: [
                    Semantics(
                      label:
                          'QR code untuk booking. Kode: $_formattedCode. Tunjukkan kepada staff saat check-in.',
                      child: QrDisplay(
                        data: data.code.replaceAll('-', ''),
                        size: 200,
                      ),
                    ),
                    const SizedBox(height: 16),
                    // Code text — monospace style, tap to copy
                    GestureDetector(
                      onLongPress: () => _copyCode(context),
                      child: Text(
                        _formattedCode,
                        style: const TextStyle(
                          fontSize: 28,
                          fontWeight: FontWeight.w700,
                          letterSpacing: 4.0,
                          fontFamily: 'monospace',
                        ),
                        textAlign: TextAlign.center,
                      ),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      'Ketuk lama untuk menyalin kode',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: cs.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      'Tunjukkan ini saat check-in',
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: cs.onSurfaceVariant,
                      ),
                      textAlign: TextAlign.center,
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 24),

            // Booking summary card
            Card(
              child: Column(
                children: [
                  ListTile(
                    leading: const Icon(Icons.store_outlined),
                    title: Text(data.branchName),
                  ),
                  ListTile(
                    leading: const Icon(Icons.spa_outlined),
                    title: Text(data.serviceName),
                  ),
                  ListTile(
                    leading: const Icon(Icons.calendar_today_outlined),
                    title: Text(
                      '${DateFormatter.formatDate(data.scheduledStart)} • ${DateFormatter.formatSlotRange(data.scheduledStart, data.scheduledEnd)}',
                    ),
                  ),
                  if (data.therapistName != null)
                    ListTile(
                      leading: const Icon(Icons.person_outlined),
                      title: Text(data.therapistName!),
                    )
                  else
                    const ListTile(
                      leading: Icon(Icons.person_outlined),
                      title: Text('Terapis dipilihkan'),
                    ),
                  ListTile(
                    leading: const Icon(Icons.payments_outlined),
                    title: Text(
                      CurrencyFormatter.formatRupiah(data.totalPriceIdr),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),

            // Email notice
            Row(
              children: [
                Icon(Icons.email_outlined, size: 16, color: cs.primary),
                const SizedBox(width: 4),
                Expanded(
                  child: Text(
                    'Kode ini juga sudah dikirim ke email kamu. Simpan kode ini!',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: cs.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 24),

            // Share button
            OutlinedButton.icon(
              icon: const Icon(Icons.share_outlined),
              label: const Text('Bagikan Kode'),
              onPressed: () {
                // Share.share di Phase 6 via share_plus
                _copyCode(context);
              },
              style: OutlinedButton.styleFrom(
                minimumSize: const Size(double.infinity, 48),
              ),
            ),
            const SizedBox(height: 12),

            // Back to home
            TextButton(
              onPressed: () => context.go('/'),
              child: const Text('Kembali ke Beranda'),
            ),
          ],
        ),
      ),
    );
  }

  void _copyCode(BuildContext context) {
    Clipboard.setData(ClipboardData(text: _formattedCode));
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('Kode disalin.')));
  }
}
