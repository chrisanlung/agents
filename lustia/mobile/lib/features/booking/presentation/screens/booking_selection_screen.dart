// Booking Selection Screen — BK-A6..A8 redesign (Phase 6 extended).
// Single progressive page (new order after UX tweaks):
//   ① Pilih Layanan  ② Tanggal  ③ Pilih Terapis  ④ Pilih Waktu (modal)
//   ⑤ Pilih Ruangan  ⑥ Tambahan  ⑦ Data Anda  ⑧ Ringkasan Booking
//
// UX tweaks applied:
//   Tweak-1: Service list → bottom-sheet modal (trigger row pattern).
//   Tweak-2: Time grid → bottom-sheet modal triggered by _TriggerRow.
//   Tweak-3: Tambahan moved after Ruangan; locked until room is picked.
//   Tweak-4: All selector fields use uniform _TriggerRow + bottom-sheet.
//            Section ② uses native showDatePicker (Material 3 dialog).
//
// Wizard screen (BookingWizardScreen) has been removed — this screen IS the
// entry-point for /branches/:id/book (and /branches/:id/book/select redirects
// here for backwards compatibility).
//
// Issue-1 fix (customer info clears on slot change):
//   Root cause: when _roomExplicitlyChosen flips false→true the Form remounts
//   and _customerInfoValid was stuck at false. Text controllers (_nameCtrl etc.)
//   are long-lived and retain text — fields never actually cleared. Fix: on
//   remount we schedule a postFrameCallback to re-run validation so
//   _customerInfoValid is restored if the controllers already contain valid
//   text. selectSlot / confirmTherapist do NOT touch customerName/Phone/Email
//   in the provider — confirmed by reading booking_provider.dart.
//
// Design tokens: docs/DESIGN_SYSTEM.md BS-T8, BS-T9, BS-T10, BS-T11.

import 'dart:async' show unawaited;

import 'package:cached_network_image/cached_network_image.dart';
import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:smooth_page_indicator/smooth_page_indicator.dart';

import '../../../../core/exceptions/app_exception.dart';
import '../../../../core/storage/recent_bookings_storage.dart';
import '../../../../shared/utils/currency_formatter.dart';
import '../../../../shared/utils/date_formatter.dart';
import '../../../branch/data/branch_model.dart';
import '../../../branch/presentation/providers/branch_detail_provider.dart';
import '../../../my_bookings/data/recent_bookings_notifier.dart';
import '../../data/availability_model.dart';
import '../../data/booking_model.dart';
import '../providers/availability_provider.dart';
import '../providers/booking_provider.dart';
import 'payment_screen.dart';

// ---------------------------------------------------------------------------
// Entry-point
// ---------------------------------------------------------------------------

class BookingSelectionScreen extends ConsumerWidget {
  const BookingSelectionScreen({super.key, required this.branchId});
  final String branchId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final branchAsync = ref.watch(branchDetailProvider(branchId));

    return branchAsync.when(
      loading: () => Scaffold(
        appBar: AppBar(title: const Text('Booking')),
        body: const Center(child: CircularProgressIndicator.adaptive()),
      ),
      error: (e, _) => Scaffold(
        appBar: AppBar(title: const Text('Booking')),
        body: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.cloud_off_outlined, size: 48),
              const SizedBox(height: 12),
              const Text(
                'Gagal memuat data cabang. Coba lagi.',
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 12),
              OutlinedButton(
                onPressed: () => ref.invalidate(branchDetailProvider(branchId)),
                child: const Text('Coba Lagi'),
              ),
            ],
          ),
        ),
      ),
      data: (branch) => _SelectionBody(branchId: branchId, branch: branch),
    );
  }
}

// ---------------------------------------------------------------------------
// Main body — owns the scroll controller + section key refs + form controllers
// ---------------------------------------------------------------------------

class _SelectionBody extends ConsumerStatefulWidget {
  const _SelectionBody({required this.branchId, required this.branch});
  final String branchId;
  final BranchDetail branch;

  @override
  ConsumerState<_SelectionBody> createState() => _SelectionBodyState();
}

class _SelectionBodyState extends ConsumerState<_SelectionBody> {
  final _scrollController = ScrollController();

  // GlobalKeys for scroll-to anchors. Section numbers map to the new order:
  //   ①service  ②date  ③therapist  ④time  ⑤room  ⑥addon  ⑦customerInfo  ⑧summary
  final _dateSectionKey = GlobalKey();
  final _therapistSectionKey = GlobalKey();
  final _timeSectionKey = GlobalKey();
  final _roomSectionKey = GlobalKey();
  final _addonSectionKey = GlobalKey();
  final _customerInfoSectionKey = GlobalKey();
  final _summarySectionKey = GlobalKey();

  // Customer info form key — owned here so CTA can call validate().
  final _customerFormKey = GlobalKey<FormState>();
  bool _customerInfoValid = false;

  // Tracks whether the user has made an explicit room selection (including
  // "Otomatis"). Required because selectedRoomId == null is ambiguous:
  // it means both "not yet chosen" and "Otomatis chosen".
  bool _roomExplicitlyChosen = false;

  bool _submitting = false;

  // Text controllers for the customer info fields.
  // These are long-lived (owned by _SelectionBodyState) and survive Section ⑦
  // being conditionally unmounted/remounted when _roomExplicitlyChosen flips.
  // See Issue-1 fix note at the top of the file.
  late TextEditingController _nameCtrl;
  late TextEditingController _phoneCtrl;
  late TextEditingController _emailCtrl;

  @override
  void initState() {
    super.initState();
    final s = ref.read(bookingWizardProvider(widget.branchId));
    _nameCtrl = TextEditingController(text: s.customerName);
    _phoneCtrl = TextEditingController(text: s.customerPhone);
    _emailCtrl = TextEditingController(text: s.customerEmail);

    // Auto-select today on first load if no date yet.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final state = ref.read(bookingWizardProvider(widget.branchId));
      if (state.selectedDate == null) {
        ref
            .read(bookingWizardProvider(widget.branchId).notifier)
            .selectDate(DateTime.now());
      }
    });
  }

  @override
  void dispose() {
    _scrollController.dispose();
    _nameCtrl.dispose();
    _phoneCtrl.dispose();
    _emailCtrl.dispose();
    super.dispose();
  }

  void _revalidateCustomerInfo() {
    final valid = _customerFormKey.currentState?.validate() ?? false;
    if (valid != _customerInfoValid) {
      setState(() => _customerInfoValid = valid);
    }
  }

  // Called whenever Section ⑦ remounts (room was re-picked after a slot change).
  // The controllers still have the user's text so we restore _customerInfoValid.
  void _onCustomerSectionMounted() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _revalidateCustomerInfo();
    });
  }

  void _scrollTo(GlobalKey key) {
    final ctx = key.currentContext;
    if (ctx == null) return;
    Scrollable.ensureVisible(
      ctx,
      duration: const Duration(milliseconds: 200),
      curve: Curves.easeOut,
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final state = ref.watch(bookingWizardProvider(widget.branchId));

    // Gate conditions (ordered top-down).
    final serviceSelected = state.selectedService != null;
    final dateSelected = state.selectedDate != null;
    final therapistConfirmed = state.therapistConfirmed;
    final slotSelected = state.selectedSlot != null;

    // CTA is enabled when all mandatory gates pass.
    // Addon section is NOT a gate — addons are optional.
    final ctaEnabled = serviceSelected &&
        dateSelected &&
        therapistConfirmed &&
        slotSelected &&
        _roomExplicitlyChosen &&
        state.customerName.trim().isNotEmpty &&
        state.customerPhone.trim().isNotEmpty &&
        state.customerEmail.trim().isNotEmpty &&
        _customerInfoValid;

    final service = widget.branch.services
        .where((s) => s.id == state.selectedService)
        .firstOrNull;

    var totalPrice = service?.priceIdr ?? 0;
    for (final a in widget.branch.addons) {
      if (state.selectedAddonIds.contains(a.id)) totalPrice += a.priceIdr;
    }
    final hasAddons = state.selectedAddonIds.isNotEmpty;

    // -----------------------------------------------------------------------
    // Derived display values for trigger rows
    // -----------------------------------------------------------------------

    // Section ① — service
    final serviceLabel = service != null ? service.name : 'Pilih layanan';
    final serviceSubtitle = service != null
        ? '${service.durationMinutes} menit · ${CurrencyFormatter.formatRupiah(service.priceIdr)}'
        : null;

    // Section ② — date
    final dateLabel = state.selectedDate != null
        ? DateFormatter.formatFullDate(state.selectedDate!)
        : 'Pilih tanggal';

    // Section ③ — therapist
    final therapistItem = state.selectedTherapistId != null
        ? widget.branch.therapists
              .where((t) => t.id == state.selectedTherapistId)
              .firstOrNull
        : null;
    final therapistLabel = therapistConfirmed
        ? (therapistItem?.fullName ?? 'Otomatis')
        : 'Pilih terapis';

    // Section ④ — time
    final slot = state.selectedSlot;
    final hasSlot = slot != null;
    final timeLabel = hasSlot
        ? DateFormatter.formatSlotRange(slot.start, slot.end)
        : 'Pilih waktu';

    // Section ⑤ — room
    final roomItem = state.selectedRoomId != null
        ? widget.branch.rooms
              .where((r) => r.id == state.selectedRoomId)
              .firstOrNull
        : null;
    final roomLabel = _roomExplicitlyChosen
        ? (roomItem?.name ?? 'Otomatis')
        : 'Pilih ruangan';
    final roomSubtitle = _roomExplicitlyChosen && roomItem != null
        ? '${roomItem.roomTypeLabel} · ${roomItem.capacity} kapasitas'
        : null;

    // Section ⑥ — addons
    final addonCount = state.selectedAddonIds.length;
    int addonTotal = 0;
    for (final a in widget.branch.addons) {
      if (state.selectedAddonIds.contains(a.id)) addonTotal += a.priceIdr;
    }
    const addonLabel = 'Tambahan (opsional)';
    final addonSubtitle = addonCount > 0
        ? '$addonCount item dipilih · ${CurrencyFormatter.formatRupiah(addonTotal)}'
        : 'Belum ada tambahan';

    return Scaffold(
      appBar: AppBar(
        title: const Text('Booking'),
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(3),
          child: LinearProgressIndicator(
            value: _progress(state),
            minHeight: 3,
            backgroundColor: cs.surfaceContainerHighest,
          ),
        ),
      ),
      bottomNavigationBar: _CtaBar(
        totalPrice: totalPrice,
        hasAddons: hasAddons,
        enabled: ctaEnabled && !_submitting,
        isLoading: _submitting,
        onTap: (ctaEnabled && !_submitting)
            ? () => _submitAndNavigate(context, state)
            : null,
      ),
      body: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 600),
        child: Align(
          alignment: Alignment.topCenter,
          child: SingleChildScrollView(
            controller: _scrollController,
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 32),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Section ① — Pilih Layanan (trigger row → bottom-sheet)
                const _SectionHeader(number: 1, title: 'Pilih Layanan'),
                const SizedBox(height: 8),
                _TriggerRow(
                  icon: Icons.spa_outlined,
                  label: serviceLabel,
                  subtitle: serviceSubtitle,
                  locked: false,
                  onTap: () => _openServiceSheet(context, state),
                ),
                const SizedBox(height: 24),

                // Section ② — Tanggal (trigger row → native date picker)
                const _SectionHeader(number: 2, title: 'Tanggal'),
                const SizedBox(height: 8),
                KeyedSubtree(
                  key: _dateSectionKey,
                  child: _TriggerRow(
                    icon: Icons.calendar_today_outlined,
                    label: dateLabel,
                    locked: !serviceSelected,
                    onTap: serviceSelected
                        ? () => _openDatePicker(context, state)
                        : () {},
                  ),
                ),
                const SizedBox(height: 24),

                // Section ③ — Pilih Terapis (trigger row → bottom-sheet carousel)
                const _SectionHeader(number: 3, title: 'Pilih Terapis'),
                const SizedBox(height: 8),
                KeyedSubtree(
                  key: _therapistSectionKey,
                  child: _TriggerRow(
                    leading: _buildTherapistLeading(
                      context,
                      therapistConfirmed,
                      therapistItem,
                    ),
                    label: therapistLabel,
                    locked: !serviceSelected,
                    onTap: serviceSelected
                        ? () => _openTherapistSheet(context, state)
                        : () {},
                  ),
                ),
                const SizedBox(height: 24),

                // Section ④ — Pilih Waktu (trigger row → bottom-sheet)
                const _SectionHeader(number: 4, title: 'Pilih Waktu'),
                const SizedBox(height: 8),
                KeyedSubtree(
                  key: _timeSectionKey,
                  child: _TriggerRow(
                    icon: Icons.schedule_outlined,
                    label: timeLabel,
                    locked: !therapistConfirmed,
                    onTap: therapistConfirmed
                        ? () => _openTimeSheet(context, state)
                        : () {},
                  ),
                ),
                const SizedBox(height: 24),

                // Section ⑤ — Pilih Ruangan (trigger row → bottom-sheet)
                const _SectionHeader(number: 5, title: 'Pilih Ruangan'),
                const SizedBox(height: 8),
                KeyedSubtree(
                  key: _roomSectionKey,
                  child: _TriggerRow(
                    icon: Icons.meeting_room_outlined,
                    label: roomLabel,
                    subtitle: roomSubtitle,
                    locked: !slotSelected,
                    onTap: slotSelected
                        ? () => _openRoomSheet(context, state)
                        : () {},
                  ),
                ),
                const SizedBox(height: 24),

                // Section ⑥ — Tambahan (trigger row → bottom-sheet; optional)
                const _SectionHeader(number: 6, title: 'Tambahan'),
                const SizedBox(height: 8),
                KeyedSubtree(
                  key: _addonSectionKey,
                  child: _TriggerRow(
                    icon: Icons.add_circle_outline,
                    label: addonLabel,
                    subtitle: addonSubtitle,
                    locked: !_roomExplicitlyChosen,
                    onTap: _roomExplicitlyChosen
                        ? () => _openAddonSheet(context, state)
                        : () {},
                  ),
                ),
                const SizedBox(height: 24),

                // Section ⑦ — Data Anda (inline form; keyboard flow)
                const _SectionHeader(number: 7, title: 'Data Anda'),
                const SizedBox(height: 8),
                KeyedSubtree(
                  key: _customerInfoSectionKey,
                  child: _roomExplicitlyChosen
                      ? _CustomerInfoSection(
                          branchId: widget.branchId,
                          formKey: _customerFormKey,
                          nameCtrl: _nameCtrl,
                          phoneCtrl: _phoneCtrl,
                          emailCtrl: _emailCtrl,
                          onChanged: () {
                            _revalidateCustomerInfo();
                            if (_customerInfoValid) {
                              WidgetsBinding.instance.addPostFrameCallback(
                                (_) => _scrollTo(_summarySectionKey),
                              );
                            }
                          },
                        )
                      : const _LockedSection(
                          icon: Icons.person_outline,
                          label: 'Isi data Anda...',
                          hint: 'Pilih ruangan terlebih dahulu',
                        ),
                ),
                const SizedBox(height: 24),

                // Section ⑧ — Ringkasan Booking (read-only; inline)
                const _SectionHeader(number: 8, title: 'Ringkasan Booking'),
                const SizedBox(height: 8),
                KeyedSubtree(
                  key: _summarySectionKey,
                  child: _customerInfoValid
                      ? _BookingSummarySection(
                          branch: widget.branch,
                          state: state,
                          totalPrice: totalPrice,
                        )
                      : const _LockedSection(
                          icon: Icons.receipt_long_outlined,
                          label: 'Ringkasan...',
                          hint: 'Lengkapi data Anda terlebih dahulu',
                        ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Therapist leading widget for trigger row
  // ---------------------------------------------------------------------------

  Widget _buildTherapistLeading(
    BuildContext context,
    bool confirmed,
    TherapistItem? therapist,
  ) {
    final cs = Theme.of(context).colorScheme;
    if (!confirmed) {
      return Icon(Icons.person_outline, size: 20, color: cs.onSurfaceVariant);
    }
    if (therapist == null) {
      // Otomatis
      return CircleAvatar(
        radius: 16,
        backgroundColor: cs.primaryContainer,
        child: Icon(Icons.auto_awesome, size: 16, color: cs.primary),
      );
    }
    if (therapist.photoUrl != null) {
      return CircleAvatar(
        radius: 16,
        backgroundImage: NetworkImage(therapist.photoUrl!),
        backgroundColor: cs.secondaryContainer,
      );
    }
    return CircleAvatar(
      radius: 16,
      backgroundColor: cs.secondaryContainer,
      child: Text(
        therapist.initials,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w700,
          color: cs.onSecondaryContainer,
        ),
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Sheet / picker openers
  // ---------------------------------------------------------------------------

  void _openServiceSheet(BuildContext context, BookingWizardState state) {
    _openSelectorSheet(
      ctx: context,
      title: 'Pilih Layanan',
      builder: (_) => _ServiceSheetContent(
        branchId: widget.branchId,
        services: widget.branch.services,
        state: state,
        onSelected: () {
          Navigator.of(context).pop();
          setState(() {
            _roomExplicitlyChosen = false;
            _customerInfoValid = false;
          });
          WidgetsBinding.instance.addPostFrameCallback(
            (_) => _scrollTo(_dateSectionKey),
          );
        },
      ),
    );
  }

  Future<void> _openDatePicker(
    BuildContext context,
    BookingWizardState state,
  ) async {
    final today = DateTime.now();
    final initial = state.selectedDate ?? today;
    final picked = await showDatePicker(
      context: context,
      initialDate: initial,
      firstDate: today,
      lastDate: today.add(const Duration(days: 30)),
    );
    if (picked != null && context.mounted) {
      ref
          .read(bookingWizardProvider(widget.branchId).notifier)
          .selectDate(picked);
    }
  }

  void _openTherapistSheet(BuildContext context, BookingWizardState state) {
    // Filter to therapists who actually perform the selected service.
    // service_ids comes from the public branch detail (backend populates it
    // from therapist↔service mappings). When empty (older payload or no
    // mapping), fall back to showing all therapists rather than zero.
    final selectedServiceId = state.selectedService;
    final allTherapists = widget.branch.therapists;
    final therapistsForService = selectedServiceId == null
        ? allTherapists
        : allTherapists.where((t) {
            if (t.serviceIds.isEmpty) return false;
            return t.serviceIds.contains(selectedServiceId);
          }).toList();
    _openSelectorSheet(
      ctx: context,
      title: 'Pilih Terapis',
      // Portrait carousel needs more vertical room than the default 0.6.
      initialChildSize: 0.92,
      maxChildSize: 0.95,
      builder: (_) => _TherapistSheetContent(
        branchId: widget.branchId,
        therapists: therapistsForService,
        state: state,
        onSelected: () {
          Navigator.of(context).pop();
          WidgetsBinding.instance.addPostFrameCallback(
            (_) => _scrollTo(_timeSectionKey),
          );
        },
      ),
    );
  }

  void _openTimeSheet(BuildContext context, BookingWizardState state) {
    final date = state.selectedDate ?? DateTime.now();
    _openSelectorSheet(
      ctx: context,
      title: 'Pilih Waktu',
      subtitle: DateFormatter.formatFullDate(date),
      builder: (scrollController) => _TimeSheetContent(
        branchId: widget.branchId,
        branch: widget.branch,
        state: state,
        date: date,
        scrollController: scrollController,
        onSelected: () {
          Navigator.of(context).pop();
          // Auto-pick "Otomatis" room so customer skips step 5 by default.
          // selectSlot has already nulled selectedRoomId via clearRoom; we just
          // mark the choice as explicit so steps 6/7 unlock immediately. Customer
          // can still tap step 5 to override with a specific room.
          final wasChosen = _roomExplicitlyChosen;
          if (!wasChosen) {
            setState(() => _roomExplicitlyChosen = true);
            _onCustomerSectionMounted();
          }
          WidgetsBinding.instance.addPostFrameCallback(
            (_) => _scrollTo(_addonSectionKey),
          );
        },
      ),
    );
  }

  void _openRoomSheet(BuildContext context, BookingWizardState state) {
    _openSelectorSheet(
      ctx: context,
      title: 'Pilih Ruangan',
      builder: (scrollController) => _RoomSheetContent(
        branchId: widget.branchId,
        rooms: widget.branch.rooms,
        state: state,
        scrollController: scrollController,
        onSelected: () {
          Navigator.of(context).pop();
          final wasChosen = _roomExplicitlyChosen;
          if (!wasChosen) {
            setState(() => _roomExplicitlyChosen = true);
            _onCustomerSectionMounted();
          }
          WidgetsBinding.instance.addPostFrameCallback(
            (_) => _scrollTo(_addonSectionKey),
          );
        },
      ),
    );
  }

  void _openAddonSheet(BuildContext context, BookingWizardState state) {
    _openSelectorSheet(
      ctx: context,
      title: 'Tambahan (Opsional)',
      builder: (scrollController) => _AddonSheetContent(
        branchId: widget.branchId,
        addons: widget.branch.addons,
        scrollController: scrollController,
        onDone: () => Navigator.of(context).pop(),
      ),
    );
  }

  // ---------------------------------------------------------------------------
  // Progress bar (6 mandatory steps)
  // ---------------------------------------------------------------------------

  double _progress(BookingWizardState s) {
    var done = 0;
    if (s.selectedService != null) done++;
    if (s.selectedDate != null) done++;
    if (s.therapistConfirmed) done++;
    if (s.selectedSlot != null) done++;
    if (_roomExplicitlyChosen) done++;
    if (_customerInfoValid) done++;
    return done / 6.0;
  }

  Future<void> _submitAndNavigate(
    BuildContext context,
    BookingWizardState state,
  ) async {
    if (!(_customerFormKey.currentState?.validate() ?? false)) return;

    setState(() => _submitting = true);

    final branch = widget.branch;
    final service = branch.services
        .where((s) => s.id == state.selectedService)
        .firstOrNull;

    final response = await ref
        .read(bookingSubmitProvider.notifier)
        .submit(state);

    if (mounted) setState(() => _submitting = false);

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
            onPressed: () {},
          ),
        ),
      );
      return;
    }

    final recentBooking = RecentBooking(
      code: response.code,
      branchName: branch.name,
      scheduledStart: response.scheduledStart,
      serviceName: service?.name ?? '',
      totalPriceIdr: response.totalPriceIdr,
    );
    await ref.read(recentBookingsProvider.notifier).add(recentBooking);

    ref.read(bookingWizardProvider(widget.branchId).notifier).reset();

    if (!context.mounted) return;

    final routeData = PaymentRouteData(
      code: response.code,
      totalPriceIdr: response.totalPriceIdr,
      scheduledStart: response.scheduledStart,
      scheduledEnd: response.scheduledEnd,
      branchName: branch.name,
      serviceName: service?.name ?? '',
      qrString: response.qrString,
      qrExpiresAt: response.qrExpiresAt,
      paymentReference: response.paymentReference,
    );

    unawaited(
      context.push(
        '/branches/${widget.branchId}/book/payment',
        extra: routeData,
      ),
    );
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
      return (err).message;
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
// SHARED WIDGET: _TriggerRow
// Uniform form-row component used by every selector section (①②③④⑤⑥).
// ---------------------------------------------------------------------------
//
// Shape:
//   ┌────────────────────────────────────────────┐
//   │  [icon]   Label / placeholder    [chevron] │  ← 56dp tall, rounded 12dp
//   │           (subtitle if value-set)          │
//   └────────────────────────────────────────────┘
//
// - Card: elevation 0, cs.surface, 1dp cs.outline border, 12dp radius.
// - Locked state (BS-T9 opacity 0.38, non-interactive).
// - Value selected: label = titleMedium semibold, subtitle = bodyMedium muted.
// - Trailing: chevron_right 24dp.

class _TriggerRow extends StatelessWidget {
  const _TriggerRow({
    this.icon,
    this.leading,
    required this.label,
    this.subtitle,
    required this.locked,
    required this.onTap,
  }) : assert(
         icon != null || leading != null,
         '_TriggerRow requires icon or leading',
       );

  /// Standard icon data. Mutually exclusive with [leading].
  final IconData? icon;

  /// Custom leading widget (e.g. therapist avatar). Overrides [icon].
  final Widget? leading;

  /// Primary text line. Shows value when selected, placeholder otherwise.
  final String label;

  /// Optional secondary line. Displayed when a value has been chosen.
  final String? subtitle;

  /// When true: opacity 0.38, pointer-events disabled, lock icon replaces chevron.
  final bool locked;

  /// Called on tap (no-op when locked).
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final hasSubtitle = subtitle != null && subtitle!.isNotEmpty;

    Widget inner = Card(
      elevation: 0,
      color: cs.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: cs.outline),
      ),
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: locked ? null : onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Row(
            children: [
              // Leading icon or widget
              SizedBox(
                width: 24,
                height: 24,
                child: Center(
                  child: leading ??
                      Icon(icon!, size: 20, color: cs.onSurfaceVariant),
                ),
              ),
              const SizedBox(width: 12),

              // Label + optional subtitle column
              Expanded(
                child: hasSubtitle
                    ? Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            label,
                            style: theme.textTheme.titleMedium?.copyWith(
                              fontWeight: FontWeight.w600,
                              color: cs.onSurface,
                            ),
                          ),
                          const SizedBox(height: 2),
                          Text(
                            subtitle!,
                            style: theme.textTheme.bodyMedium?.copyWith(
                              color: cs.onSurfaceVariant,
                            ),
                          ),
                        ],
                      )
                    : Text(
                        label,
                        style: theme.textTheme.bodyMedium?.copyWith(
                          color: cs.onSurfaceVariant,
                        ),
                      ),
              ),

              // Trailing
              Icon(
                locked ? Icons.lock_outline : Icons.chevron_right,
                size: locked ? 16 : 24,
                color: cs.onSurfaceVariant,
              ),
            ],
          ),
        ),
      ),
    );

    if (locked) {
      inner = Opacity(opacity: 0.38, child: IgnorePointer(child: inner));
    }

    return inner;
  }
}

// ---------------------------------------------------------------------------
// SHARED WIDGET: _SheetHeader
// Consistent header for all bottom-sheet selectors.
// ---------------------------------------------------------------------------
//
// Structure (top→bottom):
//   12dp gap
//   40×4dp grab handle (cs.outlineVariant, centered)
//   12dp gap
//   Row: title (titleLarge, left) + close IconButton (right)
//   [optional subtitle line if subtitle != null]
//   Divider 1dp cs.outlineVariant

class _SheetHeader extends StatelessWidget {
  const _SheetHeader({required this.title, this.subtitle});
  final String title;
  final String? subtitle;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Grab handle
        const SizedBox(height: 12),
        Center(
          child: Container(
            width: 40,
            height: 4,
            decoration: BoxDecoration(
              color: cs.outlineVariant,
              borderRadius: BorderRadius.circular(999),
            ),
          ),
        ),
        const SizedBox(height: 12),

        // Title row
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(title, style: theme.textTheme.titleLarge),
                    if (subtitle != null) ...[
                      const SizedBox(height: 2),
                      Text(
                        subtitle!,
                        style: theme.textTheme.bodyMedium?.copyWith(
                          color: cs.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
              IconButton(
                icon: const Icon(Icons.close),
                onPressed: () => Navigator.of(context).pop(),
                tooltip: 'Tutup',
              ),
            ],
          ),
        ),

        const Divider(height: 16),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// SHARED HELPER: _openSelectorSheet
// Wraps showModalBottomSheet + DraggableScrollableSheet for all sheet selectors.
// ---------------------------------------------------------------------------

void _openSelectorSheet({
  required BuildContext ctx,
  required String title,
  String? subtitle,
  required Widget Function(ScrollController scrollController) builder,
  double initialChildSize = 0.6,
  double maxChildSize = 0.92,
}) {
  showModalBottomSheet<void>(
    context: ctx,
    isScrollControlled: true,
    useSafeArea: true,
    backgroundColor: Theme.of(ctx).colorScheme.surface,
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(24)),
    ),
    builder: (_) => DraggableScrollableSheet(
      initialChildSize: initialChildSize,
      minChildSize: 0.4,
      maxChildSize: maxChildSize,
      expand: false,
      builder: (_, scrollController) {
        return Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _SheetHeader(title: title, subtitle: subtitle),
            Expanded(child: builder(scrollController)),
          ],
        );
      },
    ),
  );
}

// ---------------------------------------------------------------------------
// Section header
// ---------------------------------------------------------------------------

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({required this.number, required this.title});
  final int number;
  final String title;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    return Row(
      children: [
        Container(
          width: 22,
          height: 22,
          decoration: BoxDecoration(color: cs.primary, shape: BoxShape.circle),
          child: Center(
            child: Text(
              '$number',
              style: theme.textTheme.labelSmall?.copyWith(
                color: cs.onPrimary,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
        ),
        const SizedBox(width: 8),
        Text(
          title,
          style: theme.textTheme.titleMedium?.copyWith(
            fontWeight: FontWeight.w600,
          ),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Locked section placeholder (BS-T9)
// Used only for Sections ⑦ and ⑧ which are not trigger rows.
// ---------------------------------------------------------------------------

class _LockedSection extends StatelessWidget {
  const _LockedSection({
    required this.icon,
    required this.label,
    required this.hint,
  });
  final IconData icon;
  final String label;
  final String hint;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    return Semantics(
      label: '$label. Tersedia setelah langkah sebelumnya dipilih.',
      excludeSemantics: true,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Opacity(
            opacity: 0.38,
            child: Container(
              height: 56,
              decoration: BoxDecoration(
                color: cs.surface,
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: cs.outlineVariant),
              ),
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: Row(
                children: [
                  Icon(icon, size: 20, color: cs.onSurfaceVariant),
                  const SizedBox(width: 12),
                  Text(
                    label,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: cs.onSurfaceVariant,
                    ),
                  ),
                  const Spacer(),
                  Icon(Icons.lock_outline, size: 16, color: cs.outlineVariant),
                ],
              ),
            ),
          ),
          const SizedBox(height: 4),
          Text(
            hint,
            style: theme.textTheme.bodySmall?.copyWith(
              color: cs.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Section ① — Service sheet content
// ---------------------------------------------------------------------------

class _ServiceSheetContent extends ConsumerWidget {
  const _ServiceSheetContent({
    required this.branchId,
    required this.services,
    required this.state,
    required this.onSelected,
  });
  final String branchId;
  final List<ServiceItem> services;
  final BookingWizardState state;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    if (services.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Icon(Icons.info_outline, size: 20, color: cs.onSurfaceVariant),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                'Belum ada layanan tersedia di cabang ini.',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: cs.onSurfaceVariant,
                ),
              ),
            ),
          ],
        ),
      );
    }

    void selectService(String id) {
      ref.read(bookingWizardProvider(branchId).notifier).selectService(id);
      HapticFeedback.lightImpact();
      onSelected();
    }

    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(16, 0, 16, 24),
      child: Container(
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: cs.outlineVariant),
          color: cs.surface,
        ),
        clipBehavior: Clip.antiAlias,
        child: RadioGroup<String>(
          groupValue: state.selectedService,
          onChanged: (id) {
            if (id != null && id != state.selectedService) selectService(id);
          },
          child: Column(
            children: services.mapIndexed((index, service) {
              final isSelected = state.selectedService == service.id;
              final isLast = index == services.length - 1;
              final subtitle =
                  '${service.durationMinutes} menit'
                  '${service.category.isNotEmpty ? " · ${service.category}" : ""}';

              return Semantics(
                label: isSelected
                    ? '${service.name}, dipilih.'
                    : '${service.name}. Ketuk untuk memilih.',
                button: !isSelected,
                child: InkWell(
                  onTap: isSelected ? null : () => selectService(service.id),
                  child: AnimatedContainer(
                    duration: const Duration(milliseconds: 150),
                    color: isSelected
                        ? cs.primaryContainer.withAlpha(180)
                        : Colors.transparent,
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Padding(
                          padding: const EdgeInsets.symmetric(
                            vertical: 12,
                            horizontal: 16,
                          ),
                          child: Row(
                            children: [
                              SizedBox(
                                width: 24,
                                height: 24,
                                child: Radio<String>(
                                  value: service.id,
                                  materialTapTargetSize:
                                      MaterialTapTargetSize.shrinkWrap,
                                  visualDensity: VisualDensity.compact,
                                ),
                              ),
                              const SizedBox(width: 12),
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  mainAxisSize: MainAxisSize.min,
                                  children: [
                                    Text(
                                      service.name,
                                      style: theme.textTheme.titleMedium
                                          ?.copyWith(color: cs.onSurface),
                                    ),
                                    const SizedBox(height: 4),
                                    Text(
                                      subtitle,
                                      style:
                                          theme.textTheme.bodyMedium?.copyWith(
                                        color: cs.onSurfaceVariant,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                              const SizedBox(width: 8),
                              Text(
                                CurrencyFormatter.formatRupiah(service.priceIdr),
                                style: theme.textTheme.titleMedium?.copyWith(
                                  fontWeight: FontWeight.w700,
                                  color: cs.primary,
                                  fontFeatures: const [
                                    FontFeature.tabularFigures(),
                                  ],
                                ),
                              ),
                            ],
                          ),
                        ),
                        if (!isLast)
                          Divider(
                            height: 1,
                            thickness: 1,
                            color: cs.outlineVariant,
                          ),
                      ],
                    ),
                  ),
                ),
              );
            }).toList(),
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Section ③ — Therapist sheet content (carousel inside modal)
// ---------------------------------------------------------------------------

class _TherapistSheetContent extends ConsumerStatefulWidget {
  const _TherapistSheetContent({
    required this.branchId,
    required this.therapists,
    required this.state,
    required this.onSelected,
  });
  final String branchId;
  final List<TherapistItem> therapists;
  final BookingWizardState state;
  final VoidCallback onSelected;

  @override
  ConsumerState<_TherapistSheetContent> createState() =>
      _TherapistSheetContentState();
}

class _TherapistSheetContentState extends ConsumerState<_TherapistSheetContent>
    with SingleTickerProviderStateMixin {
  late PageController _pageController;
  int _currentPage = 0;
  late AnimationController _popController;
  late Animation<double> _popAnimation;

  // Index 0 = "Otomatis", 1..n = therapists.
  int get _totalCards => widget.therapists.length + 1;

  bool get _showArrows {
    if (kIsWeb) return true;
    final navMode = MediaQuery.of(context).navigationMode;
    return navMode == NavigationMode.directional;
  }

  @override
  void initState() {
    super.initState();
    final initial = _initialPage();
    _pageController = PageController(
      viewportFraction: _viewportFraction(),
      initialPage: initial,
    );
    _currentPage = initial;
    _pageController.addListener(_onScroll);

    _popController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 200),
    );
    _popAnimation = TweenSequence<double>([
      TweenSequenceItem(tween: Tween(begin: 1.0, end: 1.03), weight: 50),
      TweenSequenceItem(tween: Tween(begin: 1.03, end: 1.0), weight: 50),
    ]).animate(CurvedAnimation(parent: _popController, curve: Curves.easeOut));
  }

  int _initialPage() {
    if (!widget.state.therapistConfirmed) return 0;
    if (widget.state.selectedTherapistId == null) return 0;
    final idx = widget.therapists.indexWhere(
      (t) => t.id == widget.state.selectedTherapistId,
    );
    return idx >= 0 ? idx + 1 : 0;
  }

  double _viewportFraction() => 0.92;

  @override
  void dispose() {
    _pageController.removeListener(_onScroll);
    _pageController.dispose();
    _popController.dispose();
    super.dispose();
  }

  void _onScroll() {
    final page = _pageController.page?.round() ?? _currentPage;
    if (page != _currentPage) {
      setState(() => _currentPage = page);
    }
  }

  void _selectCard(int cardIndex) {
    final notifier = ref.read(bookingWizardProvider(widget.branchId).notifier);
    final therapistId = cardIndex == 0
        ? null
        : widget.therapists[cardIndex - 1].id;
    notifier.confirmTherapist(therapistId);
    HapticFeedback.lightImpact();
    _popController.forward(from: 0);
    widget.onSelected();
  }

  void _goToPage(int page) {
    _pageController.animateToPage(
      page,
      duration: const Duration(milliseconds: 250),
      curve: Curves.easeInOut,
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    if (widget.therapists.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.person_search_outlined,
              size: 64,
              color: cs.onSurfaceVariant,
            ),
            const SizedBox(height: 12),
            Text(
              'Cabang ini belum punya\ntherapist tersedia',
              textAlign: TextAlign.center,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: cs.onSurfaceVariant,
              ),
            ),
          ],
        ),
      );
    }

    final selectedId = widget.state.selectedTherapistId;
    final isCurrentSelected = _isCardSelected(_currentPage, selectedId);

    final screenWidth = MediaQuery.of(context).size.width;
    const peekWidth = 16.0;
    final fraction = 1.0 - (peekWidth / screenWidth.clamp(300, 600));

    if ((_pageController.viewportFraction - fraction).abs() > 0.01) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        final currentPage = _currentPage;
        _pageController.removeListener(_onScroll);
        _pageController.dispose();
        _pageController = PageController(
          viewportFraction: fraction,
          initialPage: currentPage,
        );
        _pageController.addListener(_onScroll);
        setState(() {});
      });
    }

    return Padding(
      padding: const EdgeInsets.fromLTRB(0, 0, 0, 16),
      child: Semantics(
        label:
            'Pilih terapis. ${_currentPage + 1} dari $_totalCards ditampilkan.',
        container: true,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Counter badge row
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  AnimatedSwitcher(
                    duration: const Duration(milliseconds: 100),
                    child: _CounterBadge(
                      key: ValueKey('$_currentPage-$isCurrentSelected'),
                      current: _currentPage + 1,
                      total: _totalCards,
                      isCurrentSelected: isCurrentSelected,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 6),

            // Carousel — portrait card so the therapist photo is the hero
            // (taller than wide, matching standard portrait headshot framing).
            SizedBox(
              height: 560,
              child: PageView.builder(
                controller: _pageController,
                itemCount: _totalCards,
                onPageChanged: (p) => setState(() => _currentPage = p),
                itemBuilder: (context, index) {
                  final isSelected = _isCardSelected(index, selectedId);
                  final therapist = index == 0
                      ? null
                      : widget.therapists[index - 1];
                  return AnimatedBuilder(
                    animation: _popAnimation,
                    builder: (_, child) {
                      final isSelectAnimTarget =
                          _popController.isAnimating && isSelected;
                      return Transform.scale(
                        scale: isSelectAnimTarget ? _popAnimation.value : 1.0,
                        child: child,
                      );
                    },
                    child: _TherapistCard(
                      therapist: therapist,
                      isSelected: isSelected,
                      therapistConfirmed: widget.state.therapistConfirmed,
                      onSelect: () => _selectCard(index),
                    ),
                  );
                },
              ),
            ),
            const SizedBox(height: 12),

            // Indicator dots + arrow buttons
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                if (_showArrows) ...[
                  Semantics(
                    label: 'Terapis sebelumnya',
                    button: true,
                    child: IconButton.filled(
                      style: IconButton.styleFrom(
                        backgroundColor: cs.primary,
                        foregroundColor: cs.onPrimary,
                        disabledBackgroundColor: cs.primary.withAlpha(60),
                        disabledForegroundColor: cs.onPrimary.withAlpha(180),
                        minimumSize: const Size(44, 44),
                      ),
                      icon: const Icon(Icons.chevron_left),
                      onPressed: _currentPage > 0
                          ? () => _goToPage(_currentPage - 1)
                          : null,
                    ),
                  ),
                  const SizedBox(width: 8),
                ],
                if (_totalCards <= 8)
                  SmoothPageIndicator(
                    controller: _pageController,
                    count: _totalCards,
                    effect: WormEffect(
                      dotHeight: 8,
                      dotWidth: 8,
                      activeDotColor: cs.primary,
                      dotColor: cs.outlineVariant,
                      spacing: 6,
                    ),
                  )
                else
                  SizedBox(
                    width: 160,
                    child: LinearProgressIndicator(
                      value: _totalCards > 1
                          ? _currentPage / (_totalCards - 1)
                          : 0,
                      minHeight: 4,
                      backgroundColor: cs.outlineVariant,
                      color: cs.primary,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                if (_showArrows) ...[
                  const SizedBox(width: 8),
                  Semantics(
                    label: 'Terapis berikutnya',
                    button: true,
                    child: IconButton.filled(
                      style: IconButton.styleFrom(
                        backgroundColor: cs.primary,
                        foregroundColor: cs.onPrimary,
                        disabledBackgroundColor: cs.primary.withAlpha(60),
                        disabledForegroundColor: cs.onPrimary.withAlpha(180),
                        minimumSize: const Size(44, 44),
                      ),
                      icon: const Icon(Icons.chevron_right),
                      onPressed: _currentPage < _totalCards - 1
                          ? () => _goToPage(_currentPage + 1)
                          : null,
                    ),
                  ),
                ],
              ],
            ),
          ],
        ),
      ),
    );
  }

  bool _isCardSelected(int cardIndex, String? selectedId) {
    if (!widget.state.therapistConfirmed) return false;
    if (cardIndex == 0) return selectedId == null;
    if (cardIndex > widget.therapists.length) return false;
    return widget.therapists[cardIndex - 1].id == selectedId;
  }
}

// ---------------------------------------------------------------------------
// Counter badge — BS-T11
// ---------------------------------------------------------------------------

class _CounterBadge extends StatelessWidget {
  const _CounterBadge({
    super.key,
    required this.current,
    required this.total,
    required this.isCurrentSelected,
  });
  final int current;
  final int total;
  final bool isCurrentSelected;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final bg = isCurrentSelected ? cs.primaryContainer : cs.primary;
    final fg = isCurrentSelected ? cs.onPrimaryContainer : cs.onPrimary;
    return Semantics(
      label: '$current dari $total terapis',
      excludeSemantics: true,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        decoration: BoxDecoration(
          color: bg,
          borderRadius: BorderRadius.circular(999),
        ),
        child: Text(
          '$current / $total',
          style: Theme.of(context).textTheme.labelSmall?.copyWith(
            color: fg,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Therapist card (including Otomatis when therapist == null)
// ---------------------------------------------------------------------------

class _TherapistCard extends StatelessWidget {
  const _TherapistCard({
    required this.therapist,
    required this.isSelected,
    required this.therapistConfirmed,
    required this.onSelect,
  });

  final TherapistItem? therapist;
  final bool isSelected;
  final bool therapistConfirmed;
  final VoidCallback onSelect;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final isAuto = therapist == null;
    final name = isAuto ? 'Otomatis' : therapist!.fullName;

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 4),
      child: Semantics(
        label: isSelected
            ? '${isAuto ? "Terapis otomatis" : "Terapis $name"}, dipilih.'
            : '${isAuto ? "Pilih otomatis" : "Terapis $name"}. '
                  'Belum dipilih. Pilih $name.',
        button: !isSelected,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 200),
          decoration: BoxDecoration(
            color: isSelected ? cs.primaryContainer : cs.surface,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(
              color: isSelected ? cs.primary : cs.outlineVariant,
              width: isSelected ? 2 : 1,
            ),
            boxShadow: isSelected
                ? [
                    BoxShadow(
                      color: cs.shadow.withAlpha(40),
                      blurRadius: 8,
                      offset: const Offset(0, 4),
                    ),
                  ]
                : null,
          ),
          clipBehavior: Clip.antiAlias,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              // Photo area — portrait 4:5-ish framing (taller than wide so a
              // full-body therapist photo is rendered correctly without
              // cropping the head/legs).
              SizedBox(
                height: 360,
                child: isAuto
                    ? _AutoPhotoArea(cs: cs, theme: theme)
                    : _TherapistPhotoArea(therapist: therapist!, cs: cs),
              ),

              // Body — remaining ~152dp
              Expanded(
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // Name row + selection indicator
                      Row(
                        children: [
                          Expanded(
                            child: Text(
                              name,
                              style: theme.textTheme.titleLarge?.copyWith(
                                fontWeight: FontWeight.w700,
                                color: isSelected
                                    ? cs.onPrimaryContainer
                                    : cs.onSurface,
                                overflow: TextOverflow.ellipsis,
                              ),
                              maxLines: 1,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Icon(
                            isSelected
                                ? Icons.check_circle
                                : Icons.radio_button_unchecked,
                            size: 24,
                            color: isSelected ? cs.primary : cs.outlineVariant,
                          ),
                        ],
                      ),

                      if (!isAuto) ...[
                        const SizedBox(height: 6),
                        _TherapistChipRow(therapist: therapist!, cs: cs),
                        if (therapist!.bio != null) ...[
                          const SizedBox(height: 6),
                          Text(
                            therapist!.bio!,
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: cs.onSurfaceVariant,
                            ),
                            maxLines: 2,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ],
                      ] else ...[
                        const SizedBox(height: 6),
                        Text(
                          'Therapist akan dipilih otomatis berdasarkan ketersediaan',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: isSelected
                                ? cs.onPrimaryContainer
                                : cs.onSurfaceVariant,
                          ),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ],

                      const Spacer(),

                      SizedBox(
                        width: double.infinity,
                        child: FilledButton(
                          style: FilledButton.styleFrom(
                            backgroundColor: cs.primary,
                            foregroundColor: cs.onPrimary,
                            disabledBackgroundColor: cs.primary,
                            disabledForegroundColor: cs.onPrimary,
                            minimumSize: const Size(double.infinity, 48),
                            textStyle: const TextStyle(
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                          onPressed: isSelected ? null : onSelect,
                          child: Text(
                            isSelected ? 'Terpilih ✓' : 'Pilih $name',
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _AutoPhotoArea extends StatelessWidget {
  const _AutoPhotoArea({required this.cs, required this.theme});
  final ColorScheme cs;
  final ThemeData theme;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: cs.primaryContainer,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(10)),
      ),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.auto_awesome, size: 64, color: cs.primary),
          const SizedBox(height: 12),
          Text(
            'Biar sistem yang memilih',
            style: theme.textTheme.bodyMedium?.copyWith(
              color: cs.onPrimaryContainer,
            ),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }
}

class _TherapistPhotoArea extends StatelessWidget {
  const _TherapistPhotoArea({required this.therapist, required this.cs});
  final TherapistItem therapist;
  final ColorScheme cs;

  @override
  Widget build(BuildContext context) {
    if (therapist.photoUrl != null) {
      return CachedNetworkImage(
        imageUrl: therapist.photoUrl!,
        fit: BoxFit.cover,
        imageBuilder: (_, img) => Container(
          decoration: BoxDecoration(
            borderRadius: const BorderRadius.vertical(top: Radius.circular(10)),
            image: DecorationImage(image: img, fit: BoxFit.cover),
          ),
        ),
        placeholder: (_, __) => _InitialsAvatar(
          initials: therapist.initials,
          cs: cs,
          isShimmer: true,
        ),
        errorWidget: (_, __, ___) => _PlaceholderTherapistArea(cs: cs),
      );
    }
    return _PlaceholderTherapistArea(cs: cs);
  }
}

/// Watercolor silhouette shown as card background when a therapist has no
/// photo. Replaces the initials-avatar fallback per design feedback.
class _PlaceholderTherapistArea extends StatelessWidget {
  const _PlaceholderTherapistArea({required this.cs});
  final ColorScheme cs;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: cs.secondaryContainer,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(10)),
        image: const DecorationImage(
          image: AssetImage('assets/images/therapist_placeholder.png'),
          fit: BoxFit.contain,
        ),
      ),
    );
  }
}

class _InitialsAvatar extends StatefulWidget {
  const _InitialsAvatar({
    required this.initials,
    required this.cs,
    this.isShimmer = false,
  });
  final String initials;
  final ColorScheme cs;
  final bool isShimmer;

  @override
  State<_InitialsAvatar> createState() => _InitialsAvatarState();
}

class _InitialsAvatarState extends State<_InitialsAvatar>
    with SingleTickerProviderStateMixin {
  late AnimationController _shimmer;

  @override
  void initState() {
    super.initState();
    _shimmer = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1400),
    );
    if (widget.isShimmer) _shimmer.repeat(reverse: true);
  }

  @override
  void dispose() {
    _shimmer.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final cs = widget.cs;
    final disableAnim =
        MediaQuery.of(context).disableAnimations || !widget.isShimmer;
    return Container(
      decoration: BoxDecoration(
        color: cs.secondaryContainer,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(10)),
      ),
      child: disableAnim
          ? Center(
              child: Text(
                widget.initials,
                style: Theme.of(context).textTheme.headlineLarge?.copyWith(
                  fontWeight: FontWeight.w700,
                  color: cs.onSecondaryContainer,
                ),
              ),
            )
          : AnimatedBuilder(
              animation: _shimmer,
              builder: (_, child) {
                return Opacity(
                  opacity: 0.4 + 0.6 * _shimmer.value,
                  child: Container(color: cs.secondaryContainer),
                );
              },
            ),
    );
  }
}

class _TherapistChipRow extends StatelessWidget {
  const _TherapistChipRow({required this.therapist, required this.cs});
  final TherapistItem therapist;
  final ColorScheme cs;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final chips = <String>[
      if (therapist.genderLabel != null) therapist.genderLabel!,
      if (therapist.build != null && therapist.build!.isNotEmpty)
        _capitalize(therapist.build!),
      if (therapist.heightCm != null) '${therapist.heightCm} cm',
      if (therapist.weightKg != null) '${therapist.weightKg} kg',
    ];
    if (chips.isEmpty) return const SizedBox.shrink();
    return Wrap(
      spacing: 6,
      runSpacing: 4,
      children: chips
          .map(
            (c) => Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
              decoration: BoxDecoration(
                color: cs.secondaryContainer,
                borderRadius: BorderRadius.circular(999),
              ),
              child: Text(
                c,
                style: theme.textTheme.labelSmall?.copyWith(
                  color: cs.onSecondaryContainer,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ),
          )
          .toList(),
    );
  }

  String _capitalize(String s) =>
      s.isEmpty ? s : s[0].toUpperCase() + s.substring(1);
}

// ---------------------------------------------------------------------------
// Section ④ — Time sheet content
// ---------------------------------------------------------------------------

class _TimeSheetContent extends ConsumerWidget {
  const _TimeSheetContent({
    required this.branchId,
    required this.branch,
    required this.state,
    required this.date,
    required this.scrollController,
    required this.onSelected,
  });
  final String branchId;
  final BranchDetail branch;
  final BookingWizardState state;
  final DateTime date;
  final ScrollController scrollController;
  final VoidCallback onSelected;

  static String _fmtDate(DateTime d) =>
      '${d.year.toString().padLeft(4, '0')}-'
      '${d.month.toString().padLeft(2, '0')}-'
      '${d.day.toString().padLeft(2, '0')}';

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final dateStr = _fmtDate(date);
    final serviceId = state.selectedService ?? '';
    final therapistId =
        (state.therapistConfirmed && state.selectedTherapistId != null)
        ? state.selectedTherapistId
        : null;

    final slotsAsync = serviceId.isNotEmpty
        ? ref.watch(
            availabilityProvider(
              branchId: branch.id,
              serviceId: serviceId,
              date: dateStr,
              therapistId: therapistId,
            ),
          )
        : null;

    if (slotsAsync == null) {
      return Padding(
        padding: const EdgeInsets.all(16),
        child: Text(
          'Pilih layanan terlebih dahulu.',
          style: theme.textTheme.bodyMedium?.copyWith(
            color: cs.onSurfaceVariant,
          ),
        ),
      );
    }

    return slotsAsync.when(
      loading: () => Padding(
        padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
        child: _SlotGridSkeleton(cs: cs),
      ),
      error: (_, __) => Padding(
        padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
        child: _SlotErrorStrip(
          onRetry: () => ref.invalidate(
            availabilityProvider(
              branchId: branch.id,
              serviceId: serviceId,
              date: dateStr,
              therapistId: therapistId,
            ),
          ),
        ),
      ),
      data: (slots) {
        if (slots.isEmpty) {
          return Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const SizedBox(height: 32),
              Icon(
                Icons.event_busy_outlined,
                size: 48,
                color: cs.onSurfaceVariant,
              ),
              const SizedBox(height: 8),
              Text(
                'Belum ada jadwal di tanggal ini',
                textAlign: TextAlign.center,
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: cs.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 32),
            ],
          );
        }

        return SingleChildScrollView(
          controller: scrollController,
          padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
          child: _SlotGrid(
            slots: slots,
            selectedSlot: state.selectedSlot,
            therapistId: therapistId,
            branchId: branchId,
            onSelected: () {
              HapticFeedback.lightImpact();
              onSelected();
            },
          ),
        );
      },
    );
  }
}

// ---------------------------------------------------------------------------
// Slot grid (shared by time sheet context)
// ---------------------------------------------------------------------------

class _SlotGrid extends ConsumerWidget {
  const _SlotGrid({
    required this.slots,
    required this.selectedSlot,
    required this.therapistId,
    required this.branchId,
    required this.onSelected,
  });
  final List<AvailabilitySlot> slots;
  final AvailabilitySlot? selectedSlot;
  final String? therapistId;
  final String branchId;
  final VoidCallback onSelected;

  bool _isDisabled(AvailabilitySlot slot) {
    if (slot.therapistsAvailableCount == 0) return true;
    if (therapistId != null && slot.therapistAvailable == false) return true;
    return false;
  }

  String _disabledReason(AvailabilitySlot slot) {
    if (slot.therapistsAvailableCount == 0) {
      return 'Tidak ada therapist tersedia';
    }
    if (therapistId != null && slot.therapistAvailable == false) {
      return 'Terapis sudah dibooking di jam ini';
    }
    return '';
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    String? legendReason;
    for (final s in slots) {
      if (_isDisabled(s)) {
        legendReason = _disabledReason(s);
        break;
      }
    }

    final screenWidth = MediaQuery.of(context).size.width;
    final crossAxisCount = screenWidth < 360 ? 2 : 3;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        GridView.count(
          crossAxisCount: crossAxisCount,
          childAspectRatio: 2.2,
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          mainAxisSpacing: 8,
          crossAxisSpacing: 8,
          children: slots.map((slot) {
            final isSelected = selectedSlot?.start == slot.start;
            final disabled = _isDisabled(slot);
            final timeLabel = DateFormatter.formatTime(slot.start);

            return Semantics(
              label: disabled
                  ? 'Slot $timeLabel, tidak tersedia'
                  : (isSelected
                        ? 'Slot $timeLabel, terpilih'
                        : 'Slot $timeLabel, tersedia'),
              button: !disabled,
              excludeSemantics: true,
              child: IgnorePointer(
                ignoring: disabled,
                child: Opacity(
                  opacity: disabled ? 0.38 : 1.0,
                  child: GestureDetector(
                    onTap: disabled
                        ? null
                        : () {
                            ref
                                .read(
                                  bookingWizardProvider(branchId).notifier,
                                )
                                .selectSlot(slot);
                            onSelected();
                          },
                    child: AnimatedContainer(
                      duration: const Duration(milliseconds: 150),
                      decoration: BoxDecoration(
                        color: isSelected
                            ? cs.primary
                            : (disabled
                                  ? cs.surfaceContainerHighest
                                  : cs.surface),
                        borderRadius: BorderRadius.circular(8),
                        border: Border.all(
                          color: isSelected
                              ? cs.primary
                              : cs.outlineVariant.withAlpha(
                                  disabled ? 97 : 255,
                                ),
                          width: isSelected ? 2 : 1,
                        ),
                      ),
                      child: Center(
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            if (isSelected) ...[
                              Icon(
                                Icons.check,
                                size: 14,
                                color: cs.onPrimary,
                              ),
                              const SizedBox(width: 4),
                            ],
                            Text(
                              timeLabel,
                              style: theme.textTheme.labelLarge?.copyWith(
                                fontWeight: FontWeight.w600,
                                color: isSelected
                                    ? cs.onPrimary
                                    : cs.onSurface.withAlpha(
                                        disabled ? 97 : 255,
                                      ),
                                fontFeatures: const [
                                  FontFeature.tabularFigures(),
                                ],
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            );
          }).toList(),
        ),
        if (legendReason != null) ...[
          const SizedBox(height: 6),
          Text(
            'Slot redup: $legendReason.',
            style: theme.textTheme.bodySmall?.copyWith(
              color: cs.onSurfaceVariant,
            ),
          ),
        ],
      ],
    );
  }
}

class _SlotGridSkeleton extends StatelessWidget {
  const _SlotGridSkeleton({required this.cs});
  final ColorScheme cs;

  @override
  Widget build(BuildContext context) {
    return GridView.count(
      crossAxisCount: 3,
      childAspectRatio: 2.2,
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      mainAxisSpacing: 8,
      crossAxisSpacing: 8,
      children: List.generate(
        9,
        (_) => Container(
          decoration: BoxDecoration(
            color: cs.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(8),
          ),
        ),
      ),
    );
  }
}

class _SlotErrorStrip extends StatelessWidget {
  const _SlotErrorStrip({required this.onRetry});
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: cs.errorContainer,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Icon(Icons.wifi_off_outlined, size: 18, color: cs.error),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              'Gagal memuat slot.',
              style: theme.textTheme.bodySmall?.copyWith(
                color: cs.onErrorContainer,
              ),
            ),
          ),
          TextButton(
            onPressed: onRetry,
            child: Text(
              'Coba Lagi',
              style: TextStyle(color: cs.onErrorContainer),
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Section ⑤ — Room sheet content
// ---------------------------------------------------------------------------

class _RoomSheetContent extends ConsumerWidget {
  const _RoomSheetContent({
    required this.branchId,
    required this.rooms,
    required this.state,
    required this.scrollController,
    required this.onSelected,
  });
  final String branchId;
  final List<RoomItem> rooms;
  final BookingWizardState state;
  final ScrollController scrollController;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final selectedSlot = state.selectedSlot;
    final availableRoomIds = selectedSlot?.availableRoomIds ?? [];

    if (rooms.isEmpty) {
      return SingleChildScrollView(
        controller: scrollController,
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            Icon(
              Icons.meeting_room_outlined,
              size: 48,
              color: cs.onSurfaceVariant,
            ),
            const SizedBox(height: 8),
            Text(
              'Belum ada ruangan terdaftar di cabang ini.\n'
              'Hubungi cabang untuk informasi lebih lanjut.',
              textAlign: TextAlign.center,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: cs.onSurfaceVariant,
              ),
            ),
          ],
        ),
      );
    }

    return SingleChildScrollView(
      controller: scrollController,
      padding: const EdgeInsets.fromLTRB(16, 0, 16, 24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // "Otomatis" room card
          _RoomCard(
            room: null,
            isSelected: state.selectedRoomId == null,
            isDisabled: false,
            branchId: branchId,
            onSelected: onSelected,
          ),
          const SizedBox(height: 8),
          ...rooms.expand((room) {
            final isAvailable = availableRoomIds.contains(room.id);
            final isSelected = state.selectedRoomId == room.id;
            return [
              _RoomCard(
                room: room,
                isSelected: isSelected,
                isDisabled: !isAvailable,
                branchId: branchId,
                onSelected: onSelected,
              ),
              if (!isAvailable)
                Padding(
                  padding: const EdgeInsets.only(left: 16, bottom: 4),
                  child: Text(
                    'Sudah dipakai di slot ini',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: cs.error,
                    ),
                  ),
                ),
              const SizedBox(height: 8),
            ];
          }),
        ],
      ),
    );
  }
}

class _RoomCard extends ConsumerWidget {
  const _RoomCard({
    required this.room,
    required this.isSelected,
    required this.isDisabled,
    required this.branchId,
    required this.onSelected,
  });

  final RoomItem? room;
  final bool isSelected;
  final bool isDisabled;
  final String branchId;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final isAuto = room == null;
    final name = isAuto ? 'Otomatis' : room!.name;

    final semanticsLabel = isAuto
        ? 'Pilih ruangan otomatis. ${isSelected ? "Dipilih." : "Belum dipilih."}'
        : '${room!.name}, ${room!.roomTypeLabel}, kapasitas ${room!.capacity}. '
              '${room!.amenities.isNotEmpty ? "${room!.amenities.join(", ")}. " : ""}'
              '${isDisabled ? "Sudah terisi di jam ini." : (isSelected ? "Dipilih." : "Tersedia.")}';

    Widget card = Semantics(
      label: semanticsLabel,
      button: !isDisabled,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 150),
        decoration: BoxDecoration(
          color: isSelected ? cs.primaryContainer : cs.surface,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: isSelected ? cs.primary : cs.outlineVariant,
            width: isSelected ? 2 : 1,
          ),
          boxShadow: isSelected
              ? [
                  BoxShadow(
                    color: cs.shadow.withAlpha(40),
                    blurRadius: 8,
                    offset: const Offset(0, 4),
                  ),
                ]
              : null,
        ),
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            borderRadius: BorderRadius.circular(12),
            onTap: isDisabled
                ? null
                : () {
                    ref
                        .read(bookingWizardProvider(branchId).notifier)
                        .selectRoom(room?.id);
                    HapticFeedback.lightImpact();
                    onSelected();
                  },
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Row(
                children: [
                  // Photo / placeholder — 64×64
                  ClipRRect(
                    borderRadius: BorderRadius.circular(8),
                    child: SizedBox(
                      width: 64,
                      height: 64,
                      child: isAuto
                          ? Container(
                              color: cs.primaryContainer,
                              child: Icon(
                                Icons.king_bed_outlined,
                                size: 32,
                                color: cs.primary,
                              ),
                            )
                          : (room!.photoUrl != null
                                ? CachedNetworkImage(
                                    imageUrl: room!.photoUrl!,
                                    fit: BoxFit.cover,
                                    errorWidget: (_, __, ___) =>
                                        _roomPlaceholder(cs),
                                  )
                                : _roomPlaceholder(cs)),
                    ),
                  ),
                  const SizedBox(width: 12),

                  // Text column
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          name,
                          style: theme.textTheme.titleMedium?.copyWith(
                            fontWeight: FontWeight.w600,
                            color: isSelected
                                ? cs.onPrimaryContainer
                                : cs.onSurface,
                          ),
                        ),
                        if (isAuto) ...[
                          const SizedBox(height: 4),
                          Text(
                            'Kami pilihkan ruangan yang tersedia untukmu.',
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: cs.onSurfaceVariant,
                            ),
                            maxLines: 2,
                          ),
                        ] else ...[
                          const SizedBox(height: 4),
                          Row(
                            children: [
                              _SmallChip(room!.roomTypeLabel, cs: cs),
                              const SizedBox(width: 6),
                              Text(
                                'Kapasitas: ${room!.capacity}',
                                style: theme.textTheme.bodySmall?.copyWith(
                                  color: cs.onSurfaceVariant,
                                ),
                              ),
                            ],
                          ),
                          if (room!.amenities.isNotEmpty) ...[
                            const SizedBox(height: 4),
                            _AmenityChips(amenities: room!.amenities, cs: cs),
                          ],
                        ],
                      ],
                    ),
                  ),
                  const SizedBox(width: 8),

                  // Trailing indicator
                  if (isSelected)
                    Icon(Icons.check_circle, color: cs.primary, size: 24)
                  else if (isDisabled)
                    Icon(
                      Icons.block_outlined,
                      color: cs.onSurfaceVariant,
                      size: 20,
                    )
                  else
                    const SizedBox(width: 24),
                ],
              ),
            ),
          ),
        ),
      ),
    );

    if (isDisabled) {
      card = Opacity(opacity: 0.38, child: IgnorePointer(child: card));
    }
    return card;
  }

  Widget _roomPlaceholder(ColorScheme cs) => Container(
    color: cs.surfaceContainerHighest,
    child: Icon(
      Icons.meeting_room_outlined,
      size: 32,
      color: cs.onSurfaceVariant,
    ),
  );
}

// ---------------------------------------------------------------------------
// Section ⑥ — Add-on sheet content (multi-select; stays open until "Selesai")
// ---------------------------------------------------------------------------

class _AddonSheetContent extends ConsumerWidget {
  const _AddonSheetContent({
    required this.branchId,
    required this.addons,
    required this.scrollController,
    required this.onDone,
  });
  final String branchId;
  final List<AddonItem> addons;
  final ScrollController scrollController;
  final VoidCallback onDone;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    // Watch provider directly so toggling a checkbox rebuilds the sheet.
    // (Receiving state as a snapshot from the parent makes the modal stale —
    // toggleAddon updates provider state but the sheet doesn't rebuild.)
    final state = ref.watch(bookingWizardProvider(branchId));

    if (addons.isEmpty) {
      return Column(
        children: [
          Expanded(
            child: SingleChildScrollView(
              controller: scrollController,
              padding: const EdgeInsets.all(16),
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 14,
                ),
                decoration: BoxDecoration(
                  color: cs.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Row(
                  children: [
                    Icon(
                      Icons.check_circle_outline,
                      size: 20,
                      color: cs.primary,
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        'Tidak ada tambahan tersedia untuk cabang ini — '
                        'Anda dapat melanjutkan tanpa tambahan.',
                        style: theme.textTheme.bodyMedium?.copyWith(
                          color: cs.onSurfaceVariant,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
          _AddonDoneButton(onDone: onDone),
        ],
      );
    }

    return Column(
      children: [
        Expanded(
          child: SingleChildScrollView(
            controller: scrollController,
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
            child: Card(
              elevation: 0,
              color: cs.surfaceContainer,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(16),
              ),
              child: Column(
                children: addons.map((addon) {
                  final checked = state.selectedAddonIds.contains(addon.id);
                  return CheckboxListTile(
                    title: Text(
                      addon.name,
                      style: theme.textTheme.bodyLarge,
                    ),
                    subtitle: addon.description != null
                        ? Text(
                            addon.description!,
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: cs.onSurfaceVariant,
                            ),
                          )
                        : null,
                    secondary: Text(
                      CurrencyFormatter.formatRupiah(addon.priceIdr),
                      style: theme.textTheme.labelLarge?.copyWith(
                        fontWeight: FontWeight.w700,
                        color: cs.primary,
                        fontFeatures: const [FontFeature.tabularFigures()],
                      ),
                    ),
                    value: checked,
                    activeColor: cs.primary,
                    checkColor: cs.onPrimary,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                    onChanged: (_) => ref
                        .read(bookingWizardProvider(branchId).notifier)
                        .toggleAddon(addon.id),
                  );
                }).toList(),
              ),
            ),
          ),
        ),
        _AddonDoneButton(onDone: onDone),
      ],
    );
  }
}

/// Full-width "Selesai" button pinned at the bottom of the addon sheet.
class _AddonDoneButton extends StatelessWidget {
  const _AddonDoneButton({required this.onDone});
  final VoidCallback onDone;

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
        child: SizedBox(
          width: double.infinity,
          child: FilledButton(
            onPressed: onDone,
            child: const Text('Selesai'),
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Helper widgets shared by room cards
// ---------------------------------------------------------------------------

class _SmallChip extends StatelessWidget {
  const _SmallChip(this.label, {required this.cs});
  final String label;
  final ColorScheme cs;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: cs.secondaryContainer,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        label,
        style: Theme.of(context).textTheme.labelSmall?.copyWith(
          color: cs.onSecondaryContainer,
          fontWeight: FontWeight.w500,
        ),
      ),
    );
  }
}

class _AmenityChips extends StatelessWidget {
  const _AmenityChips({required this.amenities, required this.cs});
  final List<String> amenities;
  final ColorScheme cs;

  @override
  Widget build(BuildContext context) {
    const maxVisible = 3;
    final visible = amenities.take(maxVisible).toList();
    final overflow = amenities.length - maxVisible;
    return Wrap(
      spacing: 6,
      runSpacing: 4,
      children: [
        ...visible.map((a) => _SmallChip(a, cs: cs)),
        if (overflow > 0) _SmallChip('+$overflow', cs: cs),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Section ⑦ — Customer info form (BS-T9 locked until room is chosen)
// ---------------------------------------------------------------------------

class _CustomerInfoSection extends ConsumerWidget {
  const _CustomerInfoSection({
    required this.branchId,
    required this.formKey,
    required this.nameCtrl,
    required this.phoneCtrl,
    required this.emailCtrl,
    required this.onChanged,
  });

  final String branchId;
  final GlobalKey<FormState> formKey;
  final TextEditingController nameCtrl;
  final TextEditingController phoneCtrl;
  final TextEditingController emailCtrl;
  final VoidCallback onChanged;

  static final _emailRegex = RegExp(r'^[^\s@]+@[^\s@]+\.[^\s@]+$');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final notifier = ref.read(bookingWizardProvider(branchId).notifier);

    final fieldBorder = OutlineInputBorder(
      borderRadius: BorderRadius.circular(12),
      borderSide: BorderSide(color: cs.outline),
    );
    final focusBorder = OutlineInputBorder(
      borderRadius: BorderRadius.circular(12),
      borderSide: BorderSide(color: cs.primary, width: 2),
    );
    final errorBorder = OutlineInputBorder(
      borderRadius: BorderRadius.circular(12),
      borderSide: BorderSide(color: cs.error),
    );
    final fieldDecoration = InputDecoration(
      border: fieldBorder,
      enabledBorder: fieldBorder,
      focusedBorder: focusBorder,
      errorBorder: errorBorder,
      focusedErrorBorder: errorBorder,
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16),
    );

    return Card(
      elevation: 0,
      color: cs.surfaceContainer,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Form(
          key: formKey,
          autovalidateMode: AutovalidateMode.onUserInteraction,
          onChanged: () {
            notifier.setCustomerName(nameCtrl.text);
            notifier.setCustomerPhone(phoneCtrl.text);
            notifier.setCustomerEmail(emailCtrl.text);
            onChanged();
          },
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              TextFormField(
                controller: nameCtrl,
                keyboardType: TextInputType.name,
                textCapitalization: TextCapitalization.words,
                decoration: fieldDecoration.copyWith(labelText: 'Nama Lengkap'),
                validator: (v) {
                  final t = v?.trim() ?? '';
                  if (t.isEmpty) return 'Nama wajib diisi';
                  if (t.length > 200) return 'Nama terlalu panjang';
                  return null;
                },
              ),
              const SizedBox(height: 16),

              TextFormField(
                controller: phoneCtrl,
                keyboardType: TextInputType.phone,
                inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                decoration: fieldDecoration.copyWith(labelText: 'Nomor HP'),
                validator: (v) {
                  final t = v?.trim() ?? '';
                  if (t.isEmpty) return 'Nomor HP wajib diisi';
                  if (t.length < 5) return 'Nomor HP minimal 5 digit';
                  if (t.length > 30) return 'Nomor HP terlalu panjang';
                  return null;
                },
              ),
              const SizedBox(height: 16),

              TextFormField(
                controller: emailCtrl,
                keyboardType: TextInputType.emailAddress,
                autocorrect: false,
                decoration: fieldDecoration.copyWith(labelText: 'Email'),
                validator: (v) {
                  final t = v?.trim() ?? '';
                  if (t.isEmpty) return 'Email wajib diisi';
                  if (!_emailRegex.hasMatch(t)) return 'Format email tidak valid';
                  if (t.length > 320) return 'Email terlalu panjang';
                  return null;
                },
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Section ⑧ — Booking summary card (locked until customer info valid)
// ---------------------------------------------------------------------------

class _BookingSummarySection extends StatelessWidget {
  const _BookingSummarySection({
    required this.branch,
    required this.state,
    required this.totalPrice,
  });

  final BranchDetail branch;
  final BookingWizardState state;
  final int totalPrice;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    final service = branch.services
        .where((s) => s.id == state.selectedService)
        .firstOrNull;
    final selectedAddons = branch.addons
        .where((a) => state.selectedAddonIds.contains(a.id))
        .toList();
    final therapist = state.selectedTherapistId != null
        ? branch.therapists
              .where((t) => t.id == state.selectedTherapistId)
              .firstOrNull
        : null;
    final room = state.selectedRoomId != null
        ? branch.rooms.where((r) => r.id == state.selectedRoomId).firstOrNull
        : null;

    return Card(
      elevation: 0,
      color: cs.surfaceContainer,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _SummaryRow(
              label: 'Cabang',
              value: branch.name,
              theme: theme,
              cs: cs,
            ),
            const SizedBox(height: 10),
            _SummaryRow(
              label: 'Layanan',
              value: service?.name ?? '-',
              theme: theme,
              cs: cs,
            ),
            if (selectedAddons.isNotEmpty) ...[
              const SizedBox(height: 10),
              _SummaryRow(
                label: 'Add-on',
                value: selectedAddons.map((a) => a.name).join('\n'),
                theme: theme,
                cs: cs,
              ),
            ],
            if (state.selectedSlot != null) ...[
              const SizedBox(height: 10),
              _SummaryRow(
                label: 'Tanggal',
                value: DateFormatter.formatDate(state.selectedSlot!.start),
                theme: theme,
                cs: cs,
              ),
              const SizedBox(height: 10),
              _SummaryRow(
                label: 'Waktu',
                value: DateFormatter.formatSlotRange(
                  state.selectedSlot!.start,
                  state.selectedSlot!.end,
                ),
                theme: theme,
                cs: cs,
              ),
            ],
            const SizedBox(height: 10),
            _SummaryRow(
              label: 'Terapis',
              value: therapist?.fullName ?? 'Otomatis (Sistem yang memilih)',
              theme: theme,
              cs: cs,
            ),
            const SizedBox(height: 10),
            _SummaryRow(
              label: 'Ruangan',
              value: room?.name ?? 'Otomatis',
              theme: theme,
              cs: cs,
            ),
            const SizedBox(height: 12),
            const Divider(),
            const SizedBox(height: 12),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  'Total',
                  style: theme.textTheme.titleLarge?.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
                ),
                Text(
                  CurrencyFormatter.formatRupiah(totalPrice),
                  style: theme.textTheme.titleLarge?.copyWith(
                    fontWeight: FontWeight.w700,
                    color: cs.primary,
                    fontFeatures: const [FontFeature.tabularFigures()],
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _SummaryRow extends StatelessWidget {
  const _SummaryRow({
    required this.label,
    required this.value,
    required this.theme,
    required this.cs,
  });

  final String label;
  final String value;
  final ThemeData theme;
  final ColorScheme cs;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 72,
          child: Text(
            label,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: cs.onSurfaceVariant,
            ),
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            value,
            style: theme.textTheme.bodyMedium,
            textAlign: TextAlign.end,
            overflow: TextOverflow.ellipsis,
            maxLines: 3,
          ),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Sticky CTA bar
// ---------------------------------------------------------------------------

class _CtaBar extends StatelessWidget {
  const _CtaBar({
    required this.totalPrice,
    required this.hasAddons,
    required this.enabled,
    required this.onTap,
    this.isLoading = false,
  });
  final int totalPrice;
  final bool hasAddons;
  final bool enabled;
  final VoidCallback? onTap;
  final bool isLoading;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    return Material(
      elevation: 8,
      color: cs.surface,
      child: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    AnimatedDefaultTextStyle(
                      duration: const Duration(milliseconds: 200),
                      style:
                          (enabled
                              ? theme.textTheme.titleMedium?.copyWith(
                                  fontWeight: FontWeight.w700,
                                  color: cs.primary,
                                  fontFeatures: const [
                                    FontFeature.tabularFigures(),
                                  ],
                                )
                              : theme.textTheme.bodyMedium?.copyWith(
                                  color: cs.onSurfaceVariant,
                                  fontFeatures: const [
                                    FontFeature.tabularFigures(),
                                  ],
                                )) ??
                          const TextStyle(),
                      child: Text(CurrencyFormatter.formatRupiah(totalPrice)),
                    ),
                    if (hasAddons)
                      Text(
                        'Termasuk tambahan',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: cs.onSurfaceVariant,
                        ),
                      ),
                  ],
                ),
              ),
              const SizedBox(width: 12),

              SizedBox(
                height: 56,
                child: ElevatedButton(
                  onPressed: onTap,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: cs.primary,
                    foregroundColor: cs.onPrimary,
                    disabledBackgroundColor: cs.primary.withAlpha(80),
                    disabledForegroundColor: cs.onPrimary.withAlpha(180),
                    elevation: 0,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                    textStyle: const TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w700,
                    ),
                    minimumSize: const Size(180, 56),
                  ),
                  child: isLoading
                      ? const SizedBox(
                          width: 20,
                          height: 20,
                          child: CircularProgressIndicator.adaptive(
                            strokeWidth: 2,
                          ),
                        )
                      : const Text('Lanjut ke Pembayaran'),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Iterable extension — mapIndexed helper
// ---------------------------------------------------------------------------

extension _IterableIndexed<T> on Iterable<T> {
  Iterable<R> mapIndexed<R>(R Function(int index, T element) f) sync* {
    var i = 0;
    for (final e in this) {
      yield f(i++, e);
    }
  }
}
