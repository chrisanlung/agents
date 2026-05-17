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
import '../../../../shared/utils/currency_formatter.dart';
import '../../../../shared/widgets/qr_display.dart';
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
    this.paymentChannel,
    this.qrString,
    this.vaNumber,
    this.vaBank,
    this.qrExpiresAt,
    this.paymentReference,
  });

  final String code;
  final int totalPriceIdr;
  final String scheduledStart;
  final String scheduledEnd;
  final String branchName;
  final String serviceName;

  /// 'qris' | 'va_<bank>'. Null fallback → assume 'qris'.
  final String? paymentChannel;

  /// QRIS string — dirender oleh qr_flutter (set when channel = 'qris').
  final String? qrString;

  /// Nomor Virtual Account (set when channel starts with 'va_').
  final String? vaNumber;

  /// Kode bank VA: bca / mandiri / bni / bri / permata / cimb.
  final String? vaBank;

  /// ISO-8601 batas waktu pembayaran.
  final String? qrExpiresAt;
  final String? paymentReference;

  /// True ketika channel = VA (bukan QRIS).
  bool get isVA => (paymentChannel ?? 'qris').startsWith('va_');
}

// ---------------------------------------------------------------------------
// Public entry-point widget — receives route data, handles submit if needed.
// ---------------------------------------------------------------------------

/// Layar pembayaran.
/// Menerima [branchId] (untuk wizard state) dan [routeData] (route extra)
/// yang sudah diisi oleh [BookingSelectionScreen] setelah submit booking berhasil.
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
  void didUpdateWidget(covariant PaymentScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    // After legacy submit screen calls context.replace with PaymentRouteData,
    // the same State is reused but routeData becomes non-null. initState
    // already ran with null, so we (re)start countdown here when expiry
    // appears or changes.
    final newExpiry = widget.routeData?.qrExpiresAt;
    final oldExpiry = oldWidget.routeData?.qrExpiresAt;
    if (newExpiry != null && newExpiry != oldExpiry) {
      _countdownTimer?.cancel();
      _qrExpired = false;
      _startCountdown(newExpiry);
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
      expiry = DateTime.parse(isoExpiry).toLocal();
    } catch (e) {
      debugPrint(
        '[payment_screen] failed to parse qr_expires_at="$isoExpiry": $e',
      );
      // Fallback: assume QR valid for 15 minutes from now (matches ADR 0015 §2.7).
      expiry = DateTime.now().add(const Duration(minutes: 15));
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
    // routeData must always be non-null post-Phase-6: the booking is now
    // submitted from BookingSelectionScreen before navigating here.
    assert(
      _data != null,
      'PaymentScreen reached with null routeData — '
      'booking submission must happen in BookingSelectionScreen.',
    );
    if (_data == null) {
      // Unreachable in production; shown in debug as a guard.
      return Scaffold(
        appBar: AppBar(title: const Text('Pembayaran')),
        body: const Center(child: Text('Data pembayaran tidak tersedia.')),
      );
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

            // -- DEV-only payment reference (for sandbox simulator) --
            if (AppConfig.isDev && data.paymentReference != null) ...[
              const SizedBox(height: 8),
              _PaymentReferenceChip(reference: data.paymentReference!),
            ],

            const SizedBox(height: 24),

            // -- Payment payload (QR or VA depending on channel) --
            if (data.isVA)
              _VaPaymentCard(
                vaNumber: data.vaNumber ?? '',
                vaBank: data.vaBank ?? '',
              )
            else
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

            // -- Instruction (channel-aware) --
            Text(
              data.isVA
                  ? 'Buka m-banking ${_bankDisplayName(data.vaBank)}, pilih Transfer → Virtual Account, masukkan nomor di atas.'
                  : 'Scan QR ini dengan Dana / GoPay / OVO / m-banking BCA / dll.',
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

class _PaymentReferenceChip extends StatelessWidget {
  const _PaymentReferenceChip({required this.reference});

  final String reference;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return InkWell(
      onTap: () {
        Clipboard.setData(ClipboardData(text: reference));
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Payment reference disalin.'),
            duration: Duration(seconds: 2),
          ),
        );
      },
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          border: Border.all(color: Colors.amber.shade700, width: 1),
          borderRadius: BorderRadius.circular(8),
          color: Colors.amber.shade50,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.bolt, size: 14, color: Colors.amber.shade800),
            const SizedBox(width: 4),
            Text(
              'DEV ref: ',
              style: TextStyle(
                fontSize: 11,
                color: Colors.amber.shade900,
                fontWeight: FontWeight.w700,
                letterSpacing: 0.5,
              ),
            ),
            SelectableText(
              reference,
              style: TextStyle(
                fontFamily: 'monospace',
                fontSize: 11,
                color: cs.onSurface,
              ),
            ),
            const SizedBox(width: 4),
            Icon(Icons.copy, size: 12, color: Colors.amber.shade800),
          ],
        ),
      ),
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

// _LegacySubmitPaymentScreen, _PriceCard, _SubmitBar, _PriceRow removed.
// Booking submission now handled in BookingSelectionScreen before navigating
// here. PaymentScreen is QR-only post-Phase-6.

// ---------------------------------------------------------------------------
// Virtual Account card — shown when channel = "va_<bank>" (migration 000037).
// ---------------------------------------------------------------------------

String _bankDisplayName(String? code) {
  switch (code) {
    case 'bca':
      return 'BCA';
    case 'mandiri':
      return 'Mandiri';
    case 'bni':
      return 'BNI';
    case 'bri':
      return 'BRI';
    case 'permata':
      return 'Permata';
    case 'cimb':
      return 'CIMB';
    default:
      return code?.toUpperCase() ?? '';
  }
}

class _VaPaymentCard extends StatelessWidget {
  const _VaPaymentCard({required this.vaNumber, required this.vaBank});

  final String vaNumber;
  final String vaBank;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final bankName = _bankDisplayName(vaBank);

    return Card(
      elevation: 2,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          children: [
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              decoration: BoxDecoration(
                color: cs.primaryContainer,
                borderRadius: BorderRadius.circular(20),
              ),
              child: Text(
                'Virtual Account $bankName',
                style: theme.textTheme.labelLarge?.copyWith(
                  fontWeight: FontWeight.w700,
                  color: cs.onPrimaryContainer,
                ),
              ),
            ),
            const SizedBox(height: 16),
            Text(
              vaNumber,
              style: const TextStyle(
                fontSize: 28,
                fontWeight: FontWeight.w700,
                letterSpacing: 2,
                fontFamily: 'monospace',
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 12),
            OutlinedButton.icon(
              icon: const Icon(Icons.copy_outlined, size: 18),
              label: const Text('Salin Nomor VA'),
              style: OutlinedButton.styleFrom(
                minimumSize: const Size(double.infinity, 44),
              ),
              onPressed: () async {
                await Clipboard.setData(ClipboardData(text: vaNumber));
                if (context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(content: Text('Nomor VA disalin.')),
                  );
                }
              },
            ),
          ],
        ),
      ),
    );
  }
}
