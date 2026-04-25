// Widget QR code — membungkus qr_flutter.
// Digunakan di BookingConfirmationScreen dan MyBookingsScreen.

import 'package:flutter/material.dart';
import 'package:qr_flutter/qr_flutter.dart';

/// Menampilkan QR code dari [data] (biasanya kode booking, contoh: "B7K3-M2QF").
class QrDisplay extends StatelessWidget {
  const QrDisplay({super.key, required this.data, this.size = 220});

  final String data;
  final double size;

  @override
  Widget build(BuildContext context) {
    return RepaintBoundary(
      // RepaintBoundary: QR tidak perlu ikut repaint saat parent rebuild.
      // Juga berguna sebagai boundary untuk share-as-image (Phase 6).
      child: QrImageView(
        data: data,
        errorCorrectionLevel: QrErrorCorrectLevel.M,
        size: size,
        semanticsLabel: 'QR kode booking $data',
      ),
    );
  }
}
