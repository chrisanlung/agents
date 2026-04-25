// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'booking_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

String _$bookingWizardHash() => r'aacb6ca68b7c77587fb23d091120d52dcc6f8c78';

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

abstract class _$BookingWizard
    extends BuildlessAutoDisposeNotifier<BookingWizardState> {
  late final String branchId;

  BookingWizardState build(String branchId);
}

/// See also [BookingWizard].
@ProviderFor(BookingWizard)
const bookingWizardProvider = BookingWizardFamily();

/// See also [BookingWizard].
class BookingWizardFamily extends Family<BookingWizardState> {
  /// See also [BookingWizard].
  const BookingWizardFamily();

  /// See also [BookingWizard].
  BookingWizardProvider call(String branchId) {
    return BookingWizardProvider(branchId);
  }

  @override
  BookingWizardProvider getProviderOverride(
    covariant BookingWizardProvider provider,
  ) {
    return call(provider.branchId);
  }

  static const Iterable<ProviderOrFamily>? _dependencies = null;

  @override
  Iterable<ProviderOrFamily>? get dependencies => _dependencies;

  static const Iterable<ProviderOrFamily>? _allTransitiveDependencies = null;

  @override
  Iterable<ProviderOrFamily>? get allTransitiveDependencies =>
      _allTransitiveDependencies;

  @override
  String? get name => r'bookingWizardProvider';
}

/// See also [BookingWizard].
class BookingWizardProvider
    extends AutoDisposeNotifierProviderImpl<BookingWizard, BookingWizardState> {
  /// See also [BookingWizard].
  BookingWizardProvider(String branchId)
    : this._internal(
        () => BookingWizard()..branchId = branchId,
        from: bookingWizardProvider,
        name: r'bookingWizardProvider',
        debugGetCreateSourceHash: const bool.fromEnvironment('dart.vm.product')
            ? null
            : _$bookingWizardHash,
        dependencies: BookingWizardFamily._dependencies,
        allTransitiveDependencies:
            BookingWizardFamily._allTransitiveDependencies,
        branchId: branchId,
      );

  BookingWizardProvider._internal(
    super._createNotifier, {
    required super.name,
    required super.dependencies,
    required super.allTransitiveDependencies,
    required super.debugGetCreateSourceHash,
    required super.from,
    required this.branchId,
  }) : super.internal();

  final String branchId;

  @override
  BookingWizardState runNotifierBuild(covariant BookingWizard notifier) {
    return notifier.build(branchId);
  }

  @override
  Override overrideWith(BookingWizard Function() create) {
    return ProviderOverride(
      origin: this,
      override: BookingWizardProvider._internal(
        () => create()..branchId = branchId,
        from: from,
        name: null,
        dependencies: null,
        allTransitiveDependencies: null,
        debugGetCreateSourceHash: null,
        branchId: branchId,
      ),
    );
  }

  @override
  AutoDisposeNotifierProviderElement<BookingWizard, BookingWizardState>
  createElement() {
    return _BookingWizardProviderElement(this);
  }

  @override
  bool operator ==(Object other) {
    return other is BookingWizardProvider && other.branchId == branchId;
  }

  @override
  int get hashCode {
    var hash = _SystemHash.combine(0, runtimeType.hashCode);
    hash = _SystemHash.combine(hash, branchId.hashCode);

    return _SystemHash.finish(hash);
  }
}

@Deprecated('Will be removed in 3.0. Use Ref instead')
// ignore: unused_element
mixin BookingWizardRef on AutoDisposeNotifierProviderRef<BookingWizardState> {
  /// The parameter `branchId` of this provider.
  String get branchId;
}

class _BookingWizardProviderElement
    extends
        AutoDisposeNotifierProviderElement<BookingWizard, BookingWizardState>
    with BookingWizardRef {
  _BookingWizardProviderElement(super.provider);

  @override
  String get branchId => (origin as BookingWizardProvider).branchId;
}

String _$bookingSubmitHash() => r'daf40d3c77f3deb440c08d308e3f70a9f3cad4f7';

/// Provider untuk submit booking dan dummy payment.
///
/// Copied from [BookingSubmit].
@ProviderFor(BookingSubmit)
final bookingSubmitProvider =
    AutoDisposeNotifierProvider<
      BookingSubmit,
      AsyncValue<CreateBookingResponse?>
    >.internal(
      BookingSubmit.new,
      name: r'bookingSubmitProvider',
      debugGetCreateSourceHash: const bool.fromEnvironment('dart.vm.product')
          ? null
          : _$bookingSubmitHash,
      dependencies: null,
      allTransitiveDependencies: null,
    );

typedef _$BookingSubmit =
    AutoDisposeNotifier<AsyncValue<CreateBookingResponse?>>;
// ignore_for_file: type=lint
// ignore_for_file: subtype_of_sealed_class, invalid_use_of_internal_member, invalid_use_of_visible_for_testing_member, deprecated_member_use_from_same_package
