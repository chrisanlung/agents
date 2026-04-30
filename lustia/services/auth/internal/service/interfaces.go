// Package service contains all business logic for the auth service.
// Services depend on interfaces declared in this file (consumer-owned).
// The concrete implementations live in repository/ and helper/; they satisfy
// these interfaces implicitly — no explicit declaration needed.
package service

import (
	"context"
	"io"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/model"
)

// ---------------------------------------------------------------------------
// ADR 0015 — Phase 6 Payment Provider (consumer-owned, declared here).
// ---------------------------------------------------------------------------

// PaymentProvider is the interface for payment gateway operations.
// Consumer-owned: declared in the service package; concrete implementations
// live in internal/helper/payment/ and are bridged in main.go.
//
// Methods per ADR 0015 §2.2.
type PaymentProvider interface {
	// CreateQR initiates a QRIS transaction. Returns QRIS string + provider txn id.
	CreateQR(ctx context.Context, req CreateQRRequest) (CreateQRResponse, error)

	// VerifyWebhook validates signature + parses payload.
	// Signature check happens inside the adapter before any DB access (H-3 equivalent).
	VerifyWebhook(ctx context.Context, payload []byte, headers map[string]string) (PaymentNotification, error)

	// GetStatus polls the provider for transaction state.
	// Used as a fallback if webhook was missed.
	GetStatus(ctx context.Context, providerReference string) (ProviderPaymentStatus, error)

	// ListSettlements fetches the daily settlement report.
	// Returns items eligible to mark payment_transaction.status = settled.
	ListSettlements(ctx context.Context, date time.Time) ([]SettlementItem, error)
}

// ProviderPaymentStatus is the normalised payment state returned by the provider.
type ProviderPaymentStatus string

const (
	ProviderStatusPending ProviderPaymentStatus = "pending"
	ProviderStatusPaid    ProviderPaymentStatus = "paid"
	ProviderStatusFailed  ProviderPaymentStatus = "failed"
	ProviderStatusExpired ProviderPaymentStatus = "expired"
)

// CreateQRRequest carries data to create a QRIS payment transaction.
type CreateQRRequest struct {
	ProviderReference string
	OrderID           string
	AmountIDR         int64
	CustomerName      string
	CustomerEmail     string
	CustomerPhone     string
	Description       string
	ExpiryMinutes     int
}

// CreateQRResponse is returned by PaymentProvider.CreateQR.
type CreateQRResponse struct {
	ProviderReference string
	QRString          string
	QRImageURL        string
	ExpiresAt         time.Time
}

// PaymentNotification is the normalised webhook payload.
type PaymentNotification struct {
	ProviderReference string
	Status            ProviderPaymentStatus
	ReceivedAmountIDR int64
	RawPayload        []byte
}

// SettlementItem represents one transaction in a provider daily settlement report.
type SettlementItem struct {
	ProviderReference string
	SettledAmountIDR  int64
	SettledAt         time.Time
}

// ---------------------------------------------------------------------------
// ADR 0011 — Storage abstraction (consumer-owned, declared in service package).
// ---------------------------------------------------------------------------

// Storage is the interface for object storage adapters.
// Implementations live in internal/helper/storage/ and are wired in main.go.
//
// Design rationale (ADR 0011 §2.1):
//   - Upload takes io.Reader to avoid buffering the whole file in memory.
//   - URL is context-aware and error-returning: R2/Supabase signed-URL
//     generation can fail; locking the correct signature now prevents a
//     breaking change later.
//   - Delete is best-effort; callers log and continue. Deleting a missing key
//     is not an error.
type Storage interface {
	Upload(ctx context.Context, key string, r io.Reader, mimeType string) error
	Delete(ctx context.Context, key string) error
	URL(ctx context.Context, key string) (string, error)
}

// ---------------------------------------------------------------------------
// Repository interfaces (consumer-owned — declared next to the service that
// needs them, not in the repository package).
// ---------------------------------------------------------------------------

// UserRepository is the interface for user persistence.
type UserRepository interface {
	// FindByEmail looks up a user globally by email with no tenant filter.
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByID(ctx context.Context, id string) (*model.User, error)
	FindByTenant(ctx context.Context, tenantID string, filter UserFilter) ([]*model.User, int64, error)
	// Save inserts a new user row (used by registration approval).
	Save(ctx context.Context, u *model.User) error
	Update(ctx context.Context, u *model.User) error
	UpdatePassword(ctx context.Context, id string, hash string) error
	IncrementFailedLogin(ctx context.Context, id string, lockUntil *time.Time) error
	ResetFailedLogin(ctx context.Context, id string) error
	SoftDelete(ctx context.Context, id string) error
}

// UserFilter carries optional filters for the list-users query.
type UserFilter struct {
	RoleID   *string
	BranchID *string
	IsActive *bool
	Page     int
	Limit    int
}

// MembershipRepository is the interface for membership persistence.
// Memberships are the join between users and tenants; roles and branches belong
// to a membership, not directly to a user.
type MembershipRepository interface {
	// FindByUser returns all memberships for the given user (any status).
	FindByUser(ctx context.Context, userID string) ([]*model.Membership, error)
	// FindByUserAndTenant returns the membership for a specific (user, tenant) pair.
	FindByUserAndTenant(ctx context.Context, userID, tenantID string) (*model.Membership, error)
	// FindByID returns a membership by primary key.
	FindByID(ctx context.Context, id string) (*model.Membership, error)
	// Save inserts a new membership row.
	Save(ctx context.Context, m *model.Membership) error
	// Update writes mutable columns.
	Update(ctx context.Context, m *model.Membership) error
	// SetStatus transitions a membership to a new status.
	SetStatus(ctx context.Context, id string, status model.MembershipStatus) error
	// AssignRoles replaces all role assignments for a membership.
	AssignRoles(ctx context.Context, membershipID string, roleIDs []string, assignedBy string) error
	// AssignBranches replaces all branch assignments for a membership.
	AssignBranches(ctx context.Context, membershipID string, branchIDs []string, assignedBy string) error
	// SuspendAllForTenant sets all active memberships for a tenant to 'suspended'.
	// Returns the slice of affected membership IDs for audit logging.
	SuspendAllForTenant(ctx context.Context, tenantID string) ([]string, error)
	// FindActiveByTenant returns all active memberships for a given tenant.
	FindActiveByTenant(ctx context.Context, tenantID string) ([]*model.Membership, error)
}

// TenantRepository is the interface for tenant lookups and management.
type TenantRepository interface {
	FindBySlug(ctx context.Context, slug string) (*model.Tenant, error)
	FindByID(ctx context.Context, id string) (*model.Tenant, error)
	// Save inserts a new tenant row. Used by registration approval.
	Save(ctx context.Context, t *model.Tenant) error
	// List returns a filtered, offset-paginated slice of tenants together with
	// denormalised membership_count and branch_count per row, plus total count.
	List(ctx context.Context, filter TenantFilter) ([]*TenantWithCounts, int64, error)
	// UpdateStatus writes a status transition plus the associated actor and
	// reason. reason may be nil for non-rejection transitions.
	UpdateStatus(ctx context.Context, id, newStatus, actorUserID string, reason *string) error
	// CountActiveBranches returns the number of non-deleted branches for a tenant.
	CountActiveBranches(ctx context.Context, tenantID string) (int, error)
}

// TenantFilter carries optional filters for the tenant list query.
type TenantFilter struct {
	Status  string // empty = all
	Q       string // trigram substring match on lower(name) OR lower(slug); min 2 chars enforced at controller
	Package string // empty = all
	Page    int
	Limit   int
}

// TenantWithCounts wraps a Tenant with read-only aggregate counters.
type TenantWithCounts struct {
	model.Tenant
	MembershipCount int
	BranchCount     int
}

// BranchRepository is the interface for branch persistence.
type BranchRepository interface {
	FindByID(ctx context.Context, id string) (*model.Branch, error)
	FindByTenant(ctx context.Context, tenantID string, filter BranchFilter) ([]*model.Branch, int64, error)
	Save(ctx context.Context, b *model.Branch) error
	Update(ctx context.Context, b *model.Branch) error
	UpdateStatus(ctx context.Context, id, newStatus string, activatedAt *time.Time) error
	SoftDelete(ctx context.Context, id string) error
}

// BranchFilter carries optional filters for the branch list query.
type BranchFilter struct {
	Status string // empty = all non-deleted
	Page   int
	Limit  int
}

// RegistrationRepository is the interface for tenant-registration persistence.
type RegistrationRepository interface {
	Save(ctx context.Context, r *model.TenantRegistration) error
	FindByID(ctx context.Context, id string) (*model.TenantRegistration, error)
	FindPendingByEmail(ctx context.Context, email string) (*model.TenantRegistration, error)
	FindPendingBySlug(ctx context.Context, slug string) (*model.TenantRegistration, error)
	List(ctx context.Context, filter RegistrationFilter) ([]*model.TenantRegistration, int64, error)
	Update(ctx context.Context, r *model.TenantRegistration) error
}

// RegistrationFilter carries optional filters for the registration list query.
type RegistrationFilter struct {
	Status string // empty = pending
	Page   int
	Limit  int
}

// RoleRepository is the interface for role and permission reads.
type RoleRepository interface {
	FindAll(ctx context.Context) ([]*model.Role, error)
	FindByIDs(ctx context.Context, ids []string) ([]*model.Role, error)
	// FindByName returns the role with the given name, or ErrRoleNotFound.
	FindByName(ctx context.Context, name string) (*model.Role, error)
}

// RefreshTokenRepository is the interface for refresh-token persistence.
// The table is append-only — no updates, only inserts and targeted revocations.
type RefreshTokenRepository interface {
	FindByHash(ctx context.Context, hash string) (*model.RefreshToken, error)
	Save(ctx context.Context, rt *model.RefreshToken) error
	Revoke(ctx context.Context, id string, replacedBy *string) error
	RevokeAllForUser(ctx context.Context, userID string) error
	// RevokeAllForTenantUsers revokes all refresh tokens that were issued for a
	// specific tenant scope. Used during tenant deactivation cascade.
	RevokeAllForTenantUsers(ctx context.Context, tenantID string) error
	DeleteExpiredAndRevoked(ctx context.Context) error
}

// PasswordResetRepository is the interface for password-reset token persistence.
type PasswordResetRepository interface {
	FindByHash(ctx context.Context, hash string) (*model.PasswordReset, error)
	Save(ctx context.Context, prt *model.PasswordReset) error
	MarkUsed(ctx context.Context, id string) error
	CountRecentByUser(ctx context.Context, userID string, since time.Time) (int, error)
}

// AuditRepository is the interface for writing compliance events.
type AuditRepository interface {
	// Append writes one audit_log row. Failures are non-fatal and should not
	// block the caller — the implementation logs and swallows errors.
	Append(ctx context.Context, entry AuditEntry) error
}

// AuditEntry is the data carried into an audit log row.
type AuditEntry struct {
	TenantID     *string
	ActorUserID  *string
	Action       string
	ResourceType string
	ResourceID   string
	Meta         map[string]interface{}
}

// TxManager provides transactional scoping for services that need to span
// multiple repository calls atomically, plus the ability to switch the
// PostgreSQL-side tenant context mid-request.
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
	// SetTenantContext sets app.current_tenant + app.current_user within the
	// current transaction (for RLS enforcement). Services call this after
	// resolving a tenant by slug on public endpoints to switch from the
	// default '__platform__' sentinel the middleware installed.
	SetTenantContext(ctx context.Context, tenantID, userID string) error
}

// ---------------------------------------------------------------------------
// Helper interfaces (consumer-owned).
// ---------------------------------------------------------------------------

// PasswordHasher is the interface for password hashing and verification.
type PasswordHasher interface {
	Hash(ctx context.Context, password string) (string, error)
	Verify(ctx context.Context, password, hash string) (bool, error)
}

// TokenIssuer is the interface for JWT operations.
type TokenIssuer interface {
	IssueAccessToken(ctx context.Context, claims model.AccessClaims) (string, error)
	VerifyAccessToken(ctx context.Context, tokenString string) (model.AccessClaims, error)
	JWKS(ctx context.Context) ([]byte, error)
}

// Clock is the interface for time operations.
type Clock interface {
	Now() time.Time
}

// RateLimiter is the interface for request rate limiting.
type RateLimiter interface {
	Allow(ctx context.Context, key string) bool
}

// EmailSender is the interface for dispatching transactional emails.
// Implementations may be real SMTP, a queue publisher, or a no-op in
// environments that have not wired email yet.
type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

// EmailMessage is the minimum shape a caller needs to supply.
type EmailMessage struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string // optional
}

// ---------------------------------------------------------------------------
// Phase 4 — Master Operational Data (ADR 0009).
// ---------------------------------------------------------------------------

// TherapistRepository is the interface for therapist persistence.
type TherapistRepository interface {
	// Save inserts a new therapist row.
	Save(ctx context.Context, t *model.Therapist) error
	// FindByID returns a non-deleted therapist by primary key.
	// Returns ErrTherapistNotFound when no matching row exists.
	FindByID(ctx context.Context, id string) (*model.Therapist, error)
	// FindByTenant returns an offset-paginated list of non-deleted therapists
	// for the given tenant, applying optional filters, plus the total count.
	FindByTenant(ctx context.Context, tenantID string, filter TherapistFilter) ([]*model.Therapist, int64, error)
	// Update writes the mutable profile columns of an existing therapist row.
	Update(ctx context.Context, t *model.Therapist) error
	// UpdateStatus sets is_active for a therapist row.
	UpdateStatus(ctx context.Context, id string, isActive bool) error
	// SoftDelete sets deleted_at and is_active=false on the therapist row.
	SoftDelete(ctx context.Context, id string) error
	// UpdatePhotoKey atomically swaps the photo_key for a therapist row within
	// the caller's transaction context.  Returns the old key so the caller can
	// schedule a background delete after the transaction commits.
	UpdatePhotoKey(ctx context.Context, id string, newKey *string, updatedBy string) (oldKey *string, err error)
}

// TherapistFilter carries optional filters for the therapist list query.
type TherapistFilter struct {
	BranchID  *string
	IsActive  *bool    // nil = active only
	BranchIDs []string // when non-empty, restricts to these branch IDs (branch_admin scope)
	Page      int
	Limit     int
}

// ServiceCatalogRepository is the interface for service-catalog persistence.
type ServiceCatalogRepository interface {
	// Save inserts a new service row.
	Save(ctx context.Context, s *model.ServiceCatalog) error
	// FindByID returns a non-deleted service by primary key.
	// Returns ErrServiceNotFound when no matching row exists.
	FindByID(ctx context.Context, id string) (*model.ServiceCatalog, error)
	// FindByTenant returns an offset-paginated list of non-deleted services for
	// the given tenant, applying optional filters, plus the total count.
	FindByTenant(ctx context.Context, tenantID string, filter ServiceFilter) ([]*model.ServiceCatalog, int64, error)
	// Update writes the mutable columns of an existing service row.
	Update(ctx context.Context, s *model.ServiceCatalog) error
	// UpdateStatus sets is_active for a service row.
	UpdateStatus(ctx context.Context, id string, isActive bool) error
	// SoftDelete sets deleted_at and is_active=false on the service row.
	SoftDelete(ctx context.Context, id string) error
	// FindByIDs returns all non-deleted services whose IDs appear in ids,
	// scoped to the given tenant.
	FindByIDs(ctx context.Context, tenantID string, ids []string) ([]*model.ServiceCatalog, error)
}

// ServiceFilter carries optional filters for the service list query.
type ServiceFilter struct {
	IsActive *bool   // nil = active only
	Category *string
	Page     int
	Limit    int
}

// TherapistServiceRepository is the interface for the therapist ↔ service
// mapping table.
type TherapistServiceRepository interface {
	// FindByTherapistID returns all mapping rows for a therapist (active + inactive),
	// excluding mappings to soft-deleted services.
	FindByTherapistID(ctx context.Context, therapistID string) ([]*model.TherapistService, error)
	// FindByServiceID returns all active mapping rows for a service,
	// excluding soft-deleted therapists.
	FindByServiceID(ctx context.Context, serviceID string) ([]*model.TherapistService, error)
	// ReconcileForTherapist performs the diff-and-set operation described in
	// ADR 0009 §Flag #2:
	//   - IDs in desiredIDs with no row → INSERT is_active=true
	//   - IDs in desiredIDs with is_active=false row → UPDATE is_active=true
	//   - IDs with is_active=true row not in desiredIDs → UPDATE is_active=false
	// All three operations run within the caller's transaction context.
	ReconcileForTherapist(ctx context.Context, tenantID, therapistID, callerUserID string, desiredIDs []string) error
	// DeactivateAllForTherapist sets is_active=false on every mapping row for
	// the given therapist. Used during therapist soft-delete cascade.
	DeactivateAllForTherapist(ctx context.Context, therapistID string) error
}

// TherapistAvailabilityRepository is the interface for per-therapist weekly
// schedule persistence.
type TherapistAvailabilityRepository interface {
	// FindByTherapistID returns all availability rows for a therapist, ordered
	// by day_of_week ASC, start_time ASC.
	FindByTherapistID(ctx context.Context, therapistID string) ([]*model.TherapistAvailability, error)
	// ReplaceAllForTherapist atomically deletes all existing rows for the
	// therapist and inserts the new set within a single transaction.
	// Passing an empty slice clears all availability.
	ReplaceAllForTherapist(ctx context.Context, therapistID string, rows []*model.TherapistAvailability) error
}

// ---------------------------------------------------------------------------
// ADR 0010 — Tenant-wide add-on catalog (rewritten 2026-04-24).
// ---------------------------------------------------------------------------

// AddonRepository is the interface for tenant-wide addon persistence.
type AddonRepository interface {
	// Save inserts a new addon row.
	Save(ctx context.Context, a *model.Addon) error
	// FindByID returns a non-deleted add-on by primary key.
	// Returns ErrAddonNotFound when no matching row exists.
	FindByID(ctx context.Context, id string) (*model.Addon, error)
	// FindByIDs returns non-deleted add-ons for the given IDs in a single query.
	// Used by the reorder flow to batch-validate tenant ownership inside the tx.
	FindByIDs(ctx context.Context, ids []string) ([]*model.Addon, error)
	// FindByTenant returns an offset-paginated list of non-deleted add-ons for
	// the given tenant, applying optional filters, plus the total count.
	// Ordered by sort_order ASC, created_at ASC.
	FindByTenant(ctx context.Context, tenantID string, filter AddonFilter) ([]*model.Addon, int64, error)
	// Update writes the mutable columns of an existing add-on row.
	Update(ctx context.Context, a *model.Addon) error
	// UpdateStatus sets is_active for an add-on row.
	UpdateStatus(ctx context.Context, id string, isActive bool, updatedBy string) error
	// SoftDelete sets deleted_at and is_active=false on the add-on row.
	SoftDelete(ctx context.Context, id string, updatedBy string) error
	// BulkUpdateSortOrder atomically updates sort_order for multiple add-ons
	// in the caller's transaction context. All IDs must belong to the same
	// tenant (validated by the service layer before this call).
	BulkUpdateSortOrder(ctx context.Context, items []AddonSortOrderItem) error
}

// AddonFilter carries optional filters for the tenant-wide add-on list query.
type AddonFilter struct {
	IsActive *bool // nil = all non-deleted (admin view); true = active only
	Page     int
	Limit    int
}

// AddonSortOrderItem is a single (id, sort_order) pair for the reorder bulk
// update.
type AddonSortOrderItem struct {
	ID        string
	SortOrder int
}

// ---------------------------------------------------------------------------
// ADR 0012 — Room (Ruangan) catalog.
// ---------------------------------------------------------------------------

// RoomRepository is the interface for branch-scoped room persistence.
// Consumer-owned per SOLID-I: declared here in the service package.
type RoomRepository interface {
	// Save inserts a new room row.
	Save(ctx context.Context, r *model.Room) error
	// FindByID returns a non-deleted room by primary key.
	// Returns ErrRoomNotFound when no matching row exists.
	FindByID(ctx context.Context, id string) (*model.Room, error)
	// FindByIDs returns non-deleted rooms for the given IDs in a single query.
	// Used by the reorder flow to batch-validate branch + tenant ownership.
	FindByIDs(ctx context.Context, ids []string) ([]*model.Room, error)
	// FindByTenant returns an offset-paginated list of non-deleted rooms for the
	// given tenant, applying optional filters, plus the total count.
	// Ordered by sort_order ASC, created_at ASC.
	FindByTenant(ctx context.Context, tenantID string, filter RoomFilter) ([]*model.Room, int64, error)
	// Update writes the mutable columns of an existing room row.
	Update(ctx context.Context, r *model.Room) error
	// UpdateStatus sets is_active for a room row.
	UpdateStatus(ctx context.Context, id string, isActive bool, updatedBy string) error
	// SoftDelete sets deleted_at and is_active=false on the room row.
	SoftDelete(ctx context.Context, id string, updatedBy string) error
	// BulkUpdateSortOrder atomically updates sort_order for multiple rooms
	// in the caller's transaction context. All IDs must belong to the same
	// branch + tenant (validated by the service layer before this call).
	BulkUpdateSortOrder(ctx context.Context, items []RoomSortOrderItem) error
	// UpdatePhotoKey atomically swaps the photo_key for a room row within the
	// caller's transaction context. Returns the old key so the caller can
	// schedule a background delete after the transaction commits.
	UpdatePhotoKey(ctx context.Context, id string, newKey *string, updatedBy string) (oldKey *string, err error)
}

// RoomFilter carries optional filters for the branch-scoped room list query.
type RoomFilter struct {
	BranchID *string
	IsActive *bool   // nil = all non-deleted (admin view); true = active only
	RoomType *string // nil = all types
	Page     int
	Limit    int
}

// RoomSortOrderItem is a single (id, sort_order) pair for the reorder bulk update.
type RoomSortOrderItem struct {
	ID        string
	SortOrder int
}

// ---------------------------------------------------------------------------
// ADR 0014 — Phase 5 Booking Engine.
// ---------------------------------------------------------------------------

// BookingRepository is the interface for booking persistence.
// Consumer-owned per SOLID-I: declared here in the service package.
type BookingRepository interface {
	// Save inserts a new booking row. The booking must already have TenantID,
	// Code, TotalPriceIDR, etc. set by the service layer. The DB exclusion
	// constraint (ADR 0002) fires here; callers must map the error to
	// ErrBookingSlotConflict.
	Save(ctx context.Context, b *model.Booking) error

	// SaveAddons bulk-inserts all booking_addon rows for a booking.
	SaveAddons(ctx context.Context, addons []*model.BookingAddon) error

	// FindByID returns a booking by primary key.
	// Returns ErrBookingNotFound when no matching row exists.
	FindByID(ctx context.Context, id string) (*model.Booking, error)

	// FindByCode returns a booking by its human-readable code (operator path).
	// This is the tenant-scoped lookup. code must be non-empty.
	// Returns ErrBookingNotFound when no matching row exists.
	FindByCode(ctx context.Context, code string) (*model.Booking, error)

	// FindByCodePublic returns a booking by code under the __public__ RLS sentinel.
	//
	// H-7 (SECURITY.md): This method MUST always include WHERE code = $1.
	// The public RLS policy (booking_public_select) is additive and does NOT
	// restrict to a specific code by itself — it would expose all booking rows
	// under __public__ if called without this predicate.
	// DO NOT add a parameterless variant of this method.
	// code must be non-empty; the method returns ErrBookingNotFound on missing code.
	FindByCodePublic(ctx context.Context, code string) (*model.Booking, error)

	// FindAddonsByBooking returns all booking_addon rows for the given booking ID.
	FindAddonsByBooking(ctx context.Context, bookingID string) ([]*model.BookingAddon, error)

	// FindByTenant returns an offset-paginated list of bookings for the given
	// tenant, applying optional filters, plus the total count.
	FindByTenant(ctx context.Context, tenantID string, filter BookingFilter) ([]*model.Booking, int64, error)

	// TransitionStatus performs a conditional UPDATE:
	//   UPDATE booking SET status=$new, <audit_cols>, updated_at=now()
	//   WHERE id=$1 AND status=$expected
	//
	// H-5 (SECURITY.md): The WHERE status=$expected ensures the transition is
	// atomic and conditional. Returns (rowsAffected, error). When rowsAffected==0,
	// the caller must look up current status to distinguish "already paid"
	// (idempotent) from "expired before payment confirmed".
	TransitionStatus(ctx context.Context, in TransitionStatusInput) (int64, error)

	// SweepExpired transitions all pending_payment bookings older than 15 minutes
	// to 'expired'. Returns the count of rows swept.
	// Called lazily before booking create and availability list.
	SweepExpired(ctx context.Context) (int, error)

	// FindByPaymentReference looks up a booking by its payment_reference.
	// Used by the webhook handler to resolve a Midtrans order_id → booking.
	FindByPaymentReference(ctx context.Context, reference string) (*model.Booking, error)

	// ReportSummary returns aggregate metrics for a tenant within a date range.
	ReportSummary(ctx context.Context, in BookingReportFilter) (BookingReportSummary, error)
}

// BookingFilter carries optional filters for the booking list query.
type BookingFilter struct {
	BranchID  *string
	Status    *string
	ServiceID *string
	FromDate  *string // RFC3339 date string, inclusive
	ToDate    *string // RFC3339 date string, inclusive
	Page      int
	Limit     int
}

// TransitionStatusInput carries parameters for the conditional status transition.
type TransitionStatusInput struct {
	BookingID        string
	ExpectedStatus   string
	NewStatus        string
	PaidAt           *string // RFC3339; set when transitioning to paid
	PaymentReference *string
	CancelledAt      *string // RFC3339; set when transitioning to cancelled
	CancelledBy      *string // user ID of the op who cancelled
	CancelReason     *string
	CheckedInAt      *string // RFC3339; set when transitioning to checked_in
	CheckedInBy      *string
	CompletedAt      *string // RFC3339; set when transitioning to completed
	CompletedBy      *string
}

// BookingReportFilter carries parameters for the aggregate report query.
type BookingReportFilter struct {
	TenantID string
	BranchID *string
	FromDate string // RFC3339 date string
	ToDate   string // RFC3339 date string
}

// BookingReportSummary carries aggregate booking metrics for the reports page.
type BookingReportSummary struct {
	TotalBookings   int64
	TotalPaidIDR    int64
	CompletedCount  int64
	CancelledCount  int64
	NoShowCount     int64
	ExpiredCount    int64
	NoShowRate      float64 // NoShowCount / (CompletedCount + NoShowCount), 0 if denominator is 0
}

// MidtransClient is the interface for payment gateway operations.
// Consumer-owned: declared in the service package; concrete implementations
// live in internal/helper/payment/ and are wired in main.go.
//
// Two implementations:
//   - DummyMidtransClient (dev/local only — C-2 hard-gated by factory + build tag)
//   - RealMidtransClient  (stub for now; real impl when Midtrans keys arrive)
type MidtransClient interface {
	// CreateTransaction creates a payment transaction and returns a snap token
	// and redirect URL. For the dummy adapter this returns a fake token and
	// marks the booking as paid immediately.
	CreateTransaction(ctx context.Context, req MidtransPaymentRequest) (MidtransPaymentResponse, error)

	// HandleNotification processes a webhook notification from Midtrans.
	// For the real adapter: verifies the SHA-512 signature (H-3) before any
	// DB read. Returns the normalised PaymentStatus.
	// For the dummy adapter: accepts any payload, returns StatusPaid.
	HandleNotification(ctx context.Context, n MidtransWebhookNotification) (MidtransPaymentStatus, error)
}

// MidtransPaymentStatus is the normalised payment state from the adapter.
type MidtransPaymentStatus string

const (
	MidtransStatusPending MidtransPaymentStatus = "pending"
	MidtransStatusPaid    MidtransPaymentStatus = "paid"
	MidtransStatusFailed  MidtransPaymentStatus = "failed"
	MidtransStatusExpired MidtransPaymentStatus = "expired"
)

// MidtransPaymentRequest carries the minimum data needed to create a
// Midtrans Snap transaction.
type MidtransPaymentRequest struct {
	OrderID       string // booking.code used as Midtrans order_id
	GrossAmount   int64
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	Description   string
}

// MidtransPaymentResponse is returned by CreateTransaction.
type MidtransPaymentResponse struct {
	SnapToken   string
	RedirectURL string
	Status      MidtransPaymentStatus
}

// MidtransWebhookNotification is the normalised webhook notification body.
type MidtransWebhookNotification struct {
	OrderID           string
	TransactionStatus string // "settlement" | "capture" | "deny" | "cancel" | "expire"
	StatusCode        string
	GrossAmount       string // string in Midtrans API; parse to int64 before comparison
	SignatureKey      string
	PaymentType       string
}

// ---------------------------------------------------------------------------
// ADR 0015 — Phase 6 Repository interfaces (consumer-owned).
// ---------------------------------------------------------------------------

// PaymentTransactionRepository is the interface for payment_transaction persistence.
type PaymentTransactionRepository interface {
	// Save inserts a new payment_transaction row.
	Save(ctx context.Context, txn *model.PaymentTransaction) error

	// Reset performs an UPDATE on the existing row for bookingID — used when
	// the QR has expired and the customer retries. Updates provider_reference,
	// qr_string, qr_image_url, and qr_expires_at. The UNIQUE(booking_id)
	// invariant means we can never INSERT a second row for the same booking.
	Reset(ctx context.Context, bookingID, newReference, newQRString, newQRImageURL string, newExpiresAt time.Time) error

	// FindByID returns a payment_transaction by primary key.
	FindByID(ctx context.Context, id string) (*model.PaymentTransaction, error)

	// FindByBookingID returns the payment_transaction for the given booking.
	FindByBookingID(ctx context.Context, bookingID string) (*model.PaymentTransaction, error)

	// FindByProviderReference returns the payment_transaction for the given
	// provider_reference (idempotency key for webhooks).
	FindByProviderReference(ctx context.Context, providerRef string) (*model.PaymentTransaction, error)

	// FindByTenant returns a paginated list of payment_transactions for a tenant
	// with optional filters.
	FindByTenant(ctx context.Context, tenantID string, filter PaymentTxnFilter) ([]*model.PaymentTransaction, int64, error)

	// MarkPaid atomically updates status awaiting→paid with amount + timestamps.
	// WHERE status='awaiting' protects idempotency and race conditions.
	// Returns rowsAffected so callers can detect duplicate webhooks.
	MarkPaid(ctx context.Context, providerRef string, receivedAmount int64, paidAt time.Time, rawWebhook []byte) (int64, error)

	// BulkMarkSettled updates matching rows to status=settled within the given
	// settlement batch. Returns the count of rows updated.
	BulkMarkSettled(ctx context.Context, batchID string, providerRefs []string, settledAt time.Time) (int, error)

	// BulkMarkDisbursed updates matching rows to status=disbursed within the
	// given disbursement. Must run inside the same transaction as the
	// disbursement status update (flag #4). Returns the count of rows updated.
	BulkMarkDisbursed(ctx context.Context, disbursementID string, txnIDs []string, disbursedAt time.Time) (int, error)

	// SumByTenantStatus returns the sum of received_amount_idr for a given
	// (tenantID, status) pair. Used for the balance card.
	SumByTenantStatus(ctx context.Context, tenantID string, status string) (int64, error)

	// SweepExpiredTransactions transitions awaiting rows past their qr_expires_at
	// to status=expired. Called alongside booking expiry sweep. Returns count swept.
	SweepExpiredTransactions(ctx context.Context) (int, error)
}

// PaymentTxnFilter carries optional filters for the payment transaction list.
type PaymentTxnFilter struct {
	Status   *string
	FromDate *string // RFC3339
	ToDate   *string // RFC3339
	Page     int
	Limit    int
}

// SettlementBatchRepository is the interface for settlement_batch persistence.
type SettlementBatchRepository interface {
	// Save inserts a new settlement_batch row.
	// Callers must SET LOCAL app.current_tenant = '__platform__' before this call
	// (handled by service layer via TxManager — flag #3).
	Save(ctx context.Context, batch *model.SettlementBatch) error

	// FindByID returns a settlement_batch by primary key.
	FindByID(ctx context.Context, id string) (*model.SettlementBatch, error)

	// FindByDate returns all batches whose settled_at falls on the given date.
	FindByDate(ctx context.Context, date time.Time) ([]*model.SettlementBatch, error)

	// List returns a paginated list of settlement batches.
	List(ctx context.Context, filter SettlementBatchFilter) ([]*model.SettlementBatch, int64, error)
}

// SettlementBatchFilter carries optional filters for the settlement batch list.
type SettlementBatchFilter struct {
	Provider *string
	Page     int
	Limit    int
}

// TenantDisbursementRepository is the interface for tenant_disbursement persistence.
type TenantDisbursementRepository interface {
	// Save inserts a new tenant_disbursement row (status=pending).
	Save(ctx context.Context, d *model.TenantDisbursement) error

	// FindByID returns a disbursement by primary key.
	FindByID(ctx context.Context, id string) (*model.TenantDisbursement, error)

	// FindByTenant returns a paginated list of disbursements for a tenant.
	FindByTenant(ctx context.Context, tenantID string, filter DisbursementFilter) ([]*model.TenantDisbursement, int64, error)

	// List returns a paginated list of all disbursements (platform admin view).
	List(ctx context.Context, filter DisbursementFilter) ([]*model.TenantDisbursement, int64, error)

	// UpdateStatus transitions a disbursement to a new status.
	// For the transferred transition, bankReference and notes are stored.
	// transferredByUserID is set when newStatus = "transferred".
	UpdateStatus(ctx context.Context, id, newStatus string, transferredByUserID *string, bankReference, notes *string) error
}

// DisbursementFilter carries optional filters for the disbursement list.
type DisbursementFilter struct {
	TenantID *string
	Status   *string
	Page     int
	Limit    int
}

// ---------------------------------------------------------------------------
// ADR 0015 — Service I/O types
// ---------------------------------------------------------------------------

// InitiatePaymentOutput is returned by PaymentService.InitiateForBooking.
type InitiatePaymentOutput struct {
	TransactionID     string
	ProviderReference string
	QRString          string
	QRImageURL        string
	QRExpiresAt       time.Time
}

// PaymentStatusView is returned by PaymentService.GetStatus (polling endpoint).
type PaymentStatusView struct {
	Status      string
	PaidAt      *time.Time
	QRExpiresAt time.Time
}

// BalanceSummary is returned by PaymentService.GetTenantBalance.
type BalanceSummary struct {
	// InProcessIDR = sum of received_amount where status='paid' (awaiting settlement).
	InProcessIDR int64
	// ReadyToDisburseIDR = sum of tenant_net_idr where status='settled' and no disbursement_id.
	ReadyToDisburseIDR int64
	// DisbursedIDR = sum of tenant_net_idr where status='disbursed' (all time).
	DisbursedIDR int64
}

// SettlementBatchSummary is returned by SettlementService.Reconcile.
type SettlementBatchSummary struct {
	BatchID          string
	SettledAt        time.Time
	TransactionCount int
	TotalAmountIDR   int64
	MismatchCount    int // provider items not in our DB or vice versa
}

// SettlementBatchDetail includes the summary plus matching/mismatch diagnostics.
type SettlementBatchDetail struct {
	SettlementBatchSummary
	Transactions []PaymentTxnSummary
	Mismatches   []SettlementMismatch
}

// SettlementMismatch describes a discrepancy between the provider report and our DB.
type SettlementMismatch struct {
	ProviderReference string
	Issue             string // "not_in_our_db" | "not_in_provider_report"
	AmountIDR         int64
}

// PaymentTxnSummary is a condensed view of a payment_transaction row.
type PaymentTxnSummary struct {
	ID                string
	BookingID         string
	ProviderReference string
	Status            string
	ExpectedAmountIDR int64
	ReceivedAmountIDR *int64
	PlatformFeeIDR    *int64
	TenantNetIDR      *int64
	PaidAt            *time.Time
	SettledAt         *time.Time
}

// PayoutPreview is returned by DisbursementService.CalculatePayout.
type PayoutPreview struct {
	TenantID         string
	PeriodStart      time.Time
	PeriodEnd        time.Time
	GrossAmountIDR   int64
	PlatformFeeIDR   int64
	NetAmountIDR     int64
	TransactionCount int
	Transactions     []PaymentTxnSummary
}

// CreateDisbursementInput is the input for DisbursementService.Create.
type CreateDisbursementInput struct {
	CallerUserID string
	TenantID     string
	PeriodStart  time.Time
	PeriodEnd    time.Time
}

// DisbursementDetail is the full view of a tenant_disbursement row.
type DisbursementDetail struct {
	ID               string
	TenantID         string
	PeriodStart      time.Time
	PeriodEnd        time.Time
	GrossAmountIDR   int64
	PlatformFeeIDR   int64
	NetAmountIDR     int64
	TransactionCount int
	Status           string
	BankReference    *string
	Notes            *string
	TransferredAt    *time.Time
	TransferredBy    *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Transactions     []PaymentTxnSummary
}
