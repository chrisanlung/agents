// Layar detail booking dari "Booking Saya" (/my-bookings/:code).
// Fetches GET /public/bookings/:code untuk status terkini.

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../../features/booking/data/booking_model.dart';
import '../../../../features/booking/data/booking_repository.dart';
import '../../../../shared/utils/currency_formatter.dart';
import '../../../../shared/utils/date_formatter.dart';
import '../../../../shared/widgets/error_view.dart';
import '../../../../shared/widgets/qr_display.dart';

part 'booking_detail_screen.g.dart';

@riverpod
Future<PublicBookingDetail> bookingDetail(
  BookingDetailRef ref,
  String code,
) async {
  final repo = ref.watch(bookingRepositoryProvider);
  return repo.getBookingByCode(code);
}

class BookingDetailScreen extends ConsumerWidget {
  const BookingDetailScreen({super.key, required this.code});
  final String code;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(bookingDetailProvider(code));

    return Scaffold(
      appBar: AppBar(title: const Text('Detail Booking')),
      body: async.when(
        loading: () =>
            const Center(child: CircularProgressIndicator.adaptive()),
        error: (_, __) => ErrorView(
          message: 'Gagal memuat detail booking. Coba lagi.',
          onRetry: () => ref.invalidate(bookingDetailProvider(code)),
        ),
        data: (booking) => _BookingDetailContent(booking: booking),
      ),
    );
  }
}

class _BookingDetailContent extends StatelessWidget {
  const _BookingDetailContent({required this.booking});
  final PublicBookingDetail booking;

  String get _formattedCode {
    final raw = booking.code.replaceAll('-', '');
    if (raw.length >= 8) {
      return '${raw.substring(0, 4)}-${raw.substring(4, 8)}';
    }
    return booking.code;
  }

  Color _statusColor(BuildContext context, String status) {
    final cs = Theme.of(context).colorScheme;
    switch (status) {
      case 'paid':
        return cs.tertiary;
      case 'checked_in':
        return cs.primary;
      case 'completed':
        return cs.onSurfaceVariant;
      case 'cancelled':
      case 'expired':
      case 'no_show':
        return cs.error;
      default:
        return cs.onSurfaceVariant;
    }
  }

  String _statusLabel(String status) {
    switch (status) {
      case 'paid':
        return 'Terbayar';
      case 'checked_in':
        return 'Check-in';
      case 'completed':
        return 'Selesai';
      case 'cancelled':
        return 'Dibatalkan';
      case 'no_show':
        return 'Tidak Hadir';
      case 'expired':
        return 'Kedaluwarsa';
      case 'pending_payment':
        return 'Menunggu Pembayaran';
      default:
        return status;
    }
  }

  bool get _isInactive =>
      ['cancelled', 'no_show', 'expired'].contains(booking.status);

  void _copyCode(BuildContext context) {
    Clipboard.setData(ClipboardData(text: _formattedCode));
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('Kode disalin.')));
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        children: [
          // Status badge
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
            decoration: BoxDecoration(
              color: _statusColor(context, booking.status).withAlpha(30),
              borderRadius: BorderRadius.circular(999),
              border: Border.all(color: _statusColor(context, booking.status)),
            ),
            child: Text(
              _statusLabel(booking.status),
              style: theme.textTheme.labelLarge?.copyWith(
                color: _statusColor(context, booking.status),
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
          const SizedBox(height: 20),

          // Inactive banner
          if (_isInactive)
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(12),
              margin: const EdgeInsets.only(bottom: 16),
              decoration: BoxDecoration(
                color: cs.errorContainer,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Text(
                'Kode ini sudah tidak bisa digunakan.',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: cs.onErrorContainer,
                ),
                textAlign: TextAlign.center,
              ),
            ),

          // QR Code
          Semantics(
            label:
                'QR code untuk booking. Kode: $_formattedCode. Tunjukkan kepada staff saat check-in.',
            child: QrDisplay(data: booking.code.replaceAll('-', ''), size: 200),
          ),
          const SizedBox(height: 16),
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
          const SizedBox(height: 24),

          // Booking summary
          Card(
            child: Column(
              children: [
                ListTile(
                  leading: const Icon(Icons.store_outlined),
                  title: Text(booking.branchName),
                ),
                ListTile(
                  leading: const Icon(Icons.spa_outlined),
                  title: Text(booking.serviceName),
                ),
                ListTile(
                  leading: const Icon(Icons.calendar_today_outlined),
                  title: Text(
                    '${DateFormatter.formatDate(booking.scheduledStart)} • ${DateFormatter.formatSlotRange(booking.scheduledStart, booking.scheduledEnd)}',
                  ),
                ),
                ListTile(
                  leading: const Icon(Icons.payments_outlined),
                  title: Text(
                    CurrencyFormatter.formatRupiah(booking.totalPriceIdr),
                  ),
                ),
                if (booking.addons.isNotEmpty)
                  ListTile(
                    leading: const Icon(Icons.add_circle_outline),
                    title: Text(booking.addons.map((a) => a.name).join(', ')),
                    subtitle: const Text('Tambahan'),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
