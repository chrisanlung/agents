// Layar "Booking Saya" (BK-A12).
// Daftar kode booking dari local storage. Tap → detail QR.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/router/app_router.dart';
import '../../../../shared/utils/currency_formatter.dart';
import '../../../../shared/utils/date_formatter.dart';
import '../../data/recent_bookings_notifier.dart';

class MyBookingsScreen extends ConsumerWidget {
  const MyBookingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final bookingsAsync = ref.watch(recentBookingsProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text('Booking Saya', style: theme.textTheme.headlineMedium),
      ),
      body: bookingsAsync.when(
        loading: () =>
            const Center(child: CircularProgressIndicator.adaptive()),
        error: (_, __) => const Center(child: Text('Gagal memuat booking.')),
        data: (bookings) {
          if (bookings.isEmpty) {
            return Column(
              children: [
                _FindBookingTile(cs: cs, theme: theme),
                Expanded(
                  child: Center(
                    child: Padding(
                      padding: const EdgeInsets.all(32),
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.receipt_long_outlined,
                            size: 64,
                            color: cs.onSurfaceVariant.withAlpha(102),
                          ),
                          const SizedBox(height: 16),
                          Text(
                            'Belum ada booking',
                            style: theme.textTheme.titleMedium?.copyWith(
                              color: cs.onSurfaceVariant,
                            ),
                          ),
                          const SizedBox(height: 8),
                          Text(
                            'Booking kamu akan muncul di sini setelah selesai memesan.',
                            style: theme.textTheme.bodyMedium?.copyWith(
                              color: cs.onSurfaceVariant,
                            ),
                            textAlign: TextAlign.center,
                          ),
                          const SizedBox(height: 24),
                          FilledButton(
                            onPressed: () => context.go('/'),
                            child: const Text('Cari Cabang'),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ],
            );
          }

          return ListView.builder(
            padding: const EdgeInsets.symmetric(vertical: 8),
            // +1 for the "Cari Booking" header tile
            itemCount: bookings.length + 1,
            itemBuilder: (context, index) {
              if (index == 0) {
                return _FindBookingTile(cs: cs, theme: theme);
              }
              final entry = bookings[index - 1];
              return Card(
                margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
                child: ListTile(
                  leading: Container(
                    width: 48,
                    height: 48,
                    decoration: BoxDecoration(
                      color: cs.primaryContainer,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Icon(Icons.receipt_long_outlined, color: cs.primary),
                  ),
                  title: Text(
                    entry.serviceName.isNotEmpty
                        ? entry.serviceName
                        : 'Booking',
                    style: theme.textTheme.titleMedium,
                  ),
                  subtitle: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        entry.branchName,
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: cs.onSurfaceVariant,
                        ),
                      ),
                      Text(
                        DateFormatter.formatDate(entry.scheduledStart),
                        style: theme.textTheme.bodySmall,
                      ),
                      if (entry.totalPriceIdr > 0)
                        Text(
                          CurrencyFormatter.formatRupiah(entry.totalPriceIdr),
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: cs.primary,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                    ],
                  ),
                  isThreeLine: true,
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () => context.push('/my-bookings/${entry.code}'),
                ),
              );
            },
          );
        },
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Reusable "Cari Booking" entry tile
// ---------------------------------------------------------------------------
class _FindBookingTile extends StatelessWidget {
  const _FindBookingTile({required this.cs, required this.theme});
  final ColorScheme cs;
  final ThemeData theme;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
      child: Card(
        color: cs.secondaryContainer,
        child: ListTile(
          leading: Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: cs.secondary,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Icon(Icons.search, color: cs.onSecondary),
          ),
          title: Text(
            'Cari Booking dengan Kode',
            style: theme.textTheme.titleSmall?.copyWith(
              fontWeight: FontWeight.w700,
              color: cs.onSecondaryContainer,
            ),
          ),
          subtitle: Text(
            'Punya kode dari email/SMS? Lihat detail booking di sini.',
            style: theme.textTheme.bodySmall?.copyWith(
              color: cs.onSecondaryContainer.withAlpha(180),
            ),
          ),
          trailing: Icon(Icons.chevron_right, color: cs.onSecondaryContainer),
          onTap: () => context.push(AppRoutes.findBooking),
        ),
      ),
    );
  }
}
