// Layar pembayaran QRIS (BK-A10) — ADR 0015 §2.7–§2.8.
// Menampilkan QR QRIS dari response booking, countdown, polling status,
// dan tombol simulasi untuk DEV flavor.

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/config/app_config.dart';
import '../../../../core/exceptions/app_exception.dart';
import '../../../../core/storage/recent_bookings_storage.dart';
import '../../../../shared/utils/currency_formatter.dart';
import '../../../../shared/widgets/qr_display.dart';
import '../../../branch/presentation/providers/branch_detail_provider.dart';
import '../../../my_bookings/data/recent_bookings_notifier.dart';
import '../../data/booking_model.dart';
import '../../data/booking_repository.dart';
import '../providers/booking_provider.dart';
import 'booking_confirmation_screen.dart';

// ---------------------------------------------------------------------------
// Route extra — dados passados de BookingWizard → PaymentScreen.
// ---------------------------------------------------------------------------

/// Dados da resposta de criação de booking necessários na tela de pagamento.
final class PaymentRouteData {
  const PaymentRouteData({
    required this.code,
    required this.totalPriceIdr,
    required this.scheduledStart,
    required this.scheduledEnd,
    required this.branchName,
    required this.serviceName,
    this.qrString,
    this.qrExpiresAt,
    this.paymentReference,
  });

  final String code;
  final int totalPriceIdr;
  final String scheduledStart;
  final String scheduledEnd;
  final String branchName;
  final String serviceName;

  /// QRIS string — dirender oleh qr_flutter.
  final String? qrString;

  /// ISO-8601 string batas waktu QR.
  final String? qrExpiresAt;
  final String? paymentReference;
}

// ---------------------------------------------------------------------------
// Public entry-point widget — receives route data, handles submit if needed.
// ---------------------------------------------------------------------------

/// Layar pembayaran.
/// Menerima [branchId] (untuk wizard state) dan [routeData] (route extra)
/// yang sudah diisi oleh [BookingWizardScreen] setelah submit booking berhasil.
class PaymentScreen extends ConsumerStatefulWidget {
  const PaymentScreen({super.key, required this.branchId, this.routeData});

  final String branchId;

  /// Jika null, screen ini menampilkan form submit (legacy).
  /// Jika tidak null, screen langsung ke mode QR polling.
  final PaymentRouteData? routeData;

  @override
  ConsumerState<PaymentScreen> createState() => _PaymentScreenState();
}

class _PaymentScreenState extends ConsumerState<PaymentScreen> {
  // -- Countdown timer state --
  Timer? _countdownTimer;
  Duration _remaining = Duration.zero;
  bool _qrExpired = false;

  // -- Polling state --
  /// Last known status from polling; null = belum ada respons.
  String? _polledStatus;

  /// Pesan error polling inline (non-fatal).
  String? _pollingError;
  bool _simulatingPayment = false;

  PaymentRouteData? get _data => widget.routeData;

  @override
  void initState() {
    super.initState();
    if (_data?.qrExpiresAt != null) {
      _startCountdown(_data!.qrExpiresAt!);
    }
  }

  @override
  void dispose() {
    _countdownTimer?.cancel();
    super.dispose();
  }

  // ---------------------------------------------------------------------------
  // Countdown
  // ---------------------------------------------------------------------------

  void _startCountdown(String isoExpiry) {
    DateTime expiry;
    try {
      expiry = DateTime.parse(isoExpiry);
    } catch (_) {
      return;
    }

    void tick() {
      if (!mounted) return;
      final now = DateTime.now();
      final diff = expiry.difference(now);
      if (diff.isNegative || diff == Duration.zero) {
        setState(() {
          _remaining = Duration.zero;
          _qrExpired = true;
        });
        _countdownTimer?.cancel();
      } else {
        setState(() => _remaining = diff);
      }
    }

    tick(); // immediate first tick
    _countdownTimer = Timer.periodic(const Duration(seconds: 1), (_) => tick());
  }

  String get _countdownLabel {
    final m = _remaining.inMinutes.remainder(60).toString().padLeft(2, '0');
    final s = _remaining.inSeconds.remainder(60).toString().padLeft(2, '0');
    return '$m:$s';
  }

  // ---------------------------------------------------------------------------
  // Polling listener callback
  // ---------------------------------------------------------------------------

  void _onPollingUpdate(
    AsyncValue<PaymentStatusResponse>? previous,
    AsyncValue<PaymentStatusResponse> next,
  ) {
    if (!mounted) return;

    next.whenOrNull(
      data: (status) {
        setState(() {
          _polledStatus = status.status;
          _pollingError = null;
        });

        if (_polledStatus == 'paid') {
          _navigateToConfirmation();
        } else if (_polledStatus == 'expired' || _polledStatus == 'failed') {
          setState(() => _qrExpired = true);
          _countdownTimer?.cancel();
        }
      },
      error: (err, _) {
        final friendly = _friendlyPollError(err);
        setState(() => _pollingError = friendly);
      },
    );
  }

  String _friendlyPollError(Object? err) {
    final app = _resolveAppException(err);
    if (app is RateLimitException) {
      return 'Terlalu banyak permintaan. Menunggu...';
    }
    if (app is NetworkException) {
      return 'Tidak dapat terhubung. Memeriksa ulang...';
    }
    if (app is AppException) return app.message;
    return 'Gagal memeriksa status. Memeriksa ulang...';
  }

  AppException? _resolveAppException(Object? err) {
    if (err is AppException) return err;
    if (err is DioException) {
      final stashed = err.requestOptions.extra['appException'];
      if (stashed is AppException) return stashed;
    }
    return null;
  }

  // ---------------------------------------------------------------------------
  // Navigate to confirmation
  // ---------------------------------------------------------------------------

  void _navigateToConfirmation() {
    if (!mounted || _data == null) return;
    final d = _data!;
    _countdownTimer?.cancel();

    context.go(
      '/confirmation/${d.code}',
      extra: BookingConfirmationData(
        code: d.code,
        branchName: d.branchName,
        serviceName: d.serviceName,
        scheduledStart: d.scheduledStart,
        scheduledEnd: d.scheduledEnd,
        totalPriceIdr: d.totalPriceIdr,
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Dev simulate payment
  // ---------------------------------------------------------------------------

  Future<void> _simulatePayment() async {
    if (_data == null || _simulatingPayment) return;
    setState(() => _simulatingPayment = true);
    try {
      await ref
          .read(bookingRepositoryProvider)
          .triggerDummyPayment(_data!.code);
    } catch (_) {
      // Polling will detect state change; ignore errors here.
    } finally {
      if (mounted) setState(() => _simulatingPayment = false);
    }
  }

  // ---------------------------------------------------------------------------
  // Build
  // ---------------------------------------------------------------------------

  @override
  Widget build(BuildContext context) {
    // If routeData is null, this is in legacy-submit mode (shouldn't happen
    // post-Phase-6 but guard gracefully).
    if (_data == null) {
      return _LegacySubmitPaymentScreen(branchId: widget.branchId);
    }

    // Watch polling stream — listen for side-effects (navigate / update state).
    ref.listen(paymentStatusProvider(_data!.code), _onPollingUpdate);

    if (_qrExpired) {
      return _ExpiredScreen(
        onNewBooking: () => context.go('/branches/${widget.branchId}/book'),
      );
    }

    return _QrPaymentBody(
      data: _data!,
      countdownLabel: _countdownLabel,
      polledStatus: _polledStatus,
      pollingError: _pollingError,
      simulatingPayment: _simulatingPayment,
      onSimulate: AppConfig.isDev ? _simulatePayment : null,
    );
  }
}

// ---------------------------------------------------------------------------
// QR payment body
// ---------------------------------------------------------------------------

class _QrPaymentBody extends StatelessWidget {
  const _QrPaymentBody({
    required this.data,
    required this.countdownLabel,
    required this.polledStatus,
    required this.pollingError,
    required this.simulatingPayment,
    required this.onSimulate,
  });

  final PaymentRouteData data;
  final String countdownLabel;
  final String? polledStatus;
  final String? pollingError;
  final bool simulatingPayment;
  final VoidCallback? onSimulate; // null in prod

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final qrValue = data.qrString ?? data.code;

    // Format booking code as XXXX-XXXX
    final rawCode = data.code.replaceAll('-', '');
    final formattedCode = rawCode.length >= 8
        ? '${rawCode.substring(0, 4)}-${rawCode.substring(4, 8)}'
        : data.code;

    return Scaffold(
      appBar: AppBar(title: const Text('Pembayaran')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 32),
        child: Column(
          children: [
            // -- Total & code --
            Text(
              CurrencyFormatter.formatRupiah(data.totalPriceIdr),
              style: theme.textTheme.headlineMedium?.copyWith(
                fontWeight: FontWeight.w700,
                color: cs.primary,
              ),
            ),
            const SizedBox(height: 4),
            Text(
              'Booking #$formattedCode',
              style: theme.textTheme.bodyMedium?.copyWith(
                color: cs.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 24),

            // -- QR code --
            Card(
              elevation: 2,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(16),
              ),
              child: Padding(
                padding: const EdgeInsets.all(20),
                child: Semantics(
                  label:
                      'QR QRIS untuk pembayaran booking $formattedCode. '
                      'Scan dengan aplikasi e-wallet atau m-banking.',
                  child: QrDisplay(data: qrValue),
                ),
              ),
            ),
            const SizedBox(height: 16),

            // -- Instruction --
            Text(
              'Scan QR ini dengan Dana / GoPay / OVO / m-banking BCA / dll.',
              style: theme.textTheme.bodyMedium?.copyWith(
                color: cs.onSurfaceVariant,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 20),

            // -- Countdown --
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(Icons.timer_outlined, size: 18, color: cs.primary),
                const SizedBox(width: 6),
                Text(
                  'Sisa waktu: $countdownLabel',
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),

            // -- Polling status indicator --
            _PollingStatusBadge(
              status: polledStatus,
              errorMessage: pollingError,
            ),
            const SizedBox(height: 24),

            // -- Action buttons --
            Row(
              children: [
                Expanded(
                  child: OutlinedButton.icon(
                    icon: const Icon(Icons.copy_outlined),
                    label: const Text('Salin QRIS'),
                    onPressed: () => _copyQris(context, qrValue),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: OutlinedButton.icon(
                    icon: const Icon(Icons.close),
                    label: const Text('Batal'),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: cs.error,
                      side: BorderSide(color: cs.error),
                    ),
                    onPressed: () => _confirmCancel(context),
                  ),
                ),
              ],
            ),

            // -- DEV mode only --
            if (onSimulate != null) ...[
              const SizedBox(height: 24),
              const _DevModeDivider(),
              const SizedBox(height: 12),
              FilledButton.icon(
                icon: simulatingPayment
                    ? const SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator.adaptive(
                          strokeWidth: 2,
                        ),
                      )
                    : const Icon(Icons.bolt),
                label: Text(
                  simulatingPayment ? 'Memproses...' : 'Simulasikan Pembayaran',
                ),
                style: FilledButton.styleFrom(
                  minimumSize: const Size(double.infinity, 48),
                  backgroundColor: Colors.amber.shade700,
                  foregroundColor: Colors.black,
                ),
                onPressed: simulatingPayment ? null : onSimulate,
              ),
            ],
          ],
        ),
      ),
    );
  }

  void _copyQris(BuildContext context, String qrValue) {
    Clipboard.setData(ClipboardData(text: qrValue));
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('QRIS disalin ke clipboard.')));
  }

  void _confirmCancel(BuildContext context) {
    showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Batalkan pembayaran?'),
        content: const Text(
          'Booking akan tetap tersimpan namun belum dibayar. '
          'Kamu bisa melanjutkan pembayaran nanti via "Cari Booking".',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Kembali'),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Batalkan'),
          ),
        ],
      ),
    ).then((confirmed) {
      if (confirmed == true && context.mounted) {
        context.go('/');
      }
    });
  }
}

// ---------------------------------------------------------------------------
// Sub-widgets
// ---------------------------------------------------------------------------

class _PollingStatusBadge extends StatelessWidget {
  const _PollingStatusBadge({required this.status, this.errorMessage});

  final String? status;
  final String? errorMessage;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;

    if (errorMessage != null) {
      return Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.warning_amber_outlined, size: 16, color: cs.error),
          const SizedBox(width: 6),
          Flexible(
            child: Text(
              errorMessage!,
              style: TextStyle(color: cs.error, fontSize: 13),
              textAlign: TextAlign.center,
            ),
          ),
        ],
      );
    }

    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        SizedBox(
          width: 16,
          height: 16,
          child: CircularProgressIndicator.adaptive(
            strokeWidth: 2,
            valueColor: AlwaysStoppedAnimation<Color>(cs.primary),
          ),
        ),
        const SizedBox(width: 8),
        Text(
          'Menunggu pembayaran...',
          style: TextStyle(color: cs.onSurfaceVariant, fontSize: 14),
        ),
      ],
    );
  }
}

class _DevModeDivider extends StatelessWidget {
  const _DevModeDivider();

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        const Expanded(child: Divider()),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8),
          child: Text(
            'DEV MODE',
            style: TextStyle(
              fontSize: 11,
              color: Colors.amber.shade700,
              fontWeight: FontWeight.w700,
              letterSpacing: 1,
            ),
          ),
        ),
        const Expanded(child: Divider()),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Expired screen
// ---------------------------------------------------------------------------

class _ExpiredScreen extends StatelessWidget {
  const _ExpiredScreen({required this.onNewBooking});

  final VoidCallback onNewBooking;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    return Scaffold(
      appBar: AppBar(title: const Text('Pembayaran')),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.timer_off_outlined, size: 64, color: cs.error),
              const SizedBox(height: 16),
              Text(
                'QR kadaluarsa',
                style: theme.textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
              Text(
                'Waktu pembayaran sudah habis. Silakan buat booking baru.',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: cs.onSurfaceVariant,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 32),
              FilledButton.icon(
                icon: const Icon(Icons.refresh),
                label: const Text('Buat Booking Baru'),
                style: FilledButton.styleFrom(
                  minimumSize: const Size(double.infinity, 52),
                ),
                onPressed: onNewBooking,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Legacy submit screen — shown only when routeData is null.
// Handles booking submission then navigates to QR mode.
// ---------------------------------------------------------------------------

class _LegacySubmitPaymentScreen extends ConsumerWidget {
  const _LegacySubmitPaymentScreen({required this.branchId});

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
                  _PriceCard(
                    service: service,
                    selectedAddons: selectedAddons,
                    total: total,
                  ),
                ],
              ),
            ),
            bottomNavigationBar: _SubmitBar(
              isLoading: isLoading,
              total: total,
              onTap: isLoading
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
          ),
        );
      },
    );
  }

  Future<void> _submitPayment(
    BuildContext context,
    WidgetRef ref,
    dynamic wizard,
    String branchName,
    String serviceName,
    int total,
  ) async {
    final response = await ref
        .read(bookingSubmitProvider.notifier)
        .submit(wizard);

    if (!context.mounted) return;

    if (response == null) {
      final errMsg = _friendlyPaymentError(
        ref.read(bookingSubmitProvider).error,
      );
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(errMsg),
          duration: const Duration(seconds: 5),
          action: SnackBarAction(
            label: 'Ganti Slot',
            onPressed: () {
              if (context.mounted) context.pop();
            },
          ),
        ),
      );
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

    // Reset wizard
    ref.read(bookingWizardProvider(branchId).notifier).reset();

    if (!context.mounted) return;

    // Navigate to payment QR screen with data
    final routeData = PaymentRouteData(
      code: response.code,
      totalPriceIdr: response.totalPriceIdr,
      scheduledStart: response.scheduledStart,
      scheduledEnd: response.scheduledEnd,
      branchName: branchName,
      serviceName: serviceName,
      qrString: response.qrString,
      qrExpiresAt: response.qrExpiresAt,
      paymentReference: response.paymentReference,
    );

    // Replace current route so back-button from QR screen goes to wizard.
    context.replace('/branches/$branchId/book/payment', extra: routeData);
  }

  String _friendlyPaymentError(Object? raw) {
    final err = _resolveAppException(raw) ?? raw;
    if (err is ConflictException) {
      return 'Slot ini baru saja terisi. Silakan pilih slot lain.';
    }
    if (err is NetworkException) {
      return 'Tidak dapat terhubung ke server. Periksa koneksi internet Anda.';
    }
    if (err is RateLimitException) {
      return 'Terlalu banyak percobaan. Coba lagi sebentar lagi.';
    }
    if (err is AppException) {
      return err.message;
    }
    return 'Terjadi kesalahan. Coba lagi.';
  }

  AppException? _resolveAppException(Object? err) {
    if (err is AppException) return err;
    if (err is DioException) {
      final stashed = err.requestOptions.extra['appException'];
      if (stashed is AppException) return stashed;
    }
    return null;
  }
}

// ---------------------------------------------------------------------------
// Price card sub-widget (used in legacy submit mode)
// ---------------------------------------------------------------------------

class _PriceCard extends StatelessWidget {
  const _PriceCard({
    required this.service,
    required this.selectedAddons,
    required this.total,
  });

  final dynamic service;
  final List<dynamic> selectedAddons;
  final int total;

  @override
  Widget build(BuildContext context) {
    return Card(
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
              _PriceRow('Layanan: ${service.name}', service.priceIdr as int),
            ...selectedAddons.map(
              (a) => _PriceRow(a.name as String, a.priceIdr as int),
            ),
            const Divider(height: 20),
            _PriceRow('Total', total, bold: true),
          ],
        ),
      ),
    );
  }
}

class _SubmitBar extends StatelessWidget {
  const _SubmitBar({
    required this.isLoading,
    required this.total,
    required this.onTap,
  });

  final bool isLoading;
  final int total;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Container(
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
                    child: CircularProgressIndicator.adaptive(strokeWidth: 2),
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
            onPressed: onTap,
          ),
          if (isLoading)
            const Padding(
              padding: EdgeInsets.only(top: 4),
              child: LinearProgressIndicator(),
            ),
        ],
      ),
    );
  }
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
