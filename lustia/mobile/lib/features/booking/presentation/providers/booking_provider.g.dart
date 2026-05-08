// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'booking_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

String _$paymentStatusHash() => r'3deb7771ec8ffe7cad34f067af6120ed40a213da';

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

/// Provider polling status pembayaran — ADR 0015 §2.7.
///
/// Emit pertama kali sekarang juga (tanpa tunggu 5 detik), lalu setiap 5 detik.
/// Stream emits status terminal (paid / expired / failed) lalu berhenti — UI
/// listener perlu lihat nilai terminal supaya bisa navigate ke konfirmasi.
/// (Bug fix: `takeWhile(!isTerminal)` membuang elemen terminal sehingga UI
/// tidak pernah lihat 'paid'.)
///
/// Copied from [paymentStatus].
@ProviderFor(paymentStatus)
const paymentStatusProvider = PaymentStatusFamily();

/// Provider polling status pembayaran — ADR 0015 §2.7.
///
/// Emit pertama kali sekarang juga (tanpa tunggu 5 detik), lalu setiap 5 detik.
/// Stream emits status terminal (paid / expired / failed) lalu berhenti — UI
/// listener perlu lihat nilai terminal supaya bisa navigate ke konfirmasi.
/// (Bug fix: `takeWhile(!isTerminal)` membuang elemen terminal sehingga UI
/// tidak pernah lihat 'paid'.)
///
/// Copied from [paymentStatus].
class PaymentStatusFamily extends Family<AsyncValue<PaymentStatusResponse>> {
  /// Provider polling status pembayaran — ADR 0015 §2.7.
  ///
  /// Emit pertama kali sekarang juga (tanpa tunggu 5 detik), lalu setiap 5 detik.
  /// Stream emits status terminal (paid / expired / failed) lalu berhenti — UI
  /// listener perlu lihat nilai terminal supaya bisa navigate ke konfirmasi.
  /// (Bug fix: `takeWhile(!isTerminal)` membuang elemen terminal sehingga UI
  /// tidak pernah lihat 'paid'.)
  ///
  /// Copied from [paymentStatus].
  const PaymentStatusFamily();

  /// Provider polling status pembayaran — ADR 0015 §2.7.
  ///
  /// Emit pertama kali sekarang juga (tanpa tunggu 5 detik), lalu setiap 5 detik.
  /// Stream emits status terminal (paid / expired / failed) lalu berhenti — UI
  /// listener perlu lihat nilai terminal supaya bisa navigate ke konfirmasi.
  /// (Bug fix: `takeWhile(!isTerminal)` membuang elemen terminal sehingga UI
  /// tidak pernah lihat 'paid'.)
  ///
  /// Copied from [paymentStatus].
  PaymentStatusProvider call(String bookingCode) {
    return PaymentStatusProvider(bookingCode);
  }

  @override
  PaymentStatusProvider getProviderOverride(
    covariant PaymentStatusProvider provider,
  ) {
    return call(provider.bookingCode);
  }

  static const Iterable<ProviderOrFamily>? _dependencies = null;

  @override
  Iterable<ProviderOrFamily>? get dependencies => _dependencies;

  static const Iterable<ProviderOrFamily>? _allTransitiveDependencies = null;

  @override
  Iterable<ProviderOrFamily>? get allTransitiveDependencies =>
      _allTransitiveDependencies;

  @override
  String? get name => r'paymentStatusProvider';
}

/// Provider polling status pembayaran — ADR 0015 §2.7.
///
/// Emit pertama kali sekarang juga (tanpa tunggu 5 detik), lalu setiap 5 detik.
/// Stream emits status terminal (paid / expired / failed) lalu berhenti — UI
/// listener perlu lihat nilai terminal supaya bisa navigate ke konfirmasi.
/// (Bug fix: `takeWhile(!isTerminal)` membuang elemen terminal sehingga UI
/// tidak pernah lihat 'paid'.)
///
/// Copied from [paymentStatus].
class PaymentStatusProvider
    extends AutoDisposeStreamProvider<PaymentStatusResponse> {
  /// Provider polling status pembayaran — ADR 0015 §2.7.
  ///
  /// Emit pertama kali sekarang juga (tanpa tunggu 5 detik), lalu setiap 5 detik.
  /// Stream emits status terminal (paid / expired / failed) lalu berhenti — UI
  /// listener perlu lihat nilai terminal supaya bisa navigate ke konfirmasi.
  /// (Bug fix: `takeWhile(!isTerminal)` membuang elemen terminal sehingga UI
  /// tidak pernah lihat 'paid'.)
  ///
  /// Copied from [paymentStatus].
  PaymentStatusProvider(String bookingCode)
    : this._internal(
        (ref) => paymentStatus(ref as PaymentStatusRef, bookingCode),
        from: paymentStatusProvider,
        name: r'paymentStatusProvider',
        debugGetCreateSourceHash: const bool.fromEnvironment('dart.vm.product')
            ? null
            : _$paymentStatusHash,
        dependencies: PaymentStatusFamily._dependencies,
        allTransitiveDependencies:
            PaymentStatusFamily._allTransitiveDependencies,
        bookingCode: bookingCode,
      );

  PaymentStatusProvider._internal(
    super._createNotifier, {
    required super.name,
    required super.dependencies,
    required super.allTransitiveDependencies,
    required super.debugGetCreateSourceHash,
    required super.from,
    required this.bookingCode,
  }) : super.internal();

  final String bookingCode;

  @override
  Override overrideWith(
    Stream<PaymentStatusResponse> Function(PaymentStatusRef provider) create,
  ) {
    return ProviderOverride(
      origin: this,
      override: PaymentStatusProvider._internal(
        (ref) => create(ref as PaymentStatusRef),
        from: from,
        name: null,
        dependencies: null,
        allTransitiveDependencies: null,
        debugGetCreateSourceHash: null,
        bookingCode: bookingCode,
      ),
    );
  }

  @override
  AutoDisposeStreamProviderElement<PaymentStatusResponse> createElement() {
    return _PaymentStatusProviderElement(this);
  }

  @override
  bool operator ==(Object other) {
    return other is PaymentStatusProvider && other.bookingCode == bookingCode;
  }

  @override
  int get hashCode {
    var hash = _SystemHash.combine(0, runtimeType.hashCode);
    hash = _SystemHash.combine(hash, bookingCode.hashCode);

    return _SystemHash.finish(hash);
  }
}

@Deprecated('Will be removed in 3.0. Use Ref instead')
// ignore: unused_element
mixin PaymentStatusRef on AutoDisposeStreamProviderRef<PaymentStatusResponse> {
  /// The parameter `bookingCode` of this provider.
  String get bookingCode;
}

class _PaymentStatusProviderElement
    extends AutoDisposeStreamProviderElement<PaymentStatusResponse>
    with PaymentStatusRef {
  _PaymentStatusProviderElement(super.provider);

  @override
  String get bookingCode => (origin as PaymentStatusProvider).bookingCode;
}

String _$bookingWizardHash() => r'7b3e6ad7669ee20dc6d9f2e21187f49dbca8205d';

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

String _$bookingSubmitHash() => r'7bc5bc0005cc7936cb59d8a4a5ae88108e05b835';

/// Provider untuk submit booking — ADR 0015 §2.8.
/// Hanya membuat booking; tidak lagi memanggil dummy webhook.
/// Setelah submit berhasil, PaymentScreen menangani polling status.
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
