package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Minimal hand-written fakes for registration service tests.
// ---------------------------------------------------------------------------

type stubRegistrationRepo struct {
	saved              []*model.TenantRegistration
	updated            []*model.TenantRegistration
	findByID           func(ctx context.Context, id string) (*model.TenantRegistration, error)
	findPendingByEmail func(ctx context.Context, email string) (*model.TenantRegistration, error)
	findPendingBySlug  func(ctx context.Context, slug string) (*model.TenantRegistration, error)
}

func (r *stubRegistrationRepo) Save(_ context.Context, reg *model.TenantRegistration) error {
	r.saved = append(r.saved, reg)
	return nil
}
func (r *stubRegistrationRepo) FindByID(ctx context.Context, id string) (*model.TenantRegistration, error) {
	if r.findByID != nil {
		return r.findByID(ctx, id)
	}
	return nil, constants.ErrRegistrationNotFound
}
func (r *stubRegistrationRepo) FindPendingByEmail(ctx context.Context, email string) (*model.TenantRegistration, error) {
	if r.findPendingByEmail != nil {
		return r.findPendingByEmail(ctx, email)
	}
	return nil, constants.ErrRegistrationNotFound
}
func (r *stubRegistrationRepo) FindPendingBySlug(ctx context.Context, slug string) (*model.TenantRegistration, error) {
	if r.findPendingBySlug != nil {
		return r.findPendingBySlug(ctx, slug)
	}
	return nil, constants.ErrRegistrationNotFound
}
func (r *stubRegistrationRepo) List(_ context.Context, _ service.RegistrationFilter) ([]*model.TenantRegistration, int64, error) {
	return nil, 0, nil
}
func (r *stubRegistrationRepo) Update(_ context.Context, reg *model.TenantRegistration) error {
	r.updated = append(r.updated, reg)
	return nil
}

type stubTenantRepoForReg struct {
	findBySlug func(ctx context.Context, slug string) (*model.Tenant, error)
	saved      []*model.Tenant
}

func (r *stubTenantRepoForReg) FindBySlug(ctx context.Context, slug string) (*model.Tenant, error) {
	if r.findBySlug != nil {
		return r.findBySlug(ctx, slug)
	}
	return nil, constants.ErrTenantNotFound
}
func (r *stubTenantRepoForReg) FindByID(_ context.Context, _ string) (*model.Tenant, error) {
	return &model.Tenant{MaxBranches: 1}, nil
}
func (r *stubTenantRepoForReg) Save(_ context.Context, t *model.Tenant) error {
	r.saved = append(r.saved, t)
	return nil
}
func (r *stubTenantRepoForReg) List(_ context.Context, _ service.TenantFilter) ([]*service.TenantWithCounts, int64, error) {
	return nil, 0, nil
}
func (r *stubTenantRepoForReg) UpdateStatus(_ context.Context, _, _, _ string, _ *string) error {
	return nil
}
func (r *stubTenantRepoForReg) CountActiveBranches(_ context.Context, _ string) (int, error) {
	return 0, nil
}

type stubUserRepoForReg struct {
	saved []*model.User
}

func (r *stubUserRepoForReg) FindByEmail(_ context.Context, _ string) (*model.User, error) {
	return nil, constants.ErrUserNotFound
}
func (r *stubUserRepoForReg) FindByUsername(_ context.Context, _ string) (*model.User, error) {
	return nil, constants.ErrUserNotFound
}
func (r *stubUserRepoForReg) FindByID(_ context.Context, _ string) (*model.User, error) {
	return &model.User{}, nil
}
func (r *stubUserRepoForReg) FindByTenant(_ context.Context, _ string, _ service.UserFilter) ([]*model.User, int64, error) {
	return nil, 0, nil
}
func (r *stubUserRepoForReg) Save(_ context.Context, u *model.User) error {
	r.saved = append(r.saved, u)
	return nil
}
func (r *stubUserRepoForReg) Update(_ context.Context, _ *model.User) error { return nil }
func (r *stubUserRepoForReg) UpdatePassword(_ context.Context, _ string, _ string) error {
	return nil
}
func (r *stubUserRepoForReg) IncrementFailedLogin(_ context.Context, _ string, _ *time.Time) error {
	return nil
}
func (r *stubUserRepoForReg) ResetFailedLogin(_ context.Context, _ string) error { return nil }
func (r *stubUserRepoForReg) SoftDelete(_ context.Context, _ string) error       { return nil }

type stubMembershipRepoForReg struct {
	saved []*model.Membership
}

func (r *stubMembershipRepoForReg) FindByUser(_ context.Context, _ string) ([]*model.Membership, error) {
	return nil, nil
}
func (r *stubMembershipRepoForReg) FindByUserAndTenant(_ context.Context, _, _ string) (*model.Membership, error) {
	return nil, constants.ErrMembershipNotFound
}
func (r *stubMembershipRepoForReg) FindByID(_ context.Context, _ string) (*model.Membership, error) {
	return nil, constants.ErrMembershipNotFound
}
func (r *stubMembershipRepoForReg) Save(_ context.Context, m *model.Membership) error {
	r.saved = append(r.saved, m)
	return nil
}
func (r *stubMembershipRepoForReg) Update(_ context.Context, _ *model.Membership) error { return nil }
func (r *stubMembershipRepoForReg) SetStatus(_ context.Context, _ string, _ model.MembershipStatus) error {
	return nil
}
func (r *stubMembershipRepoForReg) AssignRoles(_ context.Context, _ string, _ []string, _ string) error {
	return nil
}
func (r *stubMembershipRepoForReg) AssignBranches(_ context.Context, _ string, _ []string, _ string) error {
	return nil
}
func (r *stubMembershipRepoForReg) SuspendAllForTenant(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (r *stubMembershipRepoForReg) FindActiveByTenant(_ context.Context, _ string) ([]*model.Membership, error) {
	return nil, nil
}
func (r *stubMembershipRepoForReg) GetRolesAndBranches(_ context.Context, _ string) (model.MembershipAssignments, error) {
	return model.MembershipAssignments{}, nil
}
func (r *stubMembershipRepoForReg) GetRolesAndBranchesForMemberships(_ context.Context, _ []string) (map[string]model.MembershipAssignments, error) {
	return map[string]model.MembershipAssignments{}, nil
}

type stubRoleRepoForReg struct{}

func (r *stubRoleRepoForReg) FindAll(_ context.Context) ([]*model.Role, error) { return nil, nil }
func (r *stubRoleRepoForReg) FindByIDs(_ context.Context, _ []string) ([]*model.Role, error) {
	return nil, nil
}
func (r *stubRoleRepoForReg) FindByName(_ context.Context, _ string) (*model.Role, error) {
	return &model.Role{ID: "role-tenant-admin-uuid"}, nil
}

type stubTokenRepoForReg struct{}

func (r *stubTokenRepoForReg) FindByHash(_ context.Context, _ string) (*model.RefreshToken, error) {
	return nil, constants.ErrRefreshTokenNotFound
}
func (r *stubTokenRepoForReg) Save(_ context.Context, _ *model.RefreshToken) error { return nil }
func (r *stubTokenRepoForReg) Revoke(_ context.Context, _ string, _ *string) error { return nil }
func (r *stubTokenRepoForReg) RevokeAllForUser(_ context.Context, _ string) error  { return nil }
func (r *stubTokenRepoForReg) RevokeAllForTenantUsers(_ context.Context, _ string) error {
	return nil
}
func (r *stubTokenRepoForReg) DeleteExpiredAndRevoked(_ context.Context) error { return nil }

type stubHasherForReg struct{}

func (h *stubHasherForReg) Hash(_ context.Context, p string) (string, error) { return "hashed:" + p, nil }
func (h *stubHasherForReg) Verify(_ context.Context, p, hash string) (bool, error) {
	return hash == "hashed:"+p, nil
}

type stubEmailForReg struct{ sent []service.EmailMessage }

func (e *stubEmailForReg) Send(_ context.Context, msg service.EmailMessage) error {
	e.sent = append(e.sent, msg)
	return nil
}

type stubRateLimiterForReg struct{ allow bool }

func (r *stubRateLimiterForReg) Allow(_ context.Context, _ string) bool { return r.allow }

type stubTxManagerForReg struct{}

func (t *stubTxManagerForReg) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (t *stubTxManagerForReg) SetTenantContext(_ context.Context, _, _ string) error { return nil }

type stubClockForReg struct{}

func (c *stubClockForReg) Now() time.Time { return time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC) }

// newTestRegistrationService assembles a RegistrationService with the supplied fakes.
func newTestRegistrationService(
	regRepo *stubRegistrationRepo,
	tenantRepo *stubTenantRepoForReg,
	rateLimiter *stubRateLimiterForReg,
) *service.RegistrationService {
	return service.NewRegistrationService(
		regRepo,
		tenantRepo,
		&stubUserRepoForReg{},
		&stubMembershipRepoForReg{},
		&stubRoleRepoForReg{},
		&stubTokenRepoForReg{},
		&stubHasherForReg{},
		&stubClockForReg{},
		&fakeAuditRepo{},
		&stubEmailForReg{},
		rateLimiter,
		&stubRateLimiterForReg{allow: true}, // global limiter — always allow in tests
		&stubTxManagerForReg{},
		"http://localhost:3002/login",
		"Lustia",
	)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSubmitRegistration_Success(t *testing.T) {
	t.Parallel()

	regRepo := &stubRegistrationRepo{}
	svc := newTestRegistrationService(
		regRepo,
		&stubTenantRepoForReg{},
		&stubRateLimiterForReg{allow: true},
	)

	out, err := svc.SubmitRegistration(context.Background(), service.RegistrationInput{
		CompanyName:   "Acme Wellness",
		RequestedSlug: "acme-wellness",
		Package:       "starter",
		ContactName:   "Alice Founder",
		ContactEmail:  "alice@acme-wellness.example",
		ContactPhone:  "+628123456789",
		IP:            "127.0.0.1",
	})

	require.NoError(t, err)
	assert.Equal(t, "pending", out.Status)
	assert.NotEmpty(t, out.RegistrationID)
	require.Len(t, regRepo.saved, 1)
	assert.Equal(t, "acme-wellness", regRepo.saved[0].RequestedSlug)
	assert.Equal(t, model.TenantRegistrationStatusPending, regRepo.saved[0].Status)
}

func TestSubmitRegistration_DuplicateEmail(t *testing.T) {
	t.Parallel()

	existingReg := &model.TenantRegistration{
		ID:     "existing-uuid",
		Status: model.TenantRegistrationStatusPending,
	}
	regRepo := &stubRegistrationRepo{
		findPendingByEmail: func(_ context.Context, _ string) (*model.TenantRegistration, error) {
			return existingReg, nil // found — duplicate
		},
	}

	svc := newTestRegistrationService(
		regRepo,
		&stubTenantRepoForReg{},
		&stubRateLimiterForReg{allow: true},
	)

	_, err := svc.SubmitRegistration(context.Background(), service.RegistrationInput{
		CompanyName:   "Another Company",
		RequestedSlug: "another-slug",
		Package:       "starter",
		ContactName:   "Bob",
		ContactEmail:  "alice@acme-wellness.example", // same email as existing pending
		IP:            "127.0.0.1",
	})

	require.Error(t, err)
	// SECURITY.md Phase 3 M-1: public submit returns generic ErrConflict to
	// prevent unauthenticated callers from enumerating which field is taken.
	assert.True(t, errors.Is(err, constants.ErrConflict))
	assert.Empty(t, regRepo.saved)
}

func TestSubmitRegistration_DuplicateSlug(t *testing.T) {
	t.Parallel()

	existingReg := &model.TenantRegistration{
		ID:     "existing-uuid",
		Status: model.TenantRegistrationStatusPending,
	}
	regRepo := &stubRegistrationRepo{
		findPendingBySlug: func(_ context.Context, _ string) (*model.TenantRegistration, error) {
			return existingReg, nil // found — duplicate slug
		},
	}

	svc := newTestRegistrationService(
		regRepo,
		&stubTenantRepoForReg{},
		&stubRateLimiterForReg{allow: true},
	)

	_, err := svc.SubmitRegistration(context.Background(), service.RegistrationInput{
		CompanyName:   "Acme Copy",
		RequestedSlug: "acme-wellness", // taken by pending registration
		Package:       "starter",
		ContactName:   "Carol",
		ContactEmail:  "carol@other.example",
		IP:            "127.0.0.1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrConflict))
}

func TestSubmitRegistration_SlugTakenByApprovedTenant(t *testing.T) {
	t.Parallel()

	regRepo := &stubRegistrationRepo{}
	tenantRepo := &stubTenantRepoForReg{
		findBySlug: func(_ context.Context, _ string) (*model.Tenant, error) {
			// Slug already belongs to an approved tenant.
			return &model.Tenant{ID: "approved-tenant-id", Slug: "taken-slug", Status: model.TenantStatusActive}, nil
		},
	}

	svc := newTestRegistrationService(regRepo, tenantRepo, &stubRateLimiterForReg{allow: true})

	_, err := svc.SubmitRegistration(context.Background(), service.RegistrationInput{
		CompanyName:   "Copycat Corp",
		RequestedSlug: "taken-slug",
		Package:       "starter",
		ContactName:   "Dave",
		ContactEmail:  "dave@copycat.example",
		IP:            "127.0.0.1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrConflict))
	assert.Empty(t, regRepo.saved)
}

func TestSubmitRegistration_RateLimited(t *testing.T) {
	t.Parallel()

	regRepo := &stubRegistrationRepo{}
	svc := newTestRegistrationService(
		regRepo,
		&stubTenantRepoForReg{},
		&stubRateLimiterForReg{allow: false}, // limiter exhausted
	)

	_, err := svc.SubmitRegistration(context.Background(), service.RegistrationInput{
		CompanyName:   "Fast Fingers LLC",
		RequestedSlug: "fast-fingers",
		Package:       "starter",
		ContactName:   "Eve",
		ContactEmail:  "eve@fast.example",
		IP:            "10.0.0.1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrRateLimited))
	assert.Empty(t, regRepo.saved)
}
