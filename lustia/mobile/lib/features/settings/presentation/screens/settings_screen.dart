// Layar Pengaturan.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:geolocator/geolocator.dart';

import '../../../../features/favorites/data/favorites_notifier.dart';

const _appVersion = '1.0.0';

class SettingsScreen extends ConsumerWidget {
  const SettingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: Text('Pengaturan', style: theme.textTheme.headlineMedium),
      ),
      body: ListView(
        children: [
          // Location section
          const _SectionHeader('Lokasi'),
          ListTile(
            leading: const Icon(Icons.location_on_outlined),
            title: const Text('Izin Lokasi'),
            subtitle: const Text('Digunakan untuk menampilkan cabang terdekat'),
            trailing: const Icon(Icons.open_in_new, size: 18),
            onTap: () => Geolocator.openAppSettings(),
          ),
          const Divider(),

          // About section
          const _SectionHeader('Tentang'),
          ListTile(
            leading: const Icon(Icons.description_outlined),
            title: const Text('Syarat & Ketentuan'),
            onTap: () => _showModal(
              context,
              title: 'Syarat & Ketentuan',
              content:
                  'Dengan menggunakan aplikasi Lustia, Anda menyetujui syarat dan ketentuan berikut:\n\n'
                  '1. Booking yang sudah dibayar tidak dapat dibatalkan atau dikembalikan.\n'
                  '2. Kode booking hanya berlaku untuk tanggal dan waktu yang telah dipilih.\n'
                  '3. Lustia berhak membatalkan booking dengan pemberitahuan.\n\n'
                  '(Teks ini adalah placeholder — konten lengkap akan diperbarui.)',
            ),
          ),
          ListTile(
            leading: const Icon(Icons.privacy_tip_outlined),
            title: const Text('Kebijakan Privasi'),
            onTap: () => _showModal(
              context,
              title: 'Kebijakan Privasi',
              content:
                  'Lustia mengumpulkan data nama, nomor telepon, dan email untuk tujuan booking.\n\n'
                  'Data Anda tidak dijual kepada pihak ketiga.\n\n'
                  '(Teks ini adalah placeholder — konten lengkap akan diperbarui.)',
            ),
          ),
          ListTile(
            leading: const Icon(Icons.info_outlined),
            title: const Text('Versi Aplikasi'),
            trailing: Text(
              _appVersion,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ),
          const Divider(),

          // Data section
          const _SectionHeader('Data'),
          ListTile(
            leading: const Icon(Icons.favorite_outline),
            title: const Text('Reset Favorit'),
            subtitle: const Text('Hapus semua cabang favorit'),
            trailing: const Icon(Icons.delete_outline),
            onTap: () => _confirmResetFavorites(context, ref),
          ),
        ],
      ),
    );
  }

  void _showModal(
    BuildContext context, {
    required String title,
    required String content,
  }) {
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
              Text(title, style: Theme.of(context).textTheme.titleLarge),
              const SizedBox(height: 16),
              Expanded(
                child: SingleChildScrollView(
                  controller: ctrl,
                  child: Text(content),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _confirmResetFavorites(
    BuildContext context,
    WidgetRef ref,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (_) => AlertDialog(
        title: const Text('Reset Favorit'),
        content: const Text(
          'Semua cabang favorit akan dihapus dari perangkat ini. Lanjutkan?',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Reset'),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      await ref.read(favoritesProvider.notifier).clearAll();
      if (context.mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('Favorit telah direset.')));
      }
    }
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader(this.title);
  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 4),
      child: Text(
        title.toUpperCase(),
        style: Theme.of(context).textTheme.bodySmall?.copyWith(
          color: Theme.of(context).colorScheme.onSurfaceVariant,
          letterSpacing: 1.2,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}
