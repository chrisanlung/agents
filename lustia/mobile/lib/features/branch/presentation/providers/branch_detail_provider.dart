// Provider detail cabang — one-shot fetch berdasarkan branch ID.

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../data/branch_model.dart';
import '../../data/branch_repository.dart';

part 'branch_detail_provider.g.dart';

@riverpod
Future<BranchDetail> branchDetail(BranchDetailRef ref, String id) async {
  final repo = ref.watch(branchRepositoryProvider);
  return repo.getBranchDetail(id);
}
