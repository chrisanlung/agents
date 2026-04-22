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
// Minimal hand-written fakes (no mockgen dependency for Phase 2 unit tests).
// ---------------------------------------------------------------------------

type fakeUserRepo struct {
	findByEmailAndTenant func(ctx context.Context, email string, tenantID *string) (*model.User, error)
	incrementFailed      func(ctx context.Context, id string, lockUntil *time.Time) error
	resetFailed          func(ctx context.Context, id string) error
}

func (f *fakeUserRepo) FindByEmailAndTenant(ctx context.Context, email string, tenantID *string) (*model.User, error) {
	return f.findByEmailAndTenant(ctx, email, tenantID)
}
func (f *fakeUserRepo) FindByID(_ context.Context, _ string) (*model.User, error) {
	return &model.User{}, nil
}
func (f *fakeUserRepo) FindByTenant(_ context.Context, _ string, _ service.UserFilter) ([]*model.User, string, error) {
	return nil, "", nil
}
func (f *fakeUserRepo) Save(_ context.Context, _ *model.User) error { return nil }
func (f *fakeUserRepo) Update(_ context.Context, _ *model.User) error { return nil }
func (f *fakeUserRepo) UpdatePassword(_ context.Context, _ string, _ string) error { return nil }
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
func (f *fakeUserRepo) AssignRoles(_ context.Context, _ string, _ []string, _ string) error {
	return nil
}
func (f *fakeUserRepo) AssignBranches(_ context.Context, _ string, _ []string, _ string) error {
	return nil
}
func (f *fakeUserRepo) SoftDelete(_ context.Context, _ string) error { return nil }

type fakeTenantRepo struct {
	findBySlug func(ctx context.Context, slug string) (*model.Tenant, error)
}

func (f *fakeTenantRepo) FindBySlug(ctx context.Context, slug string) (*model.Tenant, error) {
	return f.findBySlug(ctx, slug)
}
func (f *fakeTenantRepo) FindByID(_ context.Context, _ string) (*model.Tenant, error) {
	return &model.Tenant{}, nil
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
func (f *fakeRefreshTokenRepo) DeleteExpiredAndRevoked(_ context.Context) error     { return nil }

type fakeHasher struct {
	hash   func(ctx context.Context, password string) (string, error)
	verify func(ctx context.Context, password, hash string) (bool, error)
}

func (f *fakeHasher) Hash(ctx context.Context, password string) (string, error) {
	return f.hash(ctx, password)
}
func (f *fakeHasher) Verify(ctx context.Context, password, hash string) (bool, error) {
	return f.verify(ctx, password, hash)
}

type fakeIssuer struct {
	issue func(ctx context.Context, claims model.AccessClaims) (string, error)
}

func (f *fakeIssuer) IssueAccessToken(ctx context.Context, claims model.AccessClaims) (string, error) {
	return f.issue(ctx, claims)
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

// newTestAuthService creates an AuthService with the provided fakes.
func newTestAuthService(
	users *fakeUserRepo,
	tenants *fakeTenantRepo,
	tokens *fakeRefreshTokenRepo,
	hasher *fakeHasher,
	issuer *fakeIssuer,
	clock *fakeClock,
	rateLimiter *fakeRateLimiter,
) *service.AuthService {
	return service.NewAuthService(
		users, tenants, tokens, hasher, issuer, clock,
		&fakeAuditRepo{}, rateLimiter, &fakeTxManager{},
	)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestLogin_GoldenPath(t *testing.T) {
	t.Parallel()

	tenantID := "tenant-uuid-1"
	userID := "user-uuid-1"
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)

	tokenRepo := &fakeRefreshTokenRepo{}
	svc := newTestAuthService(
		&fakeUserRepo{
			findByEmailAndTenant: func(_ context.Context, _ string, _ *string) (*model.User, error) {
				return &model.User{
					ID:           userID,
					TenantID:     &tenantID,
					Email:        "alice@example.com",
					PasswordHash: "hashed",
					FullName:     "Alice",
					IsActive:     true,
				}, nil
			},
			resetFailed: func(_ context.Context, _ string) error { return nil },
		},
		&fakeTenantRepo{
			findBySlug: func(_ context.Context, _ string) (*model.Tenant, error) {
				return &model.Tenant{ID: tenantID, Slug: "acme", Status: model.TenantStatusActive}, nil
			},
		},
		tokenRepo,
		&fakeHasher{
			hash:   func(_ context.Context, p string) (string, error) { return "h:" + p, nil },
			verify: func(_ context.Context, _, _ string) (bool, error) { return true, nil },
		},
		&fakeIssuer{
			issue: func(_ context.Context, _ model.AccessClaims) (string, error) { return "access.token.value", nil },
		},
		&fakeClock{t: now},
		&fakeRateLimiter{allow: true},
	)

	out, err := svc.Login(context.Background(), service.LoginInput{
		Email:      "alice@example.com",
		Password:   "password123",
		TenantSlug: "acme",
		IP:         "127.0.0.1",
	})

	require.NoError(t, err)
	assert.Equal(t, "access.token.value", out.AccessToken)
	assert.NotEmpty(t, out.RefreshToken)
	assert.Equal(t, now.Add(14*24*time.Hour), out.ExpiresAt)
	assert.Equal(t, "alice@example.com", out.User.Email)
	require.Len(t, tokenRepo.saved, 1)
}

func TestLogin_InvalidPassword(t *testing.T) {
	t.Parallel()

	tenantID := "tenant-uuid-2"
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	incrementCalled := false

	svc := newTestAuthService(
		&fakeUserRepo{
			findByEmailAndTenant: func(_ context.Context, _ string, _ *string) (*model.User, error) {
				return &model.User{
					ID:       "u1",
					TenantID: &tenantID,
					IsActive: true,
					Email:    "bob@example.com",
				}, nil
			},
			incrementFailed: func(_ context.Context, _ string, _ *time.Time) error {
				incrementCalled = true
				return nil
			},
		},
		&fakeTenantRepo{
			findBySlug: func(_ context.Context, _ string) (*model.Tenant, error) {
				return &model.Tenant{ID: tenantID, Slug: "acme", Status: model.TenantStatusActive}, nil
			},
		},
		&fakeRefreshTokenRepo{},
		&fakeHasher{
			verify: func(_ context.Context, _, _ string) (bool, error) { return false, nil },
		},
		&fakeIssuer{},
		&fakeClock{t: now},
		&fakeRateLimiter{allow: true},
	)

	_, err := svc.Login(context.Background(), service.LoginInput{
		Email:      "bob@example.com",
		Password:   "wrongpass",
		TenantSlug: "acme",
		IP:         "127.0.0.1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidCredentials))
	assert.True(t, incrementCalled, "failed login counter must be incremented")
}

func TestLogin_AccountLocked(t *testing.T) {
	t.Parallel()

	tenantID := "tenant-uuid-3"
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	lockedUntil := now.Add(5 * time.Minute)

	svc := newTestAuthService(
		&fakeUserRepo{
			findByEmailAndTenant: func(_ context.Context, _ string, _ *string) (*model.User, error) {
				return &model.User{
					ID:          "u2",
					TenantID:    &tenantID,
					IsActive:    true,
					LockedUntil: &lockedUntil,
				}, nil
			},
		},
		&fakeTenantRepo{
			findBySlug: func(_ context.Context, _ string) (*model.Tenant, error) {
				return &model.Tenant{ID: tenantID, Slug: "acme", Status: model.TenantStatusActive}, nil
			},
		},
		&fakeRefreshTokenRepo{},
		&fakeHasher{},
		&fakeIssuer{},
		&fakeClock{t: now},
		&fakeRateLimiter{allow: true},
	)

	_, err := svc.Login(context.Background(), service.LoginInput{
		Email:      "locked@example.com",
		Password:   "anything",
		TenantSlug: "acme",
		IP:         "127.0.0.1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrAccountLocked))
}

func TestLogin_RateLimited(t *testing.T) {
	t.Parallel()

	svc := newTestAuthService(
		&fakeUserRepo{},
		&fakeTenantRepo{},
		&fakeRefreshTokenRepo{},
		&fakeHasher{},
		&fakeIssuer{},
		&fakeClock{t: time.Now()},
		&fakeRateLimiter{allow: false},
	)

	_, err := svc.Login(context.Background(), service.LoginInput{
		Email:      "any@example.com",
		Password:   "password",
		TenantSlug: "acme",
		IP:         "10.0.0.1",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrRateLimited))
}
