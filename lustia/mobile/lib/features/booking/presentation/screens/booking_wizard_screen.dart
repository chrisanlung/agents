// Layar wizard booking multi-langkah (BK-A5..A9).
// Satu PageController mengelola semua langkah dalam satu Scaffold.

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:smooth_page_indicator/smooth_page_indicator.dart';

import '../../../../shared/utils/currency_formatter.dart';
import '../../../../shared/utils/date_formatter.dart';
import '../../../../shared/widgets/error_view.dart';
import '../../../branch/data/branch_model.dart';
import '../../../branch/presentation/providers/branch_detail_provider.dart';
import '../../data/booking_model.dart';
import '../providers/availability_provider.dart';
import '../providers/booking_provider.dart';

class BookingWizardScreen extends ConsumerStatefulWidget {
  const BookingWizardScreen({super.key, required this.branchId});
  final String branchId;

  @override
  ConsumerState<BookingWizardScreen> createState() =>
      _BookingWizardScreenState();
}

class _BookingWizardScreenState extends ConsumerState<BookingWizardScreen> {
  late final PageController _pageController;
  int _currentStep = 0;
  List<int> _visibleSteps = const [0, 1, 2, 3, 4, 5, 6];

  @override
  void initState() {
    super.initState();
    _pageController = PageController();
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  void _nextStep() {
    final effectiveTotal = _visibleSteps.length;
    if (_currentStep < effectiveTotal - 1) {
      setState(() => _currentStep++);
      _pageController.animateToPage(
        _currentStep,
        duration: const Duration(milliseconds: 225),
        curve: Curves.easeOut,
      );
    }
  }

  void _prevStep() {
    if (_currentStep > 0) {
      setState(() => _currentStep--);
      _pageController.animateToPage(
        _currentStep,
        duration: const Duration(milliseconds: 225),
        curve: Curves.easeOut,
      );
    } else {
      context.pop();
    }
  }

  bool _isStepValid(BookingWizardState s, int stepIndex) {
    switch (stepIndex) {
      case 0:
        return s.selectedService != null;
      case 1:
        return true; // add-ons optional
      case 2:
        return s.selectedDate != null && s.selectedSlot != null;
      case 3:
        return true; // therapist optional
      case 4:
        return true; // room optional
      case 5:
        return s.customerName.trim().length >= 2 &&
            s.customerPhone.trim().length >= 10 &&
            s.customerEmail.trim().contains('@');
      case 6:
        return true;
      default:
        return false;
    }
  }

  @override
  Widget build(BuildContext context) {
    final branchAsync = ref.watch(branchDetailProvider(widget.branchId));
    final wizardState = ref.watch(bookingWizardProvider(widget.branchId));

    return branchAsync.when(
      loading: () => Scaffold(
        appBar: AppBar(title: const Text('Booking')),
        body: const Center(child: CircularProgressIndicator.adaptive()),
      ),
      error: (e, _) => Scaffold(
        appBar: AppBar(),
        body: ErrorView(
          message: 'Gagal memuat data cabang. Coba lagi.',
          onRetry: () => ref.invalidate(branchDetailProvider(widget.branchId)),
        ),
      ),
      data: (branch) {
        // Compute visible steps: skip step 1 if branch has no addons at all
        // (ADR-0010 revised — addons are tenant-wide, top-level on branch).
        final hasAddons = branch.addons.isNotEmpty;

        _visibleSteps = [0, if (hasAddons) 1, 2, 3, 4, 5, 6];
        final effectiveTotal = _visibleSteps.length;
        if (_currentStep >= effectiveTotal) {
          _currentStep = effectiveTotal - 1;
        }

        final stepIndex = _visibleSteps[_currentStep];
        final isLastStep = _currentStep == effectiveTotal - 1;
        final isValid = _isStepValid(wizardState, stepIndex);

        return Scaffold(
          appBar: AppBar(
            leading: BackButton(onPressed: _prevStep),
            title: Text(
              'Booking — Langkah ${_currentStep + 1} dari $effectiveTotal',
            ),
            bottom: PreferredSize(
              preferredSize: const Size.fromHeight(3),
              child: LinearProgressIndicator(
                value: (_currentStep + 1) / effectiveTotal,
                minHeight: 3,
              ),
            ),
          ),
          bottomNavigationBar: _BottomNavBar(
            showBack: _currentStep > 0,
            isLastStep: isLastStep,
            isValid: isValid,
            onBack: _prevStep,
            onNext: () {
              if (isLastStep) {
                // Navigate to payment
                context.push('/branches/${widget.branchId}/book/payment');
              } else {
                _nextStep();
              }
            },
          ),
          body: PopScope(
            canPop: _currentStep == 0,
            onPopInvokedWithResult: (didPop, _) {
              if (!didPop) _prevStep();
            },
            child: PageView(
              controller: _pageController,
              physics: const NeverScrollableScrollPhysics(),
              children: _visibleSteps
                  .map((si) => _buildStep(si, branch, wizardState))
                  .toList(),
            ),
          ),
        );
      },
    );
  }

  Widget _buildStep(
    int stepIndex,
    BranchDetail branch,
    BookingWizardState state,
  ) {
    switch (stepIndex) {
      case 0:
        return _ServicePickerStep(branch: branch, state: state);
      case 1:
        return _AddonPickerStep(branch: branch, state: state);
      case 2:
        return _SlotPickerStep(branch: branch, state: state);
      case 3:
        return _TherapistPickerStep(
          therapists: branch.therapists,
          state: state,
        );
      case 4:
        return _RoomPickerStep(rooms: branch.rooms, state: state);
      case 5:
        return _CustomerInfoStep(state: state, branchId: branch.id);
      case 6:
        return _SummaryStep(branch: branch, state: state);
      default:
        return const SizedBox.shrink();
    }
  }
}

// ---------------------------------------------------------------------------
// Step 1 — Pilih Layanan
// ---------------------------------------------------------------------------
class _ServicePickerStep extends ConsumerWidget {
  const _ServicePickerStep({required this.branch, required this.state});
  final BranchDetail branch;
  final BookingWizardState state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Pilih Layanan', style: theme.textTheme.titleLarge),
          const SizedBox(height: 16),
          ...branch.services.map((service) {
            final selected = state.selectedService == service.id;
            return Card(
              margin: const EdgeInsets.only(bottom: 8),
              color: selected ? cs.primaryContainer : null,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12),
                side: selected
                    ? BorderSide(color: cs.primary, width: 2)
                    : BorderSide(color: cs.outlineVariant),
              ),
              child: ListTile(
                title: Text(
                  service.name,
                  style: theme.textTheme.bodyLarge?.copyWith(
                    fontWeight: FontWeight.w500,
                  ),
                ),
                subtitle: Text(
                  '${service.durationMinutes} menit • ${CurrencyFormatter.formatRupiah(service.priceIdr)}',
                  style: theme.textTheme.bodySmall,
                ),
                trailing: selected
                    ? Icon(Icons.check_circle, color: cs.primary)
                    : null,
                onTap: () => ref
                    .read(bookingWizardProvider(state.branchId).notifier)
                    .selectService(service.id),
              ),
            );
          }),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Step 2 — Pilih Add-on
// ---------------------------------------------------------------------------
class _AddonPickerStep extends ConsumerWidget {
  const _AddonPickerStep({required this.branch, required this.state});
  final BranchDetail branch;
  final BookingWizardState state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final selectedSvc = branch.services
        .where((s) => s.id == state.selectedService)
        .firstOrNull;
    // ADR-0010 revised: use top-level branch.addons (tenant-wide).
    final addons = branch.addons;

    var runningTotal = selectedSvc?.priceIdr ?? 0;
    for (final a in addons) {
      if (state.selectedAddonIds.contains(a.id)) runningTotal += a.priceIdr;
    }

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Pilih Tambahan', style: theme.textTheme.titleLarge),
          const SizedBox(height: 8),
          Text(
            'Opsional — tambahkan layanan pelengkap.',
            style: theme.textTheme.bodyMedium?.copyWith(
              color: cs.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 16),
          ...addons.map((addon) {
            final checked = state.selectedAddonIds.contains(addon.id);
            return CheckboxListTile(
              title: Text(addon.name),
              subtitle: Text(CurrencyFormatter.formatRupiah(addon.priceIdr)),
              value: checked,
              activeColor: cs.primary,
              onChanged: (_) => ref
                  .read(bookingWizardProvider(state.branchId).notifier)
                  .toggleAddon(addon.id),
            );
          }),
          const Divider(height: 24),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text('Subtotal:', style: theme.textTheme.titleMedium),
              Text(
                CurrencyFormatter.formatRupiah(runningTotal),
                style: theme.textTheme.titleMedium?.copyWith(
                  color: cs.primary,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Step 3 — Pilih Tanggal & Slot (BK-A6)
// ---------------------------------------------------------------------------
class _SlotPickerStep extends ConsumerStatefulWidget {
  const _SlotPickerStep({required this.branch, required this.state});
  final BranchDetail branch;
  final BookingWizardState state;

  @override
  ConsumerState<_SlotPickerStep> createState() => _SlotPickerStepState();
}

class _SlotPickerStepState extends ConsumerState<_SlotPickerStep> {
  static String _fmt(DateTime d) =>
      '${d.year.toString().padLeft(4, '0')}-'
      '${d.month.toString().padLeft(2, '0')}-'
      '${d.day.toString().padLeft(2, '0')}';

  @override
  void initState() {
    super.initState();
    // Auto-select today when the user first lands on this step. Without this
    // the date column shows today highlighted but state.selectedDate stays
    // null, which keeps the "Lanjut" button disabled even though the visual
    // suggests today is already chosen.
    if (widget.state.selectedDate == null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        ref
            .read(bookingWizardProvider(widget.state.branchId).notifier)
            .selectDate(DateTime.now());
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final today = DateTime.now();
    final dates = List.generate(14, (i) => today.add(Duration(days: i)));
    final selectedDate = widget.state.selectedDate ?? today;
    final dateStr = _fmt(selectedDate);
    final state = widget.state;
    final branch = widget.branch;
    final serviceId = state.selectedService ?? '';

    final slotsAsync = serviceId.isNotEmpty
        ? ref.watch(
            availabilityProvider(
              branchId: branch.id,
              serviceId: serviceId,
              date: dateStr,
            ),
          )
        : null;

    const dayNames = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min'];
    const monthNames = [
      '',
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'Mei',
      'Jun',
      'Jul',
      'Agu',
      'Sep',
      'Okt',
      'Nov',
      'Des',
    ];

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Pilih Tanggal', style: theme.textTheme.titleLarge),
          const SizedBox(height: 12),
          SizedBox(
            height: 80,
            child: ListView.builder(
              scrollDirection: Axis.horizontal,
              itemCount: dates.length,
              itemBuilder: (_, i) {
                final date = dates[i];
                final isSelected = _fmt(date) == _fmt(selectedDate);
                return GestureDetector(
                  onTap: () => ref
                      .read(bookingWizardProvider(state.branchId).notifier)
                      .selectDate(date),
                  child: Padding(
                    padding: const EdgeInsets.only(right: 8),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          dayNames[date.weekday - 1],
                          style: theme.textTheme.bodySmall,
                        ),
                        const SizedBox(height: 4),
                        Container(
                          width: 48,
                          height: 56,
                          decoration: BoxDecoration(
                            color: isSelected
                                ? cs.primary
                                : cs.surfaceContainerHighest,
                            borderRadius: BorderRadius.circular(12),
                          ),
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Text(
                                date.day.toString(),
                                style: theme.textTheme.titleMedium?.copyWith(
                                  fontWeight: FontWeight.w600,
                                  color: isSelected
                                      ? cs.onPrimary
                                      : cs.onSurface,
                                ),
                              ),
                              Text(
                                monthNames[date.month],
                                style: theme.textTheme.bodySmall?.copyWith(
                                  color: isSelected
                                      ? cs.onPrimary.withAlpha(204)
                                      : cs.onSurfaceVariant,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
          ),
          const SizedBox(height: 24),
          Text('Pilih Slot', style: theme.textTheme.titleLarge),
          const SizedBox(height: 12),
          if (slotsAsync == null)
            Text(
              'Pilih layanan terlebih dahulu.',
              style: theme.textTheme.bodyMedium?.copyWith(
                color: cs.onSurfaceVariant,
              ),
            )
          else
            slotsAsync.when(
              loading: () => GridView.count(
                crossAxisCount: 3,
                childAspectRatio: 2.5,
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
              ),
              error: (_, __) => Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Gagal memuat slot waktu.',
                    style: theme.textTheme.bodyMedium,
                  ),
                  const SizedBox(height: 8),
                  OutlinedButton(
                    onPressed: () => ref.invalidate(
                      availabilityProvider(
                        branchId: branch.id,
                        serviceId: serviceId,
                        date: dateStr,
                      ),
                    ),
                    child: const Text('Coba Lagi'),
                  ),
                ],
              ),
              data: (slots) {
                if (slots.isEmpty) {
                  return Padding(
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    child: Text(
                      'Tidak ada slot tersedia pada hari ini. Pilih tanggal lain.',
                      style: theme.textTheme.bodyMedium,
                      textAlign: TextAlign.center,
                    ),
                  );
                }
                return GridView.count(
                  crossAxisCount: 3,
                  childAspectRatio: 2.5,
                  shrinkWrap: true,
                  physics: const NeverScrollableScrollPhysics(),
                  mainAxisSpacing: 8,
                  crossAxisSpacing: 8,
                  children: slots.map((slot) {
                    final isSelected = state.selectedSlot?.start == slot.start;
                    final timeLabel = DateFormatter.formatTime(slot.start);
                    return Semantics(
                      label: slot.isAvailable
                          ? timeLabel
                          : '$timeLabel — tidak tersedia',
                      excludeSemantics: true,
                      child: FilterChip(
                        label: Text(
                          timeLabel,
                          style: theme.textTheme.labelLarge,
                        ),
                        selected: isSelected,
                        tooltip: !slot.isAvailable ? 'Tidak tersedia' : null,
                        onSelected: slot.isAvailable
                            ? (_) => ref
                                  .read(
                                    bookingWizardProvider(
                                      state.branchId,
                                    ).notifier,
                                  )
                                  .selectSlot(slot)
                            : null,
                      ),
                    );
                  }).toList(),
                );
              },
            ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Step 4 — Pilih Terapis (BK-A7) — swipeable card carousel
// ---------------------------------------------------------------------------
class _TherapistPickerStep extends ConsumerStatefulWidget {
  const _TherapistPickerStep({required this.therapists, required this.state});
  final List<TherapistItem> therapists;
  final BookingWizardState state;

  @override
  ConsumerState<_TherapistPickerStep> createState() =>
      _TherapistPickerStepState();
}

class _TherapistPickerStepState extends ConsumerState<_TherapistPickerStep> {
  late final PageController _carouselController;
  int _currentPage = 0;

  // Index 0 = "auto" card; indices 1..n = therapist cards.
  int get _totalCards => widget.therapists.length + 1;

  @override
  void initState() {
    super.initState();
    // If a therapist is already selected, land on their card; otherwise auto (0).
    final initial = widget.state.selectedTherapistId != null
        ? widget.therapists.indexWhere(
              (t) => t.id == widget.state.selectedTherapistId,
            ) +
              1
        : 0;
    _carouselController = PageController(
      viewportFraction: 0.85,
      initialPage: initial.clamp(0, _totalCards - 1),
    );
    _currentPage = initial.clamp(0, _totalCards - 1);
  }

  @override
  void dispose() {
    _carouselController.dispose();
    super.dispose();
  }

  void _onPageChanged(int page) {
    setState(() => _currentPage = page);
  }

  void _selectCurrentCard() {
    final notifier =
        ref.read(bookingWizardProvider(widget.state.branchId).notifier);
    if (_currentPage == 0) {
      notifier.selectTherapist(null);
    } else {
      final therapist = widget.therapists[_currentPage - 1];
      notifier.selectTherapist(therapist.id);
    }
    // Auto-advance to next step via the parent wizard's _nextStep callback.
    // We bubble via a post-frame to allow Riverpod state to settle first.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      // Walk up the widget tree to find the wizard state and trigger next step.
      final wizardState = context.findAncestorStateOfType<_BookingWizardScreenState>();
      wizardState?._nextStep();
    });
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    if (widget.therapists.isEmpty) {
      return _EmptyTherapistState(
        state: widget.state,
        onAutoSelect: () => ref
            .read(bookingWizardProvider(widget.state.branchId).notifier)
            .selectTherapist(null),
      );
    }

    return Column(
      children: [
        const SizedBox(height: 12),
        // Counter + hint label row
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Geser untuk pilih terapis',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: cs.onSurfaceVariant,
                ),
              ),
              Text(
                '${_currentPage + 1} / $_totalCards',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: cs.onSurfaceVariant,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 8),
        // Carousel
        Expanded(
          child: PageView.builder(
            controller: _carouselController,
            itemCount: _totalCards,
            onPageChanged: _onPageChanged,
            itemBuilder: (context, index) {
              if (index == 0) {
                return _AutoTherapistCard(
                  isSelected: widget.state.selectedTherapistId == null,
                  onSelect: _selectCurrentCard,
                  isCurrentPage: _currentPage == 0,
                );
              }
              final therapist = widget.therapists[index - 1];
              return _TherapistCard(
                therapist: therapist,
                isSelected:
                    widget.state.selectedTherapistId == therapist.id,
                isCurrentPage: _currentPage == index,
                onSelect: _selectCurrentCard,
              );
            },
          ),
        ),
        const SizedBox(height: 12),
        // Page indicator dots
        SmoothPageIndicator(
          controller: _carouselController,
          count: _totalCards,
          effect: WormEffect(
            dotHeight: 8,
            dotWidth: 8,
            activeDotColor: cs.primary,
            dotColor: cs.outlineVariant,
            spacing: 6,
          ),
        ),
        const SizedBox(height: 8),
        // "Lewati & auto-pilih" text button — visible only when not on auto card
        AnimatedOpacity(
          opacity: _currentPage != 0 ? 1.0 : 0.0,
          duration: const Duration(milliseconds: 200),
          child: TextButton(
            onPressed: _currentPage != 0
                ? () {
                    _carouselController.animateToPage(
                      0,
                      duration: const Duration(milliseconds: 300),
                      curve: Curves.easeInOut,
                    );
                    ref
                        .read(
                          bookingWizardProvider(widget.state.branchId).notifier,
                        )
                        .selectTherapist(null);
                  }
                : null,
            child: Text(
              'Lewati & auto-pilih',
              style: theme.textTheme.bodySmall?.copyWith(
                color: cs.onSurfaceVariant,
              ),
            ),
          ),
        ),
        const SizedBox(height: 8),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Auto-pick card (first card in carousel)
// ---------------------------------------------------------------------------
class _AutoTherapistCard extends StatelessWidget {
  const _AutoTherapistCard({
    required this.isSelected,
    required this.onSelect,
    required this.isCurrentPage,
  });

  final bool isSelected;
  final VoidCallback onSelect;
  final bool isCurrentPage;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    return AnimatedScale(
      scale: isCurrentPage ? 1.0 : 0.94,
      duration: const Duration(milliseconds: 200),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 400),
          child: Card(
            elevation: isCurrentPage ? 4 : 1,
            clipBehavior: Clip.antiAlias,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(20),
              side: isSelected
                  ? BorderSide(color: cs.primary, width: 2)
                  : BorderSide(color: cs.outlineVariant),
            ),
            child: Column(
              children: [
                // Photo area
                Container(
                  height: 280,
                  color: cs.primaryContainer,
                  child: Center(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.auto_awesome,
                          size: 72,
                          color: cs.primary,
                        ),
                        const SizedBox(height: 12),
                        Text(
                          'Sistem Pilihkan',
                          style: theme.textTheme.titleLarge?.copyWith(
                            color: cs.onPrimaryContainer,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
                // Content
                Expanded(
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Sistem Pilihkan',
                          style: theme.textTheme.titleLarge?.copyWith(
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        const SizedBox(height: 6),
                        Text(
                          'Kami akan otomatis pilih terapis yang tersedia.',
                          style: theme.textTheme.bodyMedium?.copyWith(
                            color: cs.onSurfaceVariant,
                          ),
                          maxLines: 3,
                          overflow: TextOverflow.ellipsis,
                        ),
                        const Spacer(),
                        SizedBox(
                          width: double.infinity,
                          child: FilledButton(
                            onPressed: onSelect,
                            child: Row(
                              mainAxisAlignment: MainAxisAlignment.center,
                              children: [
                                if (isSelected) ...[
                                  const Icon(Icons.check_circle, size: 18),
                                  const SizedBox(width: 6),
                                ],
                                const Text('Pilih Terapis Ini'),
                              ],
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
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Individual therapist card
// ---------------------------------------------------------------------------
class _TherapistCard extends StatelessWidget {
  const _TherapistCard({
    required this.therapist,
    required this.isSelected,
    required this.isCurrentPage,
    required this.onSelect,
  });

  final TherapistItem therapist;
  final bool isSelected;
  final bool isCurrentPage;
  final VoidCallback onSelect;

  String _capitalizeFirst(String s) =>
      s.isEmpty ? s : s[0].toUpperCase() + s.substring(1).toLowerCase();

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    return AnimatedScale(
      scale: isCurrentPage ? 1.0 : 0.94,
      duration: const Duration(milliseconds: 200),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 400),
          child: Semantics(
            label: 'Terapis ${therapist.fullName}',
            child: Card(
              elevation: isCurrentPage ? 4 : 1,
              clipBehavior: Clip.antiAlias,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(20),
                side: isSelected
                    ? BorderSide(color: cs.primary, width: 2)
                    : BorderSide(color: cs.outlineVariant),
              ),
              child: Column(
                children: [
                  // Photo — 280px tall, full-width within card
                  SizedBox(
                    height: 280,
                    width: double.infinity,
                    child: therapist.photoUrl != null
                        ? CachedNetworkImage(
                            imageUrl: therapist.photoUrl!,
                            fit: BoxFit.cover,
                            placeholder: (_, __) => Container(
                              color: cs.surfaceContainerHighest,
                              child: const Center(
                                child: CircularProgressIndicator.adaptive(
                                  strokeWidth: 2,
                                ),
                              ),
                            ),
                            errorWidget: (_, __, ___) =>
                                _PhotoPlaceholder(cs: cs),
                          )
                        : _PhotoPlaceholder(cs: cs),
                  ),
                  // Card body
                  Expanded(
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          // Name
                          Text(
                            therapist.fullName,
                            style: theme.textTheme.titleLarge?.copyWith(
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                          const SizedBox(height: 8),
                          // Body badges row
                          if (therapist.heightCm != null ||
                              therapist.weightKg != null ||
                              (therapist.build != null &&
                                  therapist.build!.isNotEmpty)) ...[
                            Wrap(
                              spacing: 6,
                              runSpacing: 4,
                              children: [
                                if (therapist.heightCm != null)
                                  _BodyBadge(
                                    '${therapist.heightCm} cm',
                                    cs: cs,
                                    theme: theme,
                                  ),
                                if (therapist.weightKg != null)
                                  _BodyBadge(
                                    '${therapist.weightKg} kg',
                                    cs: cs,
                                    theme: theme,
                                  ),
                                if (therapist.build != null &&
                                    therapist.build!.isNotEmpty)
                                  _BodyBadge(
                                    _capitalizeFirst(therapist.build!),
                                    cs: cs,
                                    theme: theme,
                                  ),
                              ],
                            ),
                            const SizedBox(height: 12),
                          ],
                          // CTA button
                          SizedBox(
                            width: double.infinity,
                            child: FilledButton(
                              onPressed: onSelect,
                              child: Row(
                                mainAxisAlignment: MainAxisAlignment.center,
                                children: [
                                  if (isSelected) ...[
                                    const Icon(Icons.check_circle, size: 18),
                                    const SizedBox(width: 6),
                                  ],
                                  const Text('Pilih Terapis Ini'),
                                ],
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
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Photo placeholder (no photo or load error)
// ---------------------------------------------------------------------------
class _PhotoPlaceholder extends StatelessWidget {
  const _PhotoPlaceholder({required this.cs});
  final ColorScheme cs;

  @override
  Widget build(BuildContext context) {
    return Container(
      color: cs.surfaceContainerHighest,
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.person, size: 72, color: cs.onSurfaceVariant),
          const SizedBox(height: 8),
          Text(
            'Belum ada foto',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
              color: cs.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Empty state — no therapists registered
// ---------------------------------------------------------------------------
class _EmptyTherapistState extends ConsumerWidget {
  const _EmptyTherapistState({
    required this.state,
    required this.onAutoSelect,
  });

  final BookingWizardState state;
  final VoidCallback onAutoSelect;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    return Column(
      children: [
        const SizedBox(height: 12),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Text(
            'Geser untuk pilih terapis',
            style: theme.textTheme.bodySmall?.copyWith(
              color: cs.onSurfaceVariant,
            ),
          ),
        ),
        const SizedBox(height: 8),
        Expanded(
          child: Center(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 24),
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 400),
                child: Card(
                  elevation: 4,
                  clipBehavior: Clip.antiAlias,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(20),
                    side: BorderSide(color: cs.primary, width: 2),
                  ),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Container(
                        height: 280,
                        color: cs.primaryContainer,
                        child: Center(
                          child: Column(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Icon(
                                Icons.auto_awesome,
                                size: 72,
                                color: cs.primary,
                              ),
                              const SizedBox(height: 12),
                              Text(
                                'Sistem Pilihkan',
                                style: theme.textTheme.titleLarge?.copyWith(
                                  color: cs.onPrimaryContainer,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                      Padding(
                        padding: const EdgeInsets.fromLTRB(16, 16, 16, 16),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              'Belum ada terapis terdaftar di cabang ini.',
                              style: theme.textTheme.titleMedium?.copyWith(
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                            const SizedBox(height: 6),
                            Text(
                              'Kami akan otomatis pilih saat hari pelayanan.',
                              style: theme.textTheme.bodyMedium?.copyWith(
                                color: cs.onSurfaceVariant,
                              ),
                            ),
                            const SizedBox(height: 16),
                            SizedBox(
                              width: double.infinity,
                              child: FilledButton(
                                onPressed: onAutoSelect,
                                child: const Text('Lanjut'),
                              ),
                            ),
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
        // Static single dot indicator
        Container(
          width: 8,
          height: 8,
          margin: const EdgeInsets.symmetric(vertical: 12),
          decoration: BoxDecoration(
            color: cs.primary,
            shape: BoxShape.circle,
          ),
        ),
        const SizedBox(height: 16),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Body badge pill chip
// ---------------------------------------------------------------------------
class _BodyBadge extends StatelessWidget {
  const _BodyBadge(this.label, {required this.cs, required this.theme});
  final String label;
  final ColorScheme cs;
  final ThemeData theme;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: cs.primaryContainer,
        borderRadius: BorderRadius.circular(20),
      ),
      child: Text(
        label,
        style: theme.textTheme.bodySmall?.copyWith(
          color: cs.onPrimaryContainer,
          fontWeight: FontWeight.w500,
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Step 5 — Pilih Ruangan (BK-A8)
// ---------------------------------------------------------------------------
class _RoomPickerStep extends ConsumerWidget {
  const _RoomPickerStep({required this.rooms, required this.state});
  final List<RoomItem> rooms;
  final BookingWizardState state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final isAuto = state.selectedRoomId == null;

    if (rooms.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.meeting_room_outlined, size: 48, color: cs.error),
              const SizedBox(height: 16),
              Text(
                'Tidak ada ruangan tersedia di slot ini.',
                textAlign: TextAlign.center,
                style: theme.textTheme.bodyMedium,
              ),
            ],
          ),
        ),
      );
    }

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Text('Pilih Ruangan', style: theme.textTheme.titleLarge),
        const SizedBox(height: 8),
        // Auto
        Card(
          color: isAuto ? cs.primaryContainer : null,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
            side: isAuto
                ? BorderSide(color: cs.primary, width: 2)
                : BorderSide(color: cs.outlineVariant),
          ),
          child: Semantics(
            label: 'Pilih saja — ruangan dipilihkan otomatis',
            child: ListTile(
              leading: CircleAvatar(
                radius: 24,
                backgroundColor: cs.primaryContainer,
                child: Icon(Icons.meeting_room_outlined, color: cs.primary),
              ),
              title: const Text('Pilih Saja'),
              subtitle: const Text(
                'Kami pilihkan ruangan yang tersedia untukmu',
              ),
              trailing: isAuto
                  ? Icon(Icons.check_circle, color: cs.primary)
                  : null,
              onTap: () => ref
                  .read(bookingWizardProvider(state.branchId).notifier)
                  .selectRoom(null),
            ),
          ),
        ),
        const SizedBox(height: 8),
        ...rooms.map((room) {
          final isSelected = state.selectedRoomId == room.id;
          final roomTypeLabel = room.roomType ?? 'Reguler';
          return Semantics(
            label: '${room.name}, $roomTypeLabel, kapasitas ${room.capacity}',
            child: Card(
              margin: const EdgeInsets.only(bottom: 8),
              color: isSelected ? cs.primaryContainer : null,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12),
                side: isSelected
                    ? BorderSide(color: cs.primary, width: 2)
                    : BorderSide(color: cs.outlineVariant),
              ),
              child: InkWell(
                borderRadius: BorderRadius.circular(12),
                onTap: () => ref
                    .read(bookingWizardProvider(state.branchId).notifier)
                    .selectRoom(room.id),
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Row(
                    children: [
                      ClipRRect(
                        borderRadius: BorderRadius.circular(8),
                        child: room.photoUrl != null
                            ? Image.network(
                                room.photoUrl!,
                                width: 72,
                                height: 72,
                                fit: BoxFit.cover,
                                errorBuilder: (_, __, ___) =>
                                    _roomPlaceholder(cs),
                              )
                            : _roomPlaceholder(cs),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(room.name, style: theme.textTheme.titleMedium),
                            const SizedBox(height: 4),
                            Row(
                              children: [
                                Chip(
                                  label: Text(
                                    roomTypeLabel,
                                    style: theme.textTheme.bodySmall,
                                  ),
                                  visualDensity: VisualDensity.compact,
                                  materialTapTargetSize:
                                      MaterialTapTargetSize.shrinkWrap,
                                ),
                                const SizedBox(width: 8),
                                Text(
                                  'Kapasitas: ${room.capacity}',
                                  style: theme.textTheme.bodySmall?.copyWith(
                                    color: cs.onSurfaceVariant,
                                  ),
                                ),
                              ],
                            ),
                          ],
                        ),
                      ),
                      if (isSelected)
                        Padding(
                          padding: const EdgeInsets.only(right: 8),
                          child: Icon(Icons.check_circle, color: cs.primary),
                        ),
                    ],
                  ),
                ),
              ),
            ),
          );
        }),
      ],
    );
  }

  Widget _roomPlaceholder(ColorScheme cs) => Container(
    width: 72,
    height: 72,
    color: cs.surfaceContainerHighest,
    child: Icon(Icons.meeting_room_outlined, color: cs.onSurfaceVariant),
  );
}

// ---------------------------------------------------------------------------
// Step 6 — Info Customer (BK-A9)
// ---------------------------------------------------------------------------
class _CustomerInfoStep extends ConsumerStatefulWidget {
  const _CustomerInfoStep({required this.state, required this.branchId});
  final BookingWizardState state;
  final String branchId;

  @override
  ConsumerState<_CustomerInfoStep> createState() => _CustomerInfoStepState();
}

class _CustomerInfoStepState extends ConsumerState<_CustomerInfoStep> {
  late final TextEditingController _nameCtrl;
  late final TextEditingController _phoneCtrl;
  late final TextEditingController _emailCtrl;
  final _formKey = GlobalKey<FormState>();

  @override
  void initState() {
    super.initState();
    _nameCtrl = TextEditingController(text: widget.state.customerName);
    _phoneCtrl = TextEditingController(text: widget.state.customerPhone);
    _emailCtrl = TextEditingController(text: widget.state.customerEmail);
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _phoneCtrl.dispose();
    _emailCtrl.dispose();
    super.dispose();
  }

  void _save() {
    ref
        .read(bookingWizardProvider(widget.branchId).notifier)
        .updateCustomerInfo(
          name: _nameCtrl.text.trim(),
          phone: _phoneCtrl.text.trim(),
          email: _emailCtrl.text.trim(),
        );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Form(
        key: _formKey,
        onChanged: _save,
        autovalidateMode: AutovalidateMode.onUserInteraction,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Informasi Pemesanan', style: theme.textTheme.titleLarge),
            const SizedBox(height: 16),
            TextFormField(
              controller: _nameCtrl,
              decoration: const InputDecoration(
                labelText: 'Nama Lengkap',
                hintText: 'Masukkan nama kamu',
              ),
              keyboardType: TextInputType.name,
              textCapitalization: TextCapitalization.words,
              validator: (v) {
                if (v == null || v.trim().isEmpty) return 'Nama wajib diisi.';
                if (v.trim().length < 2) return 'Nama terlalu pendek.';
                return null;
              },
            ),
            const SizedBox(height: 16),
            TextFormField(
              controller: _phoneCtrl,
              decoration: const InputDecoration(
                labelText: 'No. WhatsApp',
                hintText: '08xxxxxxxxxx',
              ),
              keyboardType: TextInputType.phone,
              validator: (v) {
                if (v == null || v.trim().isEmpty) {
                  return 'Nomor WhatsApp wajib diisi.';
                }
                if (v.trim().length < 10) return 'Format nomor tidak valid.';
                return null;
              },
            ),
            const SizedBox(height: 16),
            TextFormField(
              controller: _emailCtrl,
              decoration: const InputDecoration(
                labelText: 'Email',
                hintText: 'kamu@email.com',
              ),
              keyboardType: TextInputType.emailAddress,
              validator: (v) {
                if (v == null || v.trim().isEmpty) return 'Email wajib diisi.';
                if (!v.contains('@') || !v.contains('.')) {
                  return 'Format email tidak valid.';
                }
                return null;
              },
            ),
            const SizedBox(height: 16),
            const _TncNotice(),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Step 7 — Ringkasan
// ---------------------------------------------------------------------------
class _SummaryStep extends ConsumerWidget {
  const _SummaryStep({required this.branch, required this.state});
  final BranchDetail branch;
  final BookingWizardState state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    final service = branch.services
        .where((s) => s.id == state.selectedService)
        .firstOrNull;
    // ADR-0010 revised: addons are top-level on branch, not nested per service.
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

    var total = service?.priceIdr ?? 0;
    for (final a in selectedAddons) {
      total += a.priceIdr;
    }

    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Ringkasan Booking', style: theme.textTheme.titleLarge),
          const SizedBox(height: 4),
          Text(
            'Periksa kembali, lalu tap Bayar di bawah.',
            style: theme.textTheme.bodySmall?.copyWith(
              color: cs.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 12),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _Row('Cabang', branch.name),
                  _Row(
                    'Layanan',
                    '${service?.name ?? '-'} (${service?.durationMinutes ?? 0} menit)',
                  ),
                  if (selectedAddons.isNotEmpty)
                    _Row(
                      'Tambahan',
                      selectedAddons.map((a) => a.name).join(', '),
                    ),
                  if (state.selectedSlot != null)
                    _Row(
                      'Jadwal',
                      '${DateFormatter.formatDate(state.selectedSlot!.start)}\n'
                          '${DateFormatter.formatSlotRange(state.selectedSlot!.start, state.selectedSlot!.end)}',
                    ),
                  _Row('Terapis', therapist?.fullName ?? 'Pilih otomatis'),
                  _Row('Ruangan', room?.name ?? 'Pilih otomatis'),
                  const Divider(height: 20),
                  _Row('Nama', state.customerName),
                  _Row('WhatsApp', state.customerPhone),
                  _Row('Email', state.customerEmail),
                  const Divider(height: 20),
                  if (service != null)
                    _PriceRow(service.name, service.priceIdr),
                  ...selectedAddons.map((a) => _PriceRow(a.name, a.priceIdr)),
                  const SizedBox(height: 8),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        'Total',
                        style: theme.textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      Text(
                        CurrencyFormatter.formatRupiah(total),
                        style: theme.textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.w700,
                          color: cs.primary,
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _Row extends StatelessWidget {
  const _Row(this.label, this.value);
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 80,
            child: Text(
              label,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ),
          const SizedBox(width: 8),
          Expanded(child: Text(value, style: theme.textTheme.bodyMedium)),
        ],
      ),
    );
  }
}

class _PriceRow extends StatelessWidget {
  const _PriceRow(this.name, this.price);
  final String name;
  final int price;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Expanded(
            child: Text(name, style: Theme.of(context).textTheme.bodyMedium),
          ),
          Text(
            CurrencyFormatter.formatRupiah(price),
            style: Theme.of(context).textTheme.bodyMedium,
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// T&C notice with tappable link (BK-R17)
// ---------------------------------------------------------------------------
class _TncNotice extends StatefulWidget {
  const _TncNotice();

  @override
  State<_TncNotice> createState() => _TncNoticeState();
}

class _TncNoticeState extends State<_TncNotice> {
  late final TapGestureRecognizer _tapRecognizer;

  @override
  void initState() {
    super.initState();
    _tapRecognizer = TapGestureRecognizer()
      ..onTap = () => _showTncModal(context);
  }

  @override
  void dispose() {
    _tapRecognizer.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    return RichText(
      text: TextSpan(
        style: theme.textTheme.bodySmall?.copyWith(color: cs.onSurfaceVariant),
        children: [
          const TextSpan(text: 'Dengan melanjutkan, kamu menyetujui '),
          TextSpan(
            text: 'Syarat & Ketentuan',
            recognizer: _tapRecognizer,
            style: TextStyle(
              color: cs.primary,
              decoration: TextDecoration.underline,
            ),
          ),
          const TextSpan(
            text:
                ' dan memahami bahwa booking yang sudah dibayar tidak dapat dibatalkan.',
          ),
        ],
      ),
    );
  }

  void _showTncModal(BuildContext context) {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(24)),
      ),
      builder: (_) => DraggableScrollableSheet(
        initialChildSize: 0.6,
        expand: false,
        builder: (_, ctrl) => Padding(
          padding: const EdgeInsets.fromLTRB(24, 16, 24, 32),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(
                child: Container(
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: Theme.of(context).colorScheme.outlineVariant,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: 16),
              Text(
                'Syarat & Ketentuan',
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 16),
              Expanded(
                child: SingleChildScrollView(
                  controller: ctrl,
                  child: const Text(
                    '1. Booking yang sudah dibayar tidak dapat dibatalkan atau dikembalikan.\n\n'
                    '2. Kode booking hanya berlaku untuk tanggal dan waktu yang telah dipilih.\n\n'
                    '3. Lustia berhak membatalkan booking jika terdapat pelanggaran ketentuan, dengan pemberitahuan kepada pelanggan.\n\n'
                    '4. Pelanggan wajib hadir tepat waktu. Keterlambatan lebih dari 15 menit dapat mengakibatkan pembatalan sesi.\n\n'
                    '(Konten lengkap akan diperbarui sebelum peluncuran resmi.)',
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

// ---------------------------------------------------------------------------
// Bottom nav bar
// ---------------------------------------------------------------------------
class _BottomNavBar extends StatelessWidget {
  const _BottomNavBar({
    required this.showBack,
    required this.isLastStep,
    required this.isValid,
    required this.onBack,
    required this.onNext,
  });

  final bool showBack;
  final bool isLastStep;
  final bool isValid;
  final VoidCallback onBack;
  final VoidCallback onNext;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    // Explicit colors so the button is visible regardless of theme override
    // anomalies on Flutter web (FilledButton sometimes renders transparent
    // when running in `flutter run -d chrome` debug build).
    return Material(
      elevation: 8,
      color: cs.surface,
      child: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
          child: Row(
            children: [
              if (showBack) ...[
                SizedBox(
                  height: 56,
                  child: OutlinedButton(
                    onPressed: onBack,
                    style: OutlinedButton.styleFrom(
                      foregroundColor: cs.primary,
                      side: BorderSide(color: cs.primary),
                      textStyle: const TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    child: const Text('Kembali'),
                  ),
                ),
                const SizedBox(width: 12),
              ],
              Expanded(
                child: SizedBox(
                  height: 56,
                  child: ElevatedButton(
                    onPressed: isValid ? onNext : null,
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
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    child: Text(isLastStep ? 'Bayar' : 'Lanjut'),
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
