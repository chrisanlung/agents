// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'availability_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

String _$availabilityHash() => r'1164dd4ffae6751b62d12d84dfc510408b817259';

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

/// See also [availability].
@ProviderFor(availability)
const availabilityProvider = AvailabilityFamily();

/// See also [availability].
class AvailabilityFamily extends Family<AsyncValue<List<AvailabilitySlot>>> {
  /// See also [availability].
  const AvailabilityFamily();

  /// See also [availability].
  AvailabilityProvider call({
    required String branchId,
    required String serviceId,
    required String date,
  }) {
    return AvailabilityProvider(
      branchId: branchId,
      serviceId: serviceId,
      date: date,
    );
  }

  @override
  AvailabilityProvider getProviderOverride(
    covariant AvailabilityProvider provider,
  ) {
    return call(
      branchId: provider.branchId,
      serviceId: provider.serviceId,
      date: provider.date,
    );
  }

  static const Iterable<ProviderOrFamily>? _dependencies = null;

  @override
  Iterable<ProviderOrFamily>? get dependencies => _dependencies;

  static const Iterable<ProviderOrFamily>? _allTransitiveDependencies = null;

  @override
  Iterable<ProviderOrFamily>? get allTransitiveDependencies =>
      _allTransitiveDependencies;

  @override
  String? get name => r'availabilityProvider';
}

/// See also [availability].
class AvailabilityProvider
    extends AutoDisposeFutureProvider<List<AvailabilitySlot>> {
  /// See also [availability].
  AvailabilityProvider({
    required String branchId,
    required String serviceId,
    required String date,
  }) : this._internal(
         (ref) => availability(
           ref as AvailabilityRef,
           branchId: branchId,
           serviceId: serviceId,
           date: date,
         ),
         from: availabilityProvider,
         name: r'availabilityProvider',
         debugGetCreateSourceHash: const bool.fromEnvironment('dart.vm.product')
             ? null
             : _$availabilityHash,
         dependencies: AvailabilityFamily._dependencies,
         allTransitiveDependencies:
             AvailabilityFamily._allTransitiveDependencies,
         branchId: branchId,
         serviceId: serviceId,
         date: date,
       );

  AvailabilityProvider._internal(
    super._createNotifier, {
    required super.name,
    required super.dependencies,
    required super.allTransitiveDependencies,
    required super.debugGetCreateSourceHash,
    required super.from,
    required this.branchId,
    required this.serviceId,
    required this.date,
  }) : super.internal();

  final String branchId;
  final String serviceId;
  final String date;

  @override
  Override overrideWith(
    FutureOr<List<AvailabilitySlot>> Function(AvailabilityRef provider) create,
  ) {
    return ProviderOverride(
      origin: this,
      override: AvailabilityProvider._internal(
        (ref) => create(ref as AvailabilityRef),
        from: from,
        name: null,
        dependencies: null,
        allTransitiveDependencies: null,
        debugGetCreateSourceHash: null,
        branchId: branchId,
        serviceId: serviceId,
        date: date,
      ),
    );
  }

  @override
  AutoDisposeFutureProviderElement<List<AvailabilitySlot>> createElement() {
    return _AvailabilityProviderElement(this);
  }

  @override
  bool operator ==(Object other) {
    return other is AvailabilityProvider &&
        other.branchId == branchId &&
        other.serviceId == serviceId &&
        other.date == date;
  }

  @override
  int get hashCode {
    var hash = _SystemHash.combine(0, runtimeType.hashCode);
    hash = _SystemHash.combine(hash, branchId.hashCode);
    hash = _SystemHash.combine(hash, serviceId.hashCode);
    hash = _SystemHash.combine(hash, date.hashCode);

    return _SystemHash.finish(hash);
  }
}

@Deprecated('Will be removed in 3.0. Use Ref instead')
// ignore: unused_element
mixin AvailabilityRef on AutoDisposeFutureProviderRef<List<AvailabilitySlot>> {
  /// The parameter `branchId` of this provider.
  String get branchId;

  /// The parameter `serviceId` of this provider.
  String get serviceId;

  /// The parameter `date` of this provider.
  String get date;
}

class _AvailabilityProviderElement
    extends AutoDisposeFutureProviderElement<List<AvailabilitySlot>>
    with AvailabilityRef {
  _AvailabilityProviderElement(super.provider);

  @override
  String get branchId => (origin as AvailabilityProvider).branchId;
  @override
  String get serviceId => (origin as AvailabilityProvider).serviceId;
  @override
  String get date => (origin as AvailabilityProvider).date;
}

// ignore_for_file: type=lint
// ignore_for_file: subtype_of_sealed_class, invalid_use_of_internal_member, invalid_use_of_visible_for_testing_member, deprecated_member_use_from_same_package
