// Widget tampilan loading — digunakan saat AsyncLoading.

import 'package:flutter/material.dart';

import '../../core/theme/lustia_colors.dart';

class LoadingView extends StatelessWidget {
  const LoadingView({super.key});

  @override
  Widget build(BuildContext context) {
    return const Center(
      child: CircularProgressIndicator(
        valueColor: AlwaysStoppedAnimation<Color>(LustiaColors.primary),
      ),
    );
  }
}
