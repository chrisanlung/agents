package service_test

import (
	"context"
	"encoding/json"
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
// Fakes updated for ADR 0007 + 0008: user-membership model, no tenant_slug on
// login, TxManager.SetTenantContext for RLS context switching.
// ---------------------------------------------------------------------------

type fakeUserRepo struct {
	findByEmail     func(ctx context.Context, email string) (*model.User, error)
	incrementFailed func(ctx context.Context, id string, lockUntil *time.Time) error
	resetFailed     func(ctx context.Context, id string) error
}

func (f *fakeUserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	if f.findByEmail != nil {
		return f.findByEmail(ctx, email)
	}
	return nil, constants.ErrUserNotFound
}
func (f *fakeUserRepo) FindByID(_ context.Context, _ string) (*model.User, error) {
	return &model.User{}, nil
}
func (f *fakeUserRepo) FindByTenant(_ context.Context, _ string, _ service.UserFilter) ([]*model.User, int64, error) {
	return nil, 0, nil
}
func (f *fakeUserRepo) Save(_ context.Context, _ *model.User) error                 { return nil }
func (f *fakeUserRepo) Update(_ context.Context, _ *model.User) error               { return nil }
func (f *fakeUserRepo) UpdatePassword(_ context.Context, _ string, _ string) error  { return nil }
func (f *fakeUserRepo) IncrementFailedLogin(ctx context.Context, id string, lockUntil *time.Time) error {
	if f.incrementFailed != nil {
		return f.incrementFailed(ctx, id, lockUntil)
	}
	return nil
}
func (f *fakeUserRepo) ResetFailedLogin(ctx context.Context, id string) error {
	if f.resetFailed != nil {
		return f.resetFailed(ctx, id)
	}
	return nil
}
func (f *fakeUserRepo) SoftDelete(_ context.Context, _ string) error { return nil }

type fakeMembershipRepo struct {
	byUser []*model.Membership
}

func (f *fakeMembershipRepo) FindByUser(_ context.Context, _ string) ([]*model.Membership, error) {
	return f.byUser, nil
}
func (f *fakeMembershipRepo) FindByUserAndTenant(_ context.Context, _, _ string) (*model.Membership, error) {
	return nil, nil
}
func (f *fakeMembershipRepo) FindByID(_ context.Context, _ string) (*model.Membership, error) {
	return nil, nil
}
func (f *fakeMembershipRepo) Save(_ context.Context, _ *model.Membership) error   { return nil }
func (f *fakeMembershipRepo) Update(_ context.Context, _ *model.Membership) error { return nil }
func (f *fakeMembershipRepo) SetStatus(_ context.Context, _ string, _ model.MembershipStatus) error {
	return nil
}
func (f *fakeMembershipRepo) AssignRoles(_ context.Context, _ string, _ []string, _ string) error {
	return nil
}
func (f *fakeMembershipRepo) AssignBranches(_ context.Context, _ string, _ []string, _ string) error {
	return nil
}
func (f *fakeMembershipRepo) SuspendAllForTenant(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (f *fakeMembershipRepo) FindActiveByTenant(_ context.Context, _ string) ([]*model.Membership, error) {
	return nil, nil
}

type fakeTenantRepo struct {
	findBySlug func(ctx context.Context, slug string) (*model.Tenant, error)
}

func (f *fakeTenantRepo) FindBySlug(ctx context.Context, slug string) (*model.Tenant, error) {
	if f.findBySlug != nil {
		return f.findBySlug(ctx, slug)
	}
	return nil, errors.New("not found")
}
func (f *fakeTenantRepo) FindByID(_ context.Context, _ string) (*model.Tenant, error) {
	return &model.Tenant{}, nil
}
func (f *fakeTenantRepo) Save(_ context.Context, _ *model.Tenant) error { return nil }
func (f *fakeTenantRepo) List(_ context.Context, _ service.TenantFilter) ([]*service.TenantWithCounts, int64, error) {
	return nil, 0, nil
}
func (f *fakeTenantRepo) UpdateStatus(_ context.Context, _, _, _ string, _ *string) error {
	return nil
}
func (f *fakeTenantRepo) CountActiveBranches(_ context.Context, _ string) (int, error) {
	return 0, nil
}

type fakeRefreshTokenRepo struct {
	saved []*model.RefreshToken
}

func (f *fakeRefreshTokenRepo) FindByHash(_ context.Context, _ string) (*model.RefreshToken, error) {
	return nil, constants.ErrRefreshTokenNotFound
}
func (f *fakeRefreshTokenRepo) Save(_ context.Context, rt *model.RefreshToken) error {
	f.saved = append(f.saved, rt)
	return nil
}
func (f *fakeRefreshTokenRepo) Revoke(_ context.Context, _ string, _ *string) error { return nil }
func (f *fakeRefreshTokenRepo) RevokeAllForUser(_ context.Context, _ string) error  { return nil }
func (f *fakeRefreshTokenRepo) RevokeAllForTenantUsers(_ context.Context, _ string) error {
	return nil
}
func (f *fakeRefreshTokenRepo) DeleteExpiredAndRevoked(_ context.Context) error { return nil }

type fakeHasher struct {
	hash   func(ctx context.Context, password string) (string, error)
	verify func(ctx context.Context, password, hash string) (bool, error)
}

func (f *fakeHasher) Hash(ctx context.Context, password string) (string, error) {
	if f.hash != nil {
		return f.hash(ctx, password)
	}
	return "h:" + password, nil
}
func (f *fakeHasher) Verify(ctx context.Context, password, hash string) (bool, error) {
	if f.verify != nil {
		return f.verify(ctx, password, hash)
	}
	return true, nil
}

type fakeIssuer struct {
	issue func(ctx context.Context, claims model.AccessClaims) (string, error)
}

func (f *fakeIssuer) IssueAccessToken(ctx context.Context, claims model.AccessClaims) (string, error) {
	if f.issue != nil {
		return f.issue(ctx, claims)
	}
	return "access.token.value", nil
}
func (f *fakeIssuer) VerifyAccessToken(_ context.Context, _ string) (model.AccessClaims, error) {
	return model.AccessClaims{}, nil
}
func (f *fakeIssuer) JWKS(_ context.Context) ([]byte, error) { return json.RawMessage(nil), nil }

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

type fakeAuditRepo struct{}

func (f *fakeAuditRepo) Append(_ context.Context, _ service.AuditEntry) error { return nil }

type fakeRateLimiter struct{ allow bool }

func (f *fakeRateLimiter) Allow(_ context.Context, _ string) bool { return f.allow }

type fakeTxManager struct{}

func (f *fakeTxManager) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (f *fakeTxManager) SetTenantContext(_ context.Context, _, _ string) error { return nil }

// newTestAuthService wires an AuthService with the fakes above.
func newTestAuthService(
	users *fakeUserRepo,
	memberships *fakeMembershipRepo,
	tenants *fakeTenantRepo,
	tokens *fakeRefreshTokenRepo,
	hasher *fakeHasher,
	issuer *fakeIssuer,
	clock *fakeClock,
	rateLimiter *fakeRateLimiter,
) *service.AuthService {
	return service.NewAuthService(
		users, memberships, tenants, tokens, hasher, issuer, clock,
		&fakeAuditRepo{}, rateLimiter, &fakeTxManager{},
	)
}

// ---------------------------------------------------------------------------
// Tests — covering the three login branches (platform / single-tenant / user).
// ---------------------------------------------------------------------------

func TestLogin_SuperAdmin_Platform(t *testing.T) {
	t.Parallel()

	userID := "u-super"
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

	tokenRepo := &fakeRefreshTokenRepo{}
	svc := newTestAuthService(
		&fakeUserRepo{
			findByEmail: func(_ context.Context, _ string) (*model.User, error) {
				return &model.User{
					ID:           userID,
					Email:        "admin@lustia.local",
					PasswordHash: "hashed",
					FullName:     "Super Admin",
					IsActive:     true,
					IsSuperAdmin: true,
				}, nil
			},
		},
		&fakeMembershipRepo{},
		&fakeTenantRepo{},
		tokenRepo,
		&fakeHasher{verify: func(_ context.Context, _, _ string) (bool, error) { return true, nil }},
		&fakeIssuer{},
		&fakeClock{t: now},
		&fakeRateLimiter{allow: true},
	)

	out, err := svc.Login(context.Background(), service.LoginInput{
		Email:    "admin@lustia.local",
		Password: "s3cr3t",
		IP:       "127.0.0.1",
	})

	require.NoError(t, err)
	assert.Equal(t, "access.token.value", out.AccessToken)
	assert.NotEmpty(t, out.RefreshToken)
	assert.Equal(t, "platform", out.Scope)
	assert.Nil(t, out.ActiveMembershipID)
	assert.Empty(t, out.Memberships)
	require.Len(t, tokenRepo.saved, 1)
}

func TestLogin_SingleMembership_AutoSelectsTenant(t *testing.T) {
	t.Parallel()

	userID := "u-alice"
	tenantID := "t-acme"
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

	tokenRepo := &fakeRefreshTokenRepo{}
	svc := newTestAuthService(
		&fakeUserRepo{
			findByEmail: func(_ context.Context, _ string) (*model.User, error) {
				return &model.User{
					ID:           userID,
					Email:        "alice@acme-spa.example",
					PasswordHash: "hashed",
					FullName:     "Alice",
					IsActive:     true,
				}, nil
			},
			resetFailed: func(_ context.Context, _ string) error { return nil },
		},
		&fakeMembershipRepo{
			byUser: []*model.Membership{
				{
					ID:       "m-acme",
					UserID:   userID,
					TenantID: tenantID,
					Status:   model.MembershipStatusActive,
					Tenant:   model.Tenant{ID: tenantID, Name: "Acme Spa", Slug: "acme-spa", Status: model.TenantStatusActive},
				},
			},
		},
		&fakeTenantRepo{},
		tokenRepo,
		&fakeHasher{verify: func(_ context.Context, _, _ string) (bool, error) { return true, nil }},
		&fakeIssuer{},
		&fakeClock{t: now},
		&fakeRateLimiter{allow: true},
	)

	out, err := svc.Login(context.Background(), service.LoginInput{
		Email:    "alice@acme-spa.example",
		Password: "Staff2026!",
		IP:       "127.0.0.1",
	})

	require.NoError(t, err)
	assert.Equal(t, "tenant", out.Scope)
	require.NotNil(t, out.ActiveMembershipID)
	assert.Equal(t, "m-acme", *out.ActiveMembershipID)
	require.Len(t, out.Memberships, 1)
	assert.Equal(t, "acme-spa", out.Memberships[0].TenantSlug)
}

func TestLogin_MultiMembership_ScopeUser(t *testing.T) {
	t.Parallel()

	userID := "u-alice"
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

	tokenRepo := &fakeRefreshTokenRepo{}
	svc := newTestAuthService(
		&fakeUserRepo{
			findByEmail: func(_ context.Context, _ string) (*model.User, error) {
				return &model.User{
					ID:           userID,
					Email:        "alice@example.com",
					PasswordHash: "hashed",
					FullName:     "Alice",
					IsActive:     true,
				}, nil
			},
			resetFailed: func(_ context.Context, _ string) error { return nil },
		},
		&fakeMembershipRepo{
			byUser: []*model.Membership{
				{ID: "m1", UserID: userID, TenantID: "t1", Status: model.MembershipStatusActive, Tenant: model.Tenant{ID: "t1", Name: "Acme", Slug: "acme", Status: model.TenantStatusActive}},
				{ID: "m2", UserID: userID, TenantID: "t2", Status: model.MembershipStatusActive, Tenant: model.Tenant{ID: "t2", Name: "Beauty", Slug: "beauty", Status: model.TenantStatusActive}},
			},
		},
		&fakeTenantRepo{},
		tokenRepo,
		&fakeHasher{verify: func(_ context.Context, _, _ string) (bool, error) { return true, nil }},
		&fakeIssuer{},
		&fakeClock{t: now},
		&fakeRateLimiter{allow: true},
	)

	out, err := svc.Login(context.Background(), service.LoginInput{
		Email:    "alice@example.com",
		Password: "anything",
		IP:       "127.0.0.1",
	})

	require.NoError(t, err)
	assert.Equal(t, "user", out.Scope)
	assert.Nil(t, out.ActiveMembershipID)
	require.Len(t, out.Memberships, 2)
}

func TestLogin_InvalidPassword(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	incrementCalled := false

	svc := newTestAuthService(
		&fakeUserRepo{
			findByEmail: func(_ context.Context, _ string) (*model.User, error) {
				return &model.User{
					ID:       "u1",
					Email:    "bob@example.com",
					IsActive: true,
				}, nil
			},
			incrementFailed: func(_ context.Context, _ string, _ *time.Time) error {
				incrementCalled = true
				return nil
			},
		},
		&fakeMembershipRepo{},
		&fakeTenantRepo{},
		&fakeRefreshTokenRepo{},
		&fakeHasher{verify: func(_ context.Context, _, _ string) (bool, error) { return false, nil }},
		&fakeIssuer{},
		&fakeClock{t: now},
		&fakeRateLimiter{allow: true},
	)

	_, err := svc.Login(context.Background(), service.LoginInput{
		Email:    "bob@example.com",
		Password: "wrongpass",
		IP:       "127.0.0.1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidCredentials))
	assert.True(t, incrementCalled, "failed login counter must be incremented")
}

func TestLogin_AccountLocked(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	lockedUntil := now.Add(5 * time.Minute)

	svc := newTestAuthService(
		&fakeUserRepo{
			findByEmail: func(_ context.Context, _ string) (*model.User, error) {
				return &model.User{
					ID:          "u2",
					IsActive:    true,
					LockedUntil: &lockedUntil,
				}, nil
			},
		},
		&fakeMembershipRepo{},
		&fakeTenantRepo{},
		&fakeRefreshTokenRepo{},
		&fakeHasher{},
		&fakeIssuer{},
		&fakeClock{t: now},
		&fakeRateLimiter{allow: true},
	)

	_, err := svc.Login(context.Background(), service.LoginInput{
		Email:    "locked@example.com",
		Password: "anything",
		IP:       "127.0.0.1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrAccountLocked))
}

func TestLogin_RateLimited(t *testing.T) {
	t.Parallel()

	svc := newTestAuthService(
		&fakeUserRepo{},
		&fakeMembershipRepo{},
		&fakeTenantRepo{},
		&fakeRefreshTokenRepo{},
		&fakeHasher{},
		&fakeIssuer{},
		&fakeClock{t: time.Now()},
		&fakeRateLimiter{allow: false},
	)

	_, err := svc.Login(context.Background(), service.LoginInput{
		Email:    "any@example.com",
		Password: "password",
		IP:       "10.0.0.1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrRateLimited))
}
