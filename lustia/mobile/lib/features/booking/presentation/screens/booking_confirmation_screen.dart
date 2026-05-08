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
import '../../data/booking_model.dart';
import '../../data/booking_repository.dart';

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

  /// True ketika [data] sengaja kosong (router fallback / refresh halaman web)
  /// — kita perlu fetch detail dari API agar ringkasan tidak tampil placeholder.
  bool get _needsHydration =>
      data.branchName.isEmpty || data.serviceName.isEmpty;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    if (_needsHydration) {
      return _HydratingConfirmation(code: data.code);
    }

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false, // tidak bisa kembali ke payment
        title: const Text('Konfirmasi Booking'),
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          children: [
            // QR Code card — compacted: smaller QR, single helper line
            Card(
              elevation: 2,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(16),
              ),
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 16, 16, 12),
                child: Column(
                  children: [
                    Semantics(
                      label:
                          'QR code untuk booking. Kode: $_formattedCode. Tunjukkan kepada staff saat check-in.',
                      child: QrDisplay(
                        data: data.code.replaceAll('-', ''),
                        size: 160,
                      ),
                    ),
                    const SizedBox(height: 8),
                    GestureDetector(
                      onLongPress: () => _copyCode(context),
                      child: Text(
                        _formattedCode,
                        style: const TextStyle(
                          fontSize: 22,
                          fontWeight: FontWeight.w700,
                          letterSpacing: 3.0,
                          fontFamily: 'monospace',
                        ),
                        textAlign: TextAlign.center,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      'Tunjukkan QR / kode saat check-in',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: cs.onSurfaceVariant,
                      ),
                      textAlign: TextAlign.center,
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 12),

            // Booking summary card — compact rows (no ListTile padding)
            Card(
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 12,
                ),
                child: Column(
                  children: [
                    _SummaryRow(
                      icon: Icons.store_outlined,
                      text: data.branchName,
                    ),
                    _SummaryRow(
                      icon: Icons.spa_outlined,
                      text: data.serviceName,
                    ),
                    _SummaryRow(
                      icon: Icons.calendar_today_outlined,
                      text:
                          '${DateFormatter.formatDate(data.scheduledStart)} • ${DateFormatter.formatSlotRange(data.scheduledStart, data.scheduledEnd)}',
                    ),
                    _SummaryRow(
                      icon: Icons.person_outlined,
                      text: data.therapistName ?? 'Terapis dipilihkan',
                    ),
                    _SummaryRow(
                      icon: Icons.payments_outlined,
                      text: CurrencyFormatter.formatRupiah(data.totalPriceIdr),
                      isLast: true,
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 12),

            // Email notice
            Row(
              children: [
                Icon(Icons.email_outlined, size: 14, color: cs.primary),
                const SizedBox(width: 4),
                Expanded(
                  child: Text(
                    'Kode juga dikirim ke email. Simpan baik-baik.',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: cs.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
      // Sticky action bar — keeps "Salin Kode" + "Kembali ke Beranda" reachable
      // without scrolling. Only the booking detail scrolls above this.
      bottomNavigationBar: SafeArea(
        child: Container(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
          decoration: BoxDecoration(
            color: cs.surface,
            border: Border(top: BorderSide(color: cs.outlineVariant)),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              OutlinedButton.icon(
                icon: const Icon(Icons.copy_outlined),
                label: const Text('Salin Kode'),
                onPressed: () => _copyCode(context),
                style: OutlinedButton.styleFrom(
                  minimumSize: const Size(double.infinity, 48),
                ),
              ),
              const SizedBox(height: 4),
              TextButton(
                onPressed: () => context.go('/'),
                child: const Text('Kembali ke Beranda'),
              ),
            ],
          ),
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

/// Saat layar dibuka via deep-link / refresh halaman, `extra` GoRouter hilang
/// dan router menyodorkan BookingConfirmationData kosong (lihat app_router.dart
/// fallback). Komponen ini fetch detail booking dari API publik berdasarkan
/// kode di URL, lalu render ulang BookingConfirmationScreen dengan data lengkap.
class _HydratingConfirmation extends ConsumerWidget {
  const _HydratingConfirmation({required this.code});

  final String code;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncDetail = ref.watch(_publicBookingByCodeProvider(code));
    return asyncDetail.when(
      data: (detail) {
        return BookingConfirmationScreen(
          data: BookingConfirmationData(
            code: detail.code,
            branchName: detail.branchName,
            serviceName: detail.serviceName,
            scheduledStart: detail.scheduledStart,
            scheduledEnd: detail.scheduledEnd,
            totalPriceIdr: detail.totalPriceIdr,
          ),
        );
      },
      loading: () => const Scaffold(
        body: Center(child: CircularProgressIndicator()),
      ),
      error: (err, _) => Scaffold(
        appBar: AppBar(title: const Text('Konfirmasi Booking')),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.error_outline, size: 48),
                const SizedBox(height: 12),
                Text(
                  'Gagal memuat detail booking. Coba refresh halaman atau buka dari menu Riwayat.',
                  textAlign: TextAlign.center,
                  style: Theme.of(context).textTheme.bodyMedium,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

final _publicBookingByCodeProvider = FutureProvider.autoDispose
    .family<PublicBookingDetail, String>((ref, code) async {
      final repo = ref.watch(bookingRepositoryProvider);
      return repo.getBookingByCode(code);
    });

class _SummaryRow extends StatelessWidget {
  const _SummaryRow({
    required this.icon,
    required this.text,
    this.isLast = false,
  });

  final IconData icon;
  final String text;
  final bool isLast;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: EdgeInsets.only(bottom: isLast ? 0 : 8),
      child: Row(
        children: [
          Icon(icon, size: 18, color: theme.colorScheme.onSurfaceVariant),
          const SizedBox(width: 12),
          Expanded(
            child: Text(text, style: theme.textTheme.bodyMedium),
          ),
        ],
      ),
    );
  }
}
