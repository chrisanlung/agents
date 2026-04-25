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
