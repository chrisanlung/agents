// Layar detail cabang (BK-A4) — hero foto, info, layanan, CTA booking.

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../shared/utils/currency_formatter.dart';
import '../../../../shared/widgets/error_view.dart';
import '../../../favorites/data/favorites_notifier.dart';
import '../../data/branch_model.dart';
import '../providers/branch_detail_provider.dart';

class BranchDetailScreen extends ConsumerWidget {
  const BranchDetailScreen({super.key, required this.branchId});

  final String branchId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(branchDetailProvider(branchId));

    return async.when(
      loading: () => const _BranchDetailSkeleton(),
      error: (err, _) => Scaffold(
        appBar: AppBar(),
        body: ErrorView(
          message: 'Gagal memuat detail cabang. Coba lagi.',
          onRetry: () => ref.invalidate(branchDetailProvider(branchId)),
        ),
      ),
      data: (branch) => _BranchDetailContent(branch: branch),
    );
  }
}

// ---------------------------------------------------------------------------
// BK-R4: skeleton loading state for branch detail
// ---------------------------------------------------------------------------
class _BranchDetailSkeleton extends StatelessWidget {
  const _BranchDetailSkeleton();

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Scaffold(
      body: CustomScrollView(
        slivers: [
          SliverAppBar(
            expandedHeight: 240,
            pinned: true,
            backgroundColor: cs.surface,
            flexibleSpace: FlexibleSpaceBar(
              background: Container(
                color: cs.primaryContainer,
                child: Center(
                  child: Icon(Icons.spa, size: 48, color: cs.primary),
                ),
              ),
            ),
          ),
          SliverPadding(
            padding: const EdgeInsets.fromLTRB(16, 16, 16, 100),
            sliver: SliverList(
              delegate: SliverChildListDelegate([
                _ShimmerBox(
                  width: MediaQuery.of(context).size.width * 0.6,
                  height: 28,
                ),
                const SizedBox(height: 8),
                const _ShimmerBox(width: 160, height: 16),
                const SizedBox(height: 16),
                const _ShimmerBox(width: double.infinity, height: 14),
                const SizedBox(height: 6),
                const _ShimmerBox(width: 200, height: 14),
              ]),
            ),
          ),
        ],
      ),
    );
  }
}

class _ShimmerBox extends StatefulWidget {
  const _ShimmerBox({required this.width, required this.height});
  final double width;
  final double height;

  @override
  State<_ShimmerBox> createState() => _ShimmerBoxState();
}

class _ShimmerBoxState extends State<_ShimmerBox>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;
  late final Animation<double> _anim;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 900),
    )..repeat(reverse: true);
    _anim = Tween<double>(begin: 0.4, end: 1.0).animate(_ctrl);
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return FadeTransition(
      opacity: _anim,
      child: Container(
        width: widget.width,
        height: widget.height,
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(4),
        ),
      ),
    );
  }
}

class _BranchDetailContent extends ConsumerWidget {
  const _BranchDetailContent({required this.branch});
  final BranchDetail branch;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final favAsync = ref.watch(favoritesProvider);
    final isFav = favAsync.maybeWhen(
      data: (list) => list.contains(branch.id),
      orElse: () => false,
    );

    return Scaffold(
      body: Stack(
        children: [
          CustomScrollView(
            slivers: [
              // Hero AppBar
              SliverAppBar(
                expandedHeight: 240,
                pinned: true,
                backgroundColor: cs.surface,
                foregroundColor: Colors.white,
                flexibleSpace: FlexibleSpaceBar(
                  background: branch.photoUrl != null
                      ? CachedNetworkImage(
                          imageUrl: branch.photoUrl!,
                          fit: BoxFit.cover,
                          fadeInDuration: const Duration(milliseconds: 200),
                          placeholder: (_, __) => Container(
                            color: cs.primaryContainer,
                            child: Center(
                              child: Icon(
                                Icons.spa,
                                size: 48,
                                color: cs.primary,
                              ),
                            ),
                          ),
                          errorWidget: (_, __, ___) => Container(
                            color: cs.primaryContainer,
                            child: Center(
                              child: Icon(
                                Icons.spa,
                                size: 48,
                                color: cs.primary,
                              ),
                            ),
                          ),
                        )
                      : Container(
                          color: cs.primaryContainer,
                          child: Center(
                            child: Icon(Icons.spa, size: 48, color: cs.primary),
                          ),
                        ),
                ),
                actions: [
                  IconButton(
                    icon: Icon(
                      isFav ? Icons.favorite : Icons.favorite_outline,
                      color: Colors.white,
                    ),
                    tooltip: isFav ? 'Hapus dari favorit' : 'Simpan ke favorit',
                    onPressed: () =>
                        ref.read(favoritesProvider.notifier).toggle(branch.id),
                  ),
                ],
              ),

              // Body content
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(16, 16, 16, 100),
                sliver: SliverList(
                  delegate: SliverChildListDelegate([
                    // Name + tenant (contract: show "tenant_name — branch.name")
                    if (branch.tenantName.isNotEmpty)
                      Text(
                        branch.tenantName,
                        style: theme.textTheme.titleSmall?.copyWith(
                          color: cs.primary,
                          fontWeight: FontWeight.w600,
                          letterSpacing: 0.5,
                        ),
                      ),
                    if (branch.tenantName.isNotEmpty) const SizedBox(height: 2),
                    Text(
                      branch.name,
                      style: theme.textTheme.headlineMedium?.copyWith(
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(height: 12),

                    // Address
                    _InfoRow(
                      icon: Icons.location_on_outlined,
                      text: branch.fullAddress,
                    ),
                    const SizedBox(height: 8),

                    // Hours today
                    _InfoRow(
                      icon: Icons.schedule_outlined,
                      text: 'Buka hari ini: ${branch.todayHours}',
                      textColor: branch.isOpenToday ? null : cs.error,
                    ),
                    const SizedBox(height: 8),

                    // Phone
                    if (branch.contactPhone.isNotEmpty)
                      _InfoRow(
                        icon: Icons.phone_outlined,
                        text: branch.contactPhone,
                      ),

                    const Divider(height: 32),

                    // Services section
                    Text('Layanan', style: theme.textTheme.titleLarge),
                    const SizedBox(height: 12),

                    if (branch.services.isEmpty)
                      Text(
                        'Belum ada layanan tersedia.',
                        style: theme.textTheme.bodyMedium?.copyWith(
                          color: cs.onSurfaceVariant,
                        ),
                      )
                    else
                      ...branch.servicesByCategory.entries.map(
                        (entry) => _ServiceCategorySection(
                          category: entry.key,
                          services: entry.value,
                        ),
                      ),
                  ]),
                ),
              ),
            ],
          ),

          // Sticky "Booking" CTA
          Positioned(
            bottom: 0,
            left: 0,
            right: 0,
            child: Container(
              padding: EdgeInsets.fromLTRB(
                16,
                12,
                16,
                16 + MediaQuery.of(context).viewPadding.bottom,
              ),
              decoration: BoxDecoration(
                color: cs.surface,
                boxShadow: [
                  BoxShadow(
                    color: Colors.black.withAlpha(20),
                    blurRadius: 8,
                    offset: const Offset(0, -2),
                  ),
                ],
              ),
              child: FilledButton.icon(
                icon: const Icon(Icons.calendar_today_outlined),
                label: const Text('Booking'),
                style: FilledButton.styleFrom(
                  minimumSize: const Size(double.infinity, 52),
                ),
                onPressed: () => context.push('/branches/${branch.id}/book'),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  const _InfoRow({required this.icon, required this.text, this.textColor});

  final IconData icon;
  final String text;
  final Color? textColor;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 16, color: cs.primary),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            text,
            style: Theme.of(
              context,
            ).textTheme.bodyMedium?.copyWith(color: textColor),
          ),
        ),
      ],
    );
  }
}

class _ServiceCategorySection extends StatelessWidget {
  const _ServiceCategorySection({
    required this.category,
    required this.services,
  });

  final String category;
  final List<ServiceItem> services;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Text(
            category.toUpperCase(),
            style: theme.textTheme.bodySmall?.copyWith(
              color: cs.onSurfaceVariant,
              letterSpacing: 1.2,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
        ...services.map(
          (service) => ListTile(
            contentPadding: EdgeInsets.zero,
            title: Text(
              service.name,
              style: theme.textTheme.bodyLarge?.copyWith(
                fontWeight: FontWeight.w500,
              ),
            ),
            subtitle: Text(
              '${service.durationMinutes} menit',
              style: theme.textTheme.bodySmall,
            ),
            trailing: Text(
              CurrencyFormatter.formatRupiah(service.priceIdr),
              style: theme.textTheme.bodyMedium?.copyWith(
                fontWeight: FontWeight.w600,
                color: cs.primary,
              ),
            ),
          ),
        ),
      ],
    );
  }
}
