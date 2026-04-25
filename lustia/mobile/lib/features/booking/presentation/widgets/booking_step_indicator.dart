// Widget indikator langkah wizard booking (step 1/7, 2/7, dst).
// TODO Phase 5: implementasi setelah jumlah step dikonfirmasi dari design spec.

import 'package:flutter/material.dart';

class BookingStepIndicator extends StatelessWidget {
  const BookingStepIndicator({
    super.key,
    required this.currentStep,
    required this.totalSteps,
  });

  final int currentStep;
  final int totalSteps;

  @override
  Widget build(BuildContext context) {
    // TODO Phase 5: implementasi dot atau progress bar step indicator.
    return Text('Langkah $currentStep dari $totalSteps');
  }
}
