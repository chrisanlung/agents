// Layar pembayaran DUMMY (BK-A10).
// Menampilkan rincian harga + simulasi pembayaran.
// Phase 6+: ganti dengan Midtrans Snap WebView.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/storage/recent_bookings_storage.dart';
import '../../../../shared/utils/currency_formatter.dart';
import '../../../../shared/utils/date_formatter.dart';
import '../../../branch/presentation/providers/branch_detail_provider.dart';
import '../../../my_bookings/data/recent_bookings_notifier.dart';
import '../../data/booking_model.dart';
import '../providers/booking_provider.dart';

class PaymentScreen extends ConsumerWidget {
  const PaymentScreen({super.key, required this.branchId});
  final String branchId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final branchAsync = ref.watch(branchDetailProvider(branchId));
    final wizardState = ref.watch(bookingWizardProvider(branchId));
    final submitState = ref.watch(bookingSubmitProvider);

    return branchAsync.when(
      loading: () => Scaffold(
        appBar: AppBar(title: const Text('Pembayaran')),
        body: const Center(child: CircularProgressIndicator.adaptive()),
      ),
      error: (_, __) => Scaffold(
        appBar: AppBar(title: const Text('Pembayaran')),
        body: const Center(child: Text('Gagal memuat data.')),
      ),
      data: (branch) {
        final service = branch.services
            .where((s) => s.id == wizardState.selectedService)
            .firstOrNull;
        final selectedAddons = branch.services
            .expand((s) => s.addons)
            .where((a) => wizardState.selectedAddonIds.contains(a.id))
            .toList();
        final therapist = wizardState.selectedTherapistId != null
            ? branch.therapists
                  .where((t) => t.id == wizardState.selectedTherapistId)
                  .firstOrNull
            : null;
        final room = wizardState.selectedRoomId != null
            ? branch.rooms
                  .where((r) => r.id == wizardState.selectedRoomId)
                  .firstOrNull
            : null;

        var total = service?.priceIdr ?? 0;
        for (final a in selectedAddons) {
          total += a.priceIdr;
        }

        final isLoading = submitState is AsyncLoading;

        return PopScope(
          canPop: !isLoading,
          child: Scaffold(
            appBar: AppBar(
              title: const Text('Pembayaran'),
              automaticallyImplyLeading: !isLoading,
            ),
            body: SingleChildScrollView(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 100),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // DUMMY MODE notice
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: Theme.of(
                        context,
                      ).colorScheme.secondary.withAlpha(30),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Row(
                      children: [
                        Icon(
                          Icons.info_outline,
                          color: Theme.of(context).colorScheme.secondary,
                        ),
                        const SizedBox(width: 8),
                        const Expanded(
                          child: Text(
                            'MODE PENGUJIAN — Klik Bayar untuk simulasi pembayaran sukses.',
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 24),

                  // Price breakdown
                  Card(
                    child: Padding(
                      padding: const EdgeInsets.all(16),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'Rincian Pembayaran',
                            style: Theme.of(context).textTheme.titleMedium,
                          ),
                          const Divider(height: 20),
                          if (service != null)
                            _PriceRow(
                              'Layanan: ${service.name}',
                              service.priceIdr,
                            ),
                          ...selectedAddons.map(
                            (a) => _PriceRow(a.name, a.priceIdr),
                          ),
                          const Divider(height: 20),
                          _PriceRow('Total', total, bold: true),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 24),

                  // Booking summary
                  Card(
                    child: Column(
                      children: [
                        if (wizardState.selectedSlot != null)
                          ListTile(
                            leading: const Icon(Icons.calendar_today_outlined),
                            title: Text(
                              '${DateFormatter.formatDate(wizardState.selectedSlot!.start)} • ${DateFormatter.formatTime(wizardState.selectedSlot!.start)}',
                            ),
                          ),
                        ListTile(
                          leading: const Icon(Icons.person_outlined),
                          title: Text(
                            therapist?.fullName ??
                                'Terapis dipilihkan otomatis',
                          ),
                        ),
                        ListTile(
                          leading: const Icon(Icons.meeting_room_outlined),
                          title: Text(
                            room?.name ?? 'Ruangan dipilihkan otomatis',
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
            bottomNavigationBar: Container(
              padding: EdgeInsets.fromLTRB(
                16,
                12,
                16,
                12 + MediaQuery.of(context).viewPadding.bottom,
              ),
              color: Theme.of(context).colorScheme.surface,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  FilledButton.icon(
                    icon: isLoading
                        ? const SizedBox(
                            width: 16,
                            height: 16,
                            child: CircularProgressIndicator.adaptive(
                              strokeWidth: 2,
                            ),
                          )
                        : const Icon(Icons.lock_outlined),
                    label: Text(
                      isLoading
                          ? 'Memproses...'
                          : 'Bayar — ${CurrencyFormatter.formatRupiah(total)}',
                    ),
                    style: FilledButton.styleFrom(
                      minimumSize: const Size(double.infinity, 52),
                    ),
                    onPressed: isLoading
                        ? null
                        : () => _submitPayment(
                            context,
                            ref,
                            wizardState,
                            branch.name,
                            service?.name ?? '',
                            total,
                          ),
                  ),
                  if (isLoading)
                    const Padding(
                      padding: EdgeInsets.only(top: 4),
                      child: LinearProgressIndicator(),
                    ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Future<void> _submitPayment(
    BuildContext context,
    WidgetRef ref,
    BookingWizardState wizard,
    String branchName,
    String serviceName,
    int total,
  ) async {
    final response = await ref
        .read(bookingSubmitProvider.notifier)
        .submit(wizard);

    if (!context.mounted) return;

    if (response == null) {
      final errMsg =
          ref.read(bookingSubmitProvider).error?.toString() ??
          'Terjadi kesalahan. Coba lagi.';
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(errMsg)));
      return;
    }

    // Save to recent bookings
    final recentBooking = RecentBooking(
      code: response.code,
      branchName: branchName,
      scheduledStart: response.scheduledStart,
      serviceName: serviceName,
      totalPriceIdr: response.totalPriceIdr,
    );
    await ref.read(recentBookingsProvider.notifier).add(recentBooking);

    // Reset wizard state
    ref.read(bookingWizardProvider(branchId).notifier).reset();

    if (!context.mounted) return;
    // Navigate to confirmation (replace so user can't go back to payment)
    context.go(
      '/confirmation/${response.code}',
      extra: _ConfirmationExtra(
        branchName: branchName,
        serviceName: serviceName,
        scheduledStart: response.scheduledStart,
        scheduledEnd: response.scheduledEnd,
        totalPriceIdr: response.totalPriceIdr,
        therapistName: null,
      ),
    );
  }
}

/// Data passed to confirmation screen via go_router extra.
class _ConfirmationExtra {
  const _ConfirmationExtra({
    required this.branchName,
    required this.serviceName,
    required this.scheduledStart,
    required this.scheduledEnd,
    required this.totalPriceIdr,
    this.therapistName,
  });

  final String branchName;
  final String serviceName;
  final String scheduledStart;
  final String scheduledEnd;
  final int totalPriceIdr;
  final String? therapistName;
}

class _PriceRow extends StatelessWidget {
  const _PriceRow(this.label, this.amount, {this.bold = false});
  final String label;
  final int amount;
  final bool bold;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final style = bold
        ? theme.textTheme.bodyMedium?.copyWith(
            fontWeight: FontWeight.w700,
            color: theme.colorScheme.primary,
          )
        : theme.textTheme.bodyMedium;

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Expanded(child: Text(label, style: style)),
          Text(CurrencyFormatter.formatRupiah(amount), style: style),
        ],
      ),
    );
  }
}
