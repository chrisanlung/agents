// Repository cabang — interface + implementasi Dio.

import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/api/dio_client.dart';
import 'branch_model.dart';

part 'branch_repository.g.dart';

/// Parameter filter untuk daftar cabang.
final class BranchListParams {
  const BranchListParams({
    this.query,
    this.lat,
    this.lng,
    this.category,
    this.openNow,
    this.page = 1,
    this.limit = 10,
  });

  final String? query;
  final double? lat;
  final double? lng;
  final String? category;
  final bool? openNow;
  final int page;
  final int limit;
}

abstract interface class BranchRepository {
  Future<BranchListResponse> listBranches(BranchListParams params);
  Future<BranchDetail> getBranchDetail(String id);
}

class BranchRepositoryImpl implements BranchRepository {
  BranchRepositoryImpl(this._dio);
  final Dio _dio;

  @override
  Future<BranchListResponse> listBranches(BranchListParams params) async {
    final queryParams = <String, dynamic>{
      'page': params.page,
      'limit': params.limit,
    };
    if (params.query != null && params.query!.isNotEmpty) {
      queryParams['q'] = params.query;
    }
    if (params.lat != null) queryParams['lat'] = params.lat;
    if (params.lng != null) queryParams['lng'] = params.lng;
    if (params.category != null && params.category!.isNotEmpty) {
      queryParams['category'] = params.category;
    }
    if (params.openNow == true) queryParams['open_now'] = 'true';

    final response = await _dio.get<Map<String, dynamic>>(
      '/api/v1/public/branches',
      queryParameters: queryParams,
    );
    return BranchListResponse.fromJson(response.data!);
  }

  @override
  Future<BranchDetail> getBranchDetail(String id) async {
    final response = await _dio.get<Map<String, dynamic>>(
      '/api/v1/public/branches/$id',
    );
    return BranchDetail.fromJson(response.data!);
  }
}

@riverpod
BranchRepository branchRepository(BranchRepositoryRef ref) {
  final dio = ref.watch(dioClientProvider);
  return BranchRepositoryImpl(dio);
}
