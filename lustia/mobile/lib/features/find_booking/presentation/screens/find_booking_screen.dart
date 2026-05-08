// Layar "Cari Booking" — guest fallback untuk mencari booking berdasarkan kode.
// Memanggil GET /api/v1/public/bookings/:code tanpa autentikasi.
// Setelah ditemukan, menampilkan detail dan menawarkan simpan ke "Booking Saya".

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/exceptions/app_exception.dart';
import '../../../../core/storage/recent_bookings_storage.dart';
import '../../../../features/booking/data/booking_model.dart';
import '../../../../features/booking/data/booking_repository.dart';
import '../../../../features/my_bookings/data/recent_bookings_notifier.dart';
import '../../../../shared/utils/currency_formatter.dart';
import '../../../../shared/utils/date_formatter.dart';
import '../../../../shared/widgets/qr_display.dart';

// ---------------------------------------------------------------------------
// Code input formatter — auto-uppercase + inserts hyphen after 4th char
// ---------------------------------------------------------------------------
class _BookingCodeFormatter extends TextInputFormatter {
  static final _allowed = RegExp(r'[A-Z2-7]');

  @override
  TextEditingValue formatEditUpdate(
    TextEditingValue oldValue,
    TextEditingValue newValue,
  ) {
    // Strip everything except allowed base32 chars (already uppercased)
    final cleaned = newValue.text
        .toUpperCase()
        .split('')
        .where((c) => _allowed.hasMatch(c))
        .join();

    final capped = cleaned.length > 8 ? cleaned.substring(0, 8) : cleaned;

    // Re-insert hyphen after position 4
    String formatted;
    if (capped.length > 4) {
      formatted = '${capped.substring(0, 4)}-${capped.substring(4)}';
    } else {
      formatted = capped;
    }

    return TextEditingValue(
      text: formatted,
      selection: TextSelection.collapsed(offset: formatted.length),
    );
  }
}

// ---------------------------------------------------------------------------
// Regex validator
// ---------------------------------------------------------------------------
final _codeRegex = RegExp(r'^[A-Z2-7]{4}-[A-Z2-7]{4}$');

bool _isValidCode(String value) => _codeRegex.hasMatch(value);

// ---------------------------------------------------------------------------
// Main screen
// ---------------------------------------------------------------------------
class FindBookingScreen extends ConsumerStatefulWidget {
  const FindBookingScreen({super.key});

  @override
  ConsumerState<FindBookingScreen> createState() => _FindBookingScreenState();
}

class _FindBookingScreenState extends ConsumerState<FindBookingScreen> {
  final _controller = TextEditingController();
  final _formKey = GlobalKey<FormState>();

  bool _valid = false;
  bool _loading = false;
  String? _fieldError;

  @override
  void initState() {
    super.initState();
    _controller.addListener(_onTextChanged);
  }

  @override
  void dispose() {
    _controller
      ..removeListener(_onTextChanged)
      ..dispose();
    super.dispose();
  }

  void _onTextChanged() {
    final v = _controller.text;
    setState(() {
      _valid = _isValidCode(v);
      _fieldError = null; // clear inline error while user types
    });
  }

  String? _validate(String? value) {
    if (value == null || value.isEmpty) {
      return 'Masukkan kode booking.';
    }
    if (!_isValidCode(value)) {
      return 'Format kode tidak valid. Contoh: B7K3-M2QF';
    }
    return null;
  }

  Future<void> _search() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;

    final rawCode = _controller.text; // already XXXX-XXXX

    setState(() {
      _loading = true;
      _fieldError = null;
    });

    try {
      final repo = ref.read(bookingRepositoryProvider);
      final booking = await repo.getBookingByCode(rawCode);

      if (!mounted) return;

      setState(() => _loading = false);

      // Navigate to result screen via push so user can go back
      await Navigator.of(context).push<void>(
        MaterialPageRoute<void>(
          builder: (_) => _FindBookingResultScreen(booking: booking),
        ),
      );
    } on DioException catch (e) {
      if (!mounted) return;
      setState(() => _loading = false);

      final appEx = e.requestOptions.extra['appException'];
      if (appEx is NotFoundException || appEx is ValidationException) {
        setState(() {
          _fieldError =
              'Kode booking tidak ditemukan. Periksa lagi atau hubungi cabang.';
        });
      } else {
        final msg = appEx is AppException
            ? appEx.message
            : 'Terjadi kesalahan. Coba lagi.';
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(msg), behavior: SnackBarBehavior.floating),
          );
        }
      }
    } catch (_) {
      if (!mounted) return;
      setState(() => _loading = false);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Terjadi kesalahan. Coba lagi.'),
          behavior: SnackBarBehavior.floating,
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    return Scaffold(
      appBar: AppBar(title: const Text('Cari Booking')),
      body: GestureDetector(
        onTap: () => FocusScope.of(context).unfocus(),
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(24, 32, 24, 24),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Header
                Text(
                  'Masukkan kode booking 8 karakter',
                  style: theme.textTheme.titleLarge?.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  'Format: XXXX-XXXX (contoh: B7K3-M2QF). '
                  'Kode ada di email konfirmasi setelah Anda booking.',
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: cs.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 32),

                // Code input
                TextFormField(
                  controller: _controller,
                  validator: _validate,
                  keyboardType: TextInputType.text,
                  textCapitalization: TextCapitalization.characters,
                  inputFormatters: [_BookingCodeFormatter()],
                  maxLength: 9, // 8 chars + hyphen
                  style: const TextStyle(
                    fontFamily: 'monospace',
                    fontSize: 22,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 4.0,
                  ),
                  decoration: InputDecoration(
                    labelText: 'Kode Booking',
                    hintText: 'B7K3-M2QF',
                    prefixIcon: const Icon(Icons.confirmation_number_outlined),
                    errorText: _fieldError,
                    border: const OutlineInputBorder(),
                    counterText: '',
                  ),
                  onFieldSubmitted: _valid && !_loading
                      ? (_) => _search()
                      : null,
                ),
                const SizedBox(height: 24),

                // Search button
                SizedBox(
                  width: double.infinity,
                  height: 56,
                  child: FilledButton(
                    onPressed: _valid && !_loading ? _search : null,
                    child: _loading
                        ? const SizedBox(
                            width: 24,
                            height: 24,
                            child: CircularProgressIndicator.adaptive(
                              strokeWidth: 2,
                            ),
                          )
                        : const Text(
                            'Cari',
                            style: TextStyle(
                              fontSize: 16,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Result screen — shown after successful lookup
// ---------------------------------------------------------------------------
class _FindBookingResultScreen extends ConsumerStatefulWidget {
  const _FindBookingResultScreen({required this.booking});
  final PublicBookingDetail booking;

  @override
  ConsumerState<_FindBookingResultScreen> createState() =>
      _FindBookingResultScreenState();
}

class _FindBookingResultScreenState
    extends ConsumerState<_FindBookingResultScreen> {
  bool _saved = false;
  bool _saving = false;

  String get _formattedCode {
    final raw = widget.booking.code.replaceAll('-', '');
    if (raw.length >= 8) {
      return '${raw.substring(0, 4)}-${raw.substring(4, 8)}';
    }
    return widget.booking.code;
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
      ['cancelled', 'no_show', 'expired'].contains(widget.booking.status);

  Future<void> _saveToMyBookings() async {
    setState(() => _saving = true);
    final entry = RecentBooking(
      code: widget.booking.code,
      branchName: widget.booking.branchName,
      serviceName: widget.booking.serviceName,
      scheduledStart: widget.booking.scheduledStart,
      totalPriceIdr: widget.booking.totalPriceIdr,
    );
    await ref.read(recentBookingsProvider.notifier).add(entry);
    if (mounted) {
      setState(() {
        _saved = true;
        _saving = false;
      });
    }
  }

  void _copyCode(BuildContext context) {
    Clipboard.setData(ClipboardData(text: _formattedCode));
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text('Kode disalin.'),
        behavior: SnackBarBehavior.floating,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final booking = widget.booking;

    return Scaffold(
      appBar: AppBar(title: const Text('Detail Booking')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            // Saved-to-MyBookings banner
            if (_saved)
              Container(
                width: double.infinity,
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 12,
                ),
                margin: const EdgeInsets.only(bottom: 16),
                decoration: BoxDecoration(
                  color: cs.primaryContainer,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Row(
                  children: [
                    Icon(Icons.check_circle_outline, color: cs.primary),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Tersimpan ke Booking Saya',
                        style: theme.textTheme.bodyMedium?.copyWith(
                          color: cs.onPrimaryContainer,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                    TextButton(
                      onPressed: () => context.go('/my-bookings'),
                      child: const Text('Lihat'),
                    ),
                  ],
                ),
              ),

            // Status badge
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
              decoration: BoxDecoration(
                color: _statusColor(context, booking.status).withAlpha(30),
                borderRadius: BorderRadius.circular(999),
                border: Border.all(
                  color: _statusColor(context, booking.status),
                ),
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
              child: QrDisplay(
                data: booking.code.replaceAll('-', ''),
                size: 200,
              ),
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

            // Booking summary card
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
                      '${DateFormatter.formatDate(booking.scheduledStart)} • '
                      '${DateFormatter.formatSlotRange(booking.scheduledStart, booking.scheduledEnd)}',
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
            const SizedBox(height: 24),

            // Save to My Bookings CTA
            if (!_saved)
              SizedBox(
                width: double.infinity,
                height: 56,
                child: OutlinedButton.icon(
                  onPressed: _saving ? null : _saveToMyBookings,
                  icon: _saving
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator.adaptive(
                            strokeWidth: 2,
                          ),
                        )
                      : const Icon(Icons.bookmark_add_outlined),
                  label: const Text('Simpan ke Booking Saya'),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
