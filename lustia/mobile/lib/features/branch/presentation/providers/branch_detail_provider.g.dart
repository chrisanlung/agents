// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'branch_detail_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

String _$branchDetailHash() => r'ccc92a2cb196cd405e2fe1cea244fc5638330bf8';

/// Copied from Dart SDK
class _SystemHash {
  _SystemHash._();

  static int combine(int hash, int value) {
    // ignore: parameter_assignments
    hash = 0x1fffffff & (hash + value);
    // ignore: parameter_assignments
    hash = 0x1fffffff & (hash + ((0x0007ffff & hash) << 10));
    return hash ^ (hash >> 6);
  }

  static int finish(int hash) {
    // ignore: parameter_assignments
    hash = 0x1fffffff & (hash + ((0x03ffffff & hash) << 3));
    // ignore: parameter_assignments
    hash = hash ^ (hash >> 11);
    return 0x1fffffff & (hash + ((0x00003fff & hash) << 15));
  }
}

/// See also [branchDetail].
@ProviderFor(branchDetail)
const branchDetailProvider = BranchDetailFamily();

/// See also [branchDetail].
class BranchDetailFamily extends Family<AsyncValue<BranchDetail>> {
  /// See also [branchDetail].
  const BranchDetailFamily();

  /// See also [branchDetail].
  BranchDetailProvider call(String id) {
    return BranchDetailProvider(id);
  }

  @override
  BranchDetailProvider getProviderOverride(
    covariant BranchDetailProvider provider,
  ) {
    return call(provider.id);
  }

  static const Iterable<ProviderOrFamily>? _dependencies = null;

  @override
  Iterable<ProviderOrFamily>? get dependencies => _dependencies;

  static const Iterable<ProviderOrFamily>? _allTransitiveDependencies = null;

  @override
  Iterable<ProviderOrFamily>? get allTransitiveDependencies =>
      _allTransitiveDependencies;

  @override
  String? get name => r'branchDetailProvider';
}

/// See also [branchDetail].
class BranchDetailProvider extends AutoDisposeFutureProvider<BranchDetail> {
  /// See also [branchDetail].
  BranchDetailProvider(String id)
    : this._internal(
        (ref) => branchDetail(ref as BranchDetailRef, id),
        from: branchDetailProvider,
        name: r'branchDetailProvider',
        debugGetCreateSourceHash: const bool.fromEnvironment('dart.vm.product')
            ? null
            : _$branchDetailHash,
        dependencies: BranchDetailFamily._dependencies,
        allTransitiveDependencies:
            BranchDetailFamily._allTransitiveDependencies,
        id: id,
      );

  BranchDetailProvider._internal(
    super._createNotifier, {
    required super.name,
    required super.dependencies,
    required super.allTransitiveDependencies,
    required super.debugGetCreateSourceHash,
    required super.from,
    required this.id,
  }) : super.internal();

  final String id;

  @override
  Override overrideWith(
    FutureOr<BranchDetail> Function(BranchDetailRef provider) create,
  ) {
    return ProviderOverride(
      origin: this,
      override: BranchDetailProvider._internal(
        (ref) => create(ref as BranchDetailRef),
        from: from,
        name: null,
        dependencies: null,
        allTransitiveDependencies: null,
        debugGetCreateSourceHash: null,
        id: id,
      ),
    );
  }

  @override
  AutoDisposeFutureProviderElement<BranchDetail> createElement() {
    return _BranchDetailProviderElement(this);
  }

  @override
  bool operator ==(Object other) {
    return other is BranchDetailProvider && other.id == id;
  }

  @override
  int get hashCode {
    var hash = _SystemHash.combine(0, runtimeType.hashCode);
    hash = _SystemHash.combine(hash, id.hashCode);

    return _SystemHash.finish(hash);
  }
}

@Deprecated('Will be removed in 3.0. Use Ref instead')
// ignore: unused_element
mixin BranchDetailRef on AutoDisposeFutureProviderRef<BranchDetail> {
  /// The parameter `id` of this provider.
  String get id;
}

class _BranchDetailProviderElement
    extends AutoDisposeFutureProviderElement<BranchDetail>
    with BranchDetailRef {
  _BranchDetailProviderElement(super.provider);

  @override
  String get id => (origin as BranchDetailProvider).id;
}

// ignore_for_file: type=lint
// ignore_for_file: subtype_of_sealed_class, invalid_use_of_internal_member, invalid_use_of_visible_for_testing_member, deprecated_member_use_from_same_package
