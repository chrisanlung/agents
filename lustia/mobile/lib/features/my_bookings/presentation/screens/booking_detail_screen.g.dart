// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'booking_detail_screen.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

String _$bookingDetailHash() => r'7021b3e9246e7ff2fc2a6ed2b0c36b8f6d76a63b';

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

/// See also [bookingDetail].
@ProviderFor(bookingDetail)
const bookingDetailProvider = BookingDetailFamily();

/// See also [bookingDetail].
class BookingDetailFamily extends Family<AsyncValue<PublicBookingDetail>> {
  /// See also [bookingDetail].
  const BookingDetailFamily();

  /// See also [bookingDetail].
  BookingDetailProvider call(String code) {
    return BookingDetailProvider(code);
  }

  @override
  BookingDetailProvider getProviderOverride(
    covariant BookingDetailProvider provider,
  ) {
    return call(provider.code);
  }

  static const Iterable<ProviderOrFamily>? _dependencies = null;

  @override
  Iterable<ProviderOrFamily>? get dependencies => _dependencies;

  static const Iterable<ProviderOrFamily>? _allTransitiveDependencies = null;

  @override
  Iterable<ProviderOrFamily>? get allTransitiveDependencies =>
      _allTransitiveDependencies;

  @override
  String? get name => r'bookingDetailProvider';
}

/// See also [bookingDetail].
class BookingDetailProvider
    extends AutoDisposeFutureProvider<PublicBookingDetail> {
  /// See also [bookingDetail].
  BookingDetailProvider(String code)
    : this._internal(
        (ref) => bookingDetail(ref as BookingDetailRef, code),
        from: bookingDetailProvider,
        name: r'bookingDetailProvider',
        debugGetCreateSourceHash: const bool.fromEnvironment('dart.vm.product')
            ? null
            : _$bookingDetailHash,
        dependencies: BookingDetailFamily._dependencies,
        allTransitiveDependencies:
            BookingDetailFamily._allTransitiveDependencies,
        code: code,
      );

  BookingDetailProvider._internal(
    super._createNotifier, {
    required super.name,
    required super.dependencies,
    required super.allTransitiveDependencies,
    required super.debugGetCreateSourceHash,
    required super.from,
    required this.code,
  }) : super.internal();

  final String code;

  @override
  Override overrideWith(
    FutureOr<PublicBookingDetail> Function(BookingDetailRef provider) create,
  ) {
    return ProviderOverride(
      origin: this,
      override: BookingDetailProvider._internal(
        (ref) => create(ref as BookingDetailRef),
        from: from,
        name: null,
        dependencies: null,
        allTransitiveDependencies: null,
        debugGetCreateSourceHash: null,
        code: code,
      ),
    );
  }

  @override
  AutoDisposeFutureProviderElement<PublicBookingDetail> createElement() {
    return _BookingDetailProviderElement(this);
  }

  @override
  bool operator ==(Object other) {
    return other is BookingDetailProvider && other.code == code;
  }

  @override
  int get hashCode {
    var hash = _SystemHash.combine(0, runtimeType.hashCode);
    hash = _SystemHash.combine(hash, code.hashCode);

    return _SystemHash.finish(hash);
  }
}

@Deprecated('Will be removed in 3.0. Use Ref instead')
// ignore: unused_element
mixin BookingDetailRef on AutoDisposeFutureProviderRef<PublicBookingDetail> {
  /// The parameter `code` of this provider.
  String get code;
}

class _BookingDetailProviderElement
    extends AutoDisposeFutureProviderElement<PublicBookingDetail>
    with BookingDetailRef {
  _BookingDetailProviderElement(super.provider);

  @override
  String get code => (origin as BookingDetailProvider).code;
}

// ignore_for_file: type=lint
// ignore_for_file: subtype_of_sealed_class, invalid_use_of_internal_member, invalid_use_of_visible_for_testing_member, deprecated_member_use_from_same_package
