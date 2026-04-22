// Package service contains all business logic for the auth service.
// Services depend on interfaces declared in this file (consumer-owned).
// The concrete implementations live in repository/ and helper/; they satisfy
// these interfaces implicitly — no explicit declaration needed.
package service

import (
	"context"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/model"
)

// ---------------------------------------------------------------------------
// Repository interfaces (consumer-owned — declared next to the service that
// needs them, not in the repository package).
// ---------------------------------------------------------------------------

// UserRepository is the interface for user persistence.
type UserRepository interface {
	// FindByEmail looks up a user globally by email with no tenant filter.
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByID(ctx context.Context, id string) (*model.User, error)
	FindByTenant(ctx context.Context, tenantID string, filter UserFilter) ([]*model.User, string, error)
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
	Cursor   string
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
}

// TenantRepository is the interface for tenant lookups.
type TenantRepository interface {
	FindBySlug(ctx context.Context, slug string) (*model.Tenant, error)
	FindByID(ctx context.Context, id string) (*model.Tenant, error)
}

// RoleRepository is the interface for role and permission reads.
type RoleRepository interface {
	FindAll(ctx context.Context) ([]*model.Role, error)
	FindByIDs(ctx context.Context, ids []string) ([]*model.Role, error)
}

// RefreshTokenRepository is the interface for refresh-token persistence.
// The table is append-only — no updates, only inserts and targeted revocations.
type RefreshTokenRepository interface {
	FindByHash(ctx context.Context, hash string) (*model.RefreshToken, error)
	Save(ctx context.Context, rt *model.RefreshToken) error
	Revoke(ctx context.Context, id string, replacedBy *string) error
	RevokeAllForUser(ctx context.Context, userID string) error
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
