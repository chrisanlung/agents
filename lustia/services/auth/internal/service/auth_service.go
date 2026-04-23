package service

import (
	"context"
	"fmt"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

const (
	maxFailedLogins = 5
	lockoutDuration = 15 * time.Minute
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 14 * 24 * time.Hour
)

// AuthService groups the core authentication operations: Login, Refresh, Logout,
// and SwitchTenant.
type AuthService struct {
	users       UserRepository
	memberships MembershipRepository
	tenants     TenantRepository
	tokens      RefreshTokenRepository
	hasher      PasswordHasher
	issuer      TokenIssuer
	clock       Clock
	audit       AuditRepository
	rateLimiter RateLimiter
	tx          TxManager
}

// NewAuthService constructs an AuthService.
func NewAuthService(
	users UserRepository,
	memberships MembershipRepository,
	tenants TenantRepository,
	tokens RefreshTokenRepository,
	hasher PasswordHasher,
	issuer TokenIssuer,
	clock Clock,
	audit AuditRepository,
	rateLimiter RateLimiter,
	tx TxManager,
) *AuthService {
	return &AuthService{
		users:       users,
		memberships: memberships,
		tenants:     tenants,
		tokens:      tokens,
		hasher:      hasher,
		issuer:      issuer,
		clock:       clock,
		audit:       audit,
		rateLimiter: rateLimiter,
		tx:          tx,
	}
}

// Login orchestrates the email+password authentication flow.
//
// The flow has three branches depending on the user's membership state:
//
//  1. user.IsSuperAdmin = true → scope=platform JWT (tenant_id="__platform__",
//     roles=["super_admin"], membership_id=nil). Refresh token: tenant_id=NULL.
//
//  2. Exactly one active membership → auto-switch: scope=tenant JWT with that
//     membership's details. Refresh token: tenant_id = selected membership's
//     tenant_id.
//
//  3. More than one active membership → scope=user JWT (tenant_id="", roles=[],
//     membership_id=nil). Refresh token: tenant_id=NULL. The caller must call
//     POST /auth/select-tenant before accessing protected endpoints.
//
// The login endpoint sets app.current_tenant = '__platform__' before the
// user lookup so RLS lets us read the global user row regardless of tenant.
// Membership data is loaded separately (memberships table has its own RLS
// policy that allows a user to always see their own rows).
func (s *AuthService) Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	// Rate limit by IP.
	if !s.rateLimiter.Allow(ctx, "login:"+in.IP) {
		return LoginOutput{}, constants.ErrRateLimited
	}

	// Switch to the platform context so the RLS-protected "user" table lookup
	// can see any user row regardless of tenant affiliation. The global email
	// uniqueness constraint (migration 009) means the email alone is sufficient.
	if err := s.tx.SetTenantContext(ctx, constants.PlatformTenantSentinel, ""); err != nil {
		return LoginOutput{}, fmt.Errorf("switch tenant context for login: %w", err)
	}

	user, err := s.users.FindByEmail(ctx, in.Email)
	if err != nil {
		// ErrUserNotFound maps to INVALID_CREDENTIALS (anti-enumeration).
		return LoginOutput{}, constants.ErrInvalidCredentials
	}

	now := s.clock.Now()
	if user.IsLocked(now) {
		return LoginOutput{}, constants.ErrAccountLocked
	}
	if !user.IsActive {
		return LoginOutput{}, constants.ErrAccountInactive
	}

	match, err := s.hasher.Verify(ctx, in.Password, user.PasswordHash)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("verify password: %w", err)
	}
	if !match {
		newCount := user.FailedLoginCount + 1
		var lockUntil *time.Time
		if newCount >= maxFailedLogins {
			t := now.Add(lockoutDuration)
			lockUntil = &t
		}
		_ = s.users.IncrementFailedLogin(ctx, user.ID, lockUntil)
		return LoginOutput{}, constants.ErrInvalidCredentials
	}

	_ = s.users.ResetFailedLogin(ctx, user.ID)

	// -------------------------------------------------------------------------
	// Branch 1: super-admin → scope=platform
	// ADR §2.6: super admins always land in platform scope on login, even if
	// they happen to have memberships.
	// -------------------------------------------------------------------------
	if user.IsSuperAdmin {
		return s.issuePlatformLogin(ctx, user, in, now)
	}

	// -------------------------------------------------------------------------
	// Load active memberships for tenant-scoped branching.
	// -------------------------------------------------------------------------
	allMemberships, err := s.memberships.FindByUser(ctx, user.ID)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("load memberships: %w", err)
	}

	var activeMemberships []*model.Membership
	for _, m := range allMemberships {
		if m.IsActive() {
			activeMemberships = append(activeMemberships, m)
		}
	}

	summaries := toMembershipSummaries(activeMemberships)

	switch len(activeMemberships) {
	case 0:
		// No active memberships → user has been suspended across every tenant
		// (e.g. via tenant deactivation cascade). Reject the login entirely
		// rather than issue a scope=user token with empty memberships, which
		// would let a deactivated-tenant admin keep obtaining fresh JWTs.
		// SECURITY.md Phase 3 bug BUG-T7-B.
		return LoginOutput{}, constants.ErrAccountInactive
	case 1:
		// Branch 2: exactly one active membership → auto-select, scope=tenant.
		return s.issueTenantLogin(ctx, user, activeMemberships[0], summaries, in, now)
	default:
		// Branch 3: multiple active memberships → scope=user, caller picks tenant.
		return s.issueUserScopeLogin(ctx, user, summaries, in, now)
	}
}

// issuePlatformLogin handles Branch 1 (super-admin).
func (s *AuthService) issuePlatformLogin(ctx context.Context, user *model.User, in LoginInput, now time.Time) (LoginOutput, error) {
	claims := model.AccessClaims{
		Issuer:             constants.JWTIssuer,
		Subject:            user.ID,
		Audience:           []string{constants.JWTAudience},
		IssuedAt:           now,
		ExpiresAt:          now.Add(accessTokenTTL),
		JWTID:              uuid.New().String(),
		Scope:              model.ScopePlatform,
		TenantID:           constants.PlatformTenantSentinel,
		MembershipID:       nil,
		Roles:              []string{constants.RoleCodeSuperAdmin},
		Permissions:        []string{},
		Branches:           []string{},
		Email:              user.Email,
		FullName:           user.FullName,
		MustChangePassword: user.MustChangePassword,
	}

	accessToken, err := s.issuer.IssueAccessToken(ctx, claims)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("issue platform access token: %w", err)
	}

	// Refresh token for platform scope: tenant_id = NULL (not "__platform__").
	// The refresh flow detects NULL tenant_id and re-issues a platform token.
	rt, rawRefresh, err := s.saveRefreshToken(ctx, user.ID, nil, in, now)
	if err != nil {
		return LoginOutput{}, err
	}

	s.emitAuditLogin(user.ID, nil, in.IP)

	return LoginOutput{
		AccessToken:        accessToken,
		RefreshToken:       rawRefresh,
		ExpiresAt:          rt.ExpiresAt,
		Scope:              string(model.ScopePlatform),
		User:               toUserProfile(user),
		Memberships:        []MembershipSummary{},
		ActiveMembershipID: nil,
	}, nil
}

// issueTenantLogin handles Branch 2 (single active membership).
func (s *AuthService) issueTenantLogin(ctx context.Context, user *model.User, m *model.Membership, summaries []MembershipSummary, in LoginInput, now time.Time) (LoginOutput, error) {
	// Switch RLS context to the selected tenant before issuing the token so
	// any subsequent reads within this request are tenant-scoped.
	if err := s.tx.SetTenantContext(ctx, m.TenantID, user.ID); err != nil {
		return LoginOutput{}, fmt.Errorf("switch tenant context for tenant login: %w", err)
	}

	membershipID := m.ID
	claims := model.AccessClaims{
		Issuer:             constants.JWTIssuer,
		Subject:            user.ID,
		Audience:           []string{constants.JWTAudience},
		IssuedAt:           now,
		ExpiresAt:          now.Add(accessTokenTTL),
		JWTID:              uuid.New().String(),
		Scope:              model.ScopeTenant,
		TenantID:           m.TenantID,
		MembershipID:       &membershipID,
		Roles:              m.RoleNames(),
		Permissions:        m.PermissionCodes(),
		Branches:           m.BranchIDs(),
		Email:              user.Email,
		FullName:           user.FullName,
		MustChangePassword: user.MustChangePassword,
	}

	accessToken, err := s.issuer.IssueAccessToken(ctx, claims)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("issue tenant access token: %w", err)
	}

	// Refresh token for tenant scope: tenant_id = membership's tenant_id.
	tenantID := m.TenantID
	rt, rawRefresh, err := s.saveRefreshToken(ctx, user.ID, &tenantID, in, now)
	if err != nil {
		return LoginOutput{}, err
	}

	s.emitAuditLogin(user.ID, &tenantID, in.IP)

	return LoginOutput{
		AccessToken:        accessToken,
		RefreshToken:       rawRefresh,
		ExpiresAt:          rt.ExpiresAt,
		Scope:              string(model.ScopeTenant),
		User:               toUserProfile(user),
		Memberships:        summaries,
		ActiveMembershipID: &membershipID,
	}, nil
}

// issueUserScopeLogin handles Branch 3 (zero or multiple active memberships).
func (s *AuthService) issueUserScopeLogin(ctx context.Context, user *model.User, summaries []MembershipSummary, in LoginInput, now time.Time) (LoginOutput, error) {
	claims := model.AccessClaims{
		Issuer:             constants.JWTIssuer,
		Subject:            user.ID,
		Audience:           []string{constants.JWTAudience},
		IssuedAt:           now,
		ExpiresAt:          now.Add(accessTokenTTL),
		JWTID:              uuid.New().String(),
		Scope:              model.ScopeUser,
		TenantID:           "",
		MembershipID:       nil,
		Roles:              []string{},
		Permissions:        []string{},
		Branches:           []string{},
		Email:              user.Email,
		FullName:           user.FullName,
		MustChangePassword: user.MustChangePassword,
	}

	accessToken, err := s.issuer.IssueAccessToken(ctx, claims)
	if err != nil {
		return LoginOutput{}, fmt.Errorf("issue user-scope access token: %w", err)
	}

	// Refresh token for user scope: tenant_id = NULL (no tenant selected yet).
	rt, rawRefresh, err := s.saveRefreshToken(ctx, user.ID, nil, in, now)
	if err != nil {
		return LoginOutput{}, err
	}

	s.emitAuditLogin(user.ID, nil, in.IP)

	return LoginOutput{
		AccessToken:        accessToken,
		RefreshToken:       rawRefresh,
		ExpiresAt:          rt.ExpiresAt,
		Scope:              string(model.ScopeUser),
		User:               toUserProfile(user),
		Memberships:        summaries,
		ActiveMembershipID: nil,
	}, nil
}

// SwitchTenant handles POST /auth/select-tenant.
//
// Flow:
//  1. Load the user by CallerUserID.
//  2. Load the membership for (user, tenant) — 403 FORBIDDEN if missing or not active.
//  3. Load the tenant — 403 TENANT_INACTIVE if the tenant is not active.
//  4. Revoke the old refresh token and issue a new scoped one atomically.
//  5. Issue a fresh scope=tenant access token.
//  6. Audit log auth.tenant_switch.
//
// NOTE (ADR §2.6): A commented hook for switching back to platform scope
// via { "platform": true } is intentionally left out until Phase 2 MVP is done.
func (s *AuthService) SwitchTenant(ctx context.Context, in SwitchTenantInput) (SwitchTenantOutput, error) {
	now := s.clock.Now()

	user, err := s.users.FindByID(ctx, in.CallerUserID)
	if err != nil {
		return SwitchTenantOutput{}, fmt.Errorf("load user for switch-tenant: %w", err)
	}
	if !user.IsActive {
		return SwitchTenantOutput{}, constants.ErrAccountInactive
	}

	membership, err := s.memberships.FindByUserAndTenant(ctx, user.ID, in.TenantID)
	if err != nil {
		// Any error (not found, inactive, etc.) surfaces as FORBIDDEN to prevent
		// tenant enumeration via this endpoint.
		return SwitchTenantOutput{}, constants.ErrPermissionDenied
	}
	if !membership.IsActive() {
		return SwitchTenantOutput{}, constants.ErrPermissionDenied
	}

	tenant, err := s.tenants.FindByID(ctx, in.TenantID)
	if err != nil {
		return SwitchTenantOutput{}, constants.ErrPermissionDenied
	}
	if !tenant.IsActive() {
		return SwitchTenantOutput{}, constants.ErrTenantInactive
	}

	// Switch RLS context to the selected tenant.
	if err := s.tx.SetTenantContext(ctx, in.TenantID, user.ID); err != nil {
		return SwitchTenantOutput{}, fmt.Errorf("switch tenant context: %w", err)
	}

	// Rotate refresh token atomically.
	newRawRefresh := helper.GenerateOpaqueToken()
	newHash := helper.HashToken(newRawRefresh)
	newRTID := uuid.New().String()

	tenantID := in.TenantID
	newRT := &model.RefreshToken{
		ID:        newRTID,
		UserID:    user.ID,
		TenantID:  &tenantID,
		TokenHash: newHash,
		IssuedAt:  now,
		ExpiresAt: now.Add(refreshTokenTTL),
		CreatedAt: now,
	}
	if in.UserAgent != "" {
		newRT.UserAgent = helper.StrPtr(in.UserAgent)
	}
	if in.IP != "" {
		newRT.IP = helper.StrPtr(in.IP)
	}

	var oldRTID string
	if in.OldRefreshToken != "" {
		oldHash := helper.HashToken(in.OldRefreshToken)
		oldRT, findErr := s.tokens.FindByHash(ctx, oldHash)
		if findErr == nil && !oldRT.IsRevoked() {
			oldRTID = oldRT.ID
		}
	}

	if err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.tokens.Save(txCtx, newRT); err != nil {
			return fmt.Errorf("save new refresh token: %w", err)
		}
		if oldRTID != "" {
			if err := s.tokens.Revoke(txCtx, oldRTID, &newRTID); err != nil {
				return fmt.Errorf("revoke old refresh token: %w", err)
			}
		}
		return nil
	}); err != nil {
		return SwitchTenantOutput{}, fmt.Errorf("switch-tenant token rotation: %w", err)
	}

	membershipID := membership.ID
	claims := model.AccessClaims{
		Issuer:             constants.JWTIssuer,
		Subject:            user.ID,
		Audience:           []string{constants.JWTAudience},
		IssuedAt:           now,
		ExpiresAt:          now.Add(accessTokenTTL),
		JWTID:              uuid.New().String(),
		Scope:              model.ScopeTenant,
		TenantID:           in.TenantID,
		MembershipID:       &membershipID,
		Roles:              membership.RoleNames(),
		Permissions:        membership.PermissionCodes(),
		Branches:           membership.BranchIDs(),
		Email:              user.Email,
		FullName:           user.FullName,
		MustChangePassword: user.MustChangePassword,
	}

	accessToken, err := s.issuer.IssueAccessToken(ctx, claims)
	if err != nil {
		return SwitchTenantOutput{}, fmt.Errorf("issue select-tenant access token: %w", err)
	}

	go func() {
		uid := user.ID
		tid := in.TenantID
		_ = s.audit.Append(context.Background(), AuditEntry{
			TenantID:     &tid,
			ActorUserID:  &uid,
			Action:       "auth.tenant_switch",
			ResourceType: "membership",
			ResourceID:   membership.ID,
			Meta: map[string]interface{}{
				"ip":        in.IP,
				"tenant_id": in.TenantID,
			},
		})
	}()

	return SwitchTenantOutput{
		AccessToken:        accessToken,
		RefreshToken:       newRawRefresh,
		ExpiresAt:          newRT.ExpiresAt,
		Scope:              string(model.ScopeTenant),
		ActiveMembershipID: membership.ID,
		Membership: MembershipSummary{
			MembershipID: membership.ID,
			TenantID:     membership.TenantID,
			TenantName:   membership.Tenant.Name,
			TenantSlug:   membership.Tenant.Slug,
			Roles:        membership.RoleNames(),
			Branches:     membership.BranchIDs(),
			Status:       string(membership.Status),
		},
	}, nil
}

// Refresh orchestrates single-use refresh token rotation.
//
// Refresh token semantics (tenant_id column):
//   - scope=platform  → NULL  (re-issues a scope=platform token)
//   - scope=tenant    → UUID  (re-issues a scope=tenant token for that tenant)
//   - scope=user      → NULL  (re-issues a scope=user token — caller still
//     needs to call /auth/select-tenant)
//
// The scope is derived from whether rt.TenantID is non-nil AND whether the
// resolved user is a super-admin. This means:
//   - NULL + super-admin  → platform
//   - UUID + any user     → tenant
//   - NULL + non-super    → user
func (s *AuthService) Refresh(ctx context.Context, in RefreshInput) (RefreshOutput, error) {
	hash := helper.HashToken(in.RefreshToken)
	now := s.clock.Now()

	// refresh_token SELECT policy is permissive — the hash itself is the
	// capability. Lookup works regardless of the middleware-installed default
	// tenant context.
	rt, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		return RefreshOutput{}, constants.ErrRefreshTokenNotFound
	}
	if rt.IsRevoked() {
		return RefreshOutput{}, constants.ErrRefreshTokenRevoked
	}
	if rt.IsExpired(now) {
		return RefreshOutput{}, constants.ErrRefreshTokenExpired
	}

	// Switch the tenant context to platform so we can read the user row.
	// For tenant-scoped tokens we also need membership data, but we switch
	// to __platform__ first (user is always visible there), then reload.
	if err := s.tx.SetTenantContext(ctx, constants.PlatformTenantSentinel, rt.UserID); err != nil {
		return RefreshOutput{}, fmt.Errorf("switch tenant context for refresh: %w", err)
	}

	user, err := s.users.FindByID(ctx, rt.UserID)
	if err != nil {
		return RefreshOutput{}, fmt.Errorf("load user for refresh: %w", err)
	}
	if !user.IsActive {
		return RefreshOutput{}, constants.ErrAccountInactive
	}

	// Determine scope from the refresh token's tenant_id + user flags.
	var (
		scope              model.TokenScope
		tenantIDStr        string
		membershipID       *string
		roles              []string
		permissions        []string
		branches           []string
		newRTTenantID      *string // what to store on the new refresh_token row
		activeMembershipID *string
	)

	switch {
	case user.IsSuperAdmin && rt.TenantID == nil:
		// Platform scope — super-admin with no tenant in refresh token.
		scope = model.ScopePlatform
		tenantIDStr = constants.PlatformTenantSentinel
		roles = []string{constants.RoleCodeSuperAdmin}
		permissions = []string{}
		branches = []string{}
		newRTTenantID = nil

	case rt.TenantID != nil:
		// Tenant scope — refresh token carries a tenant_id.
		scope = model.ScopeTenant
		tenantIDStr = *rt.TenantID

		// Load the membership for claims re-population.
		membership, mErr := s.memberships.FindByUserAndTenant(ctx, user.ID, tenantIDStr)
		if mErr != nil || !membership.IsActive() {
			// Membership was removed or suspended — downgrade to user scope.
			scope = model.ScopeUser
			tenantIDStr = ""
			roles = []string{}
			permissions = []string{}
			branches = []string{}
			newRTTenantID = nil
			break
		}
		mid := membership.ID
		membershipID = &mid
		activeMembershipID = &mid
		roles = membership.RoleNames()
		permissions = membership.PermissionCodes()
		branches = membership.BranchIDs()
		newRTTenantID = rt.TenantID

	default:
		// User scope — no tenant selected yet.
		scope = model.ScopeUser
		tenantIDStr = ""
		roles = []string{}
		permissions = []string{}
		branches = []string{}
		newRTTenantID = nil
	}

	newRawRefresh := helper.GenerateOpaqueToken()
	newHash := helper.HashToken(newRawRefresh)
	newRTID := uuid.New().String()

	newRT := &model.RefreshToken{
		ID:        newRTID,
		UserID:    user.ID,
		TenantID:  newRTTenantID,
		TokenHash: newHash,
		IssuedAt:  now,
		ExpiresAt: now.Add(refreshTokenTTL),
		CreatedAt: now,
	}
	if in.UserAgent != "" {
		newRT.UserAgent = helper.StrPtr(in.UserAgent)
	}
	if in.IP != "" {
		newRT.IP = helper.StrPtr(in.IP)
	}

	if err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.tokens.Save(txCtx, newRT); err != nil {
			return fmt.Errorf("save new refresh token: %w", err)
		}
		if err := s.tokens.Revoke(txCtx, rt.ID, &newRTID); err != nil {
			return fmt.Errorf("revoke old refresh token: %w", err)
		}
		return nil
	}); err != nil {
		return RefreshOutput{}, fmt.Errorf("refresh rotation tx: %w", err)
	}

	claims := model.AccessClaims{
		Issuer:             constants.JWTIssuer,
		Subject:            user.ID,
		Audience:           []string{constants.JWTAudience},
		IssuedAt:           now,
		ExpiresAt:          now.Add(accessTokenTTL),
		JWTID:              uuid.New().String(),
		Scope:              scope,
		TenantID:           tenantIDStr,
		MembershipID:       membershipID,
		Roles:              roles,
		Permissions:        permissions,
		Branches:           branches,
		Email:              user.Email,
		FullName:           user.FullName,
		MustChangePassword: user.MustChangePassword,
	}

	accessToken, err := s.issuer.IssueAccessToken(ctx, claims)
	if err != nil {
		return RefreshOutput{}, fmt.Errorf("issue access token on refresh: %w", err)
	}

	go func() {
		uid := user.ID
		_ = s.audit.Append(context.Background(), AuditEntry{
			TenantID:     newRTTenantID,
			ActorUserID:  &uid,
			Action:       "auth.token_refresh",
			ResourceType: "refresh_token",
			ResourceID:   rt.ID,
			Meta:         map[string]interface{}{"ip": in.IP, "replaced_by": newRTID},
		})
	}()

	return RefreshOutput{
		AccessToken:        accessToken,
		RefreshToken:       newRawRefresh,
		ExpiresAt:          newRT.ExpiresAt,
		Scope:              string(scope),
		ActiveMembershipID: activeMembershipID,
	}, nil
}

// Logout revokes the caller's current refresh token.
// The access token is short-lived (15 min) and not blocklisted in Phase 2.
// TODO(phase-3): blocklist access token jti until its exp using Redis.
func (s *AuthService) Logout(ctx context.Context, in LogoutInput) error {
	if in.RefreshToken == "" {
		return nil
	}

	hash := helper.HashToken(in.RefreshToken)
	rt, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		// Token not found — treat as already revoked; not an error for the caller.
		return nil
	}
	if rt.IsRevoked() {
		return nil
	}

	// Verify the token belongs to the authenticated caller.
	if rt.UserID != in.CallerUserID {
		return constants.ErrPermissionDenied
	}

	if err := s.tokens.Revoke(ctx, rt.ID, nil); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}

	go func() {
		uid := in.CallerUserID
		_ = s.audit.Append(context.Background(), AuditEntry{
			ActorUserID:  &uid,
			Action:       "auth.logout",
			ResourceType: "refresh_token",
			ResourceID:   rt.ID,
			Meta:         map[string]interface{}{},
		})
	}()

	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// saveRefreshToken creates and persists a new refresh token row.
// tenantID may be nil for platform/user-scope tokens.
func (s *AuthService) saveRefreshToken(ctx context.Context, userID string, tenantID *string, in LoginInput, now time.Time) (*model.RefreshToken, string, error) {
	rawRefresh := helper.GenerateOpaqueToken()
	hash := helper.HashToken(rawRefresh)

	rt := &model.RefreshToken{
		ID:        uuid.New().String(),
		UserID:    userID,
		TenantID:  tenantID,
		TokenHash: hash,
		IssuedAt:  now,
		ExpiresAt: now.Add(refreshTokenTTL),
		CreatedAt: now,
	}
	if in.UserAgent != "" {
		rt.UserAgent = helper.StrPtr(in.UserAgent)
	}
	if in.IP != "" {
		rt.IP = helper.StrPtr(in.IP)
	}

	if err := s.tokens.Save(ctx, rt); err != nil {
		return nil, "", fmt.Errorf("save refresh token: %w", err)
	}
	return rt, rawRefresh, nil
}

// emitAuditLogin fires an auth.login audit event in a goroutine.
func (s *AuthService) emitAuditLogin(userID string, tenantID *string, ip string) {
	go func() {
		uid := userID
		_ = s.audit.Append(context.Background(), AuditEntry{
			TenantID:     tenantID,
			ActorUserID:  &uid,
			Action:       "auth.login",
			ResourceType: "user",
			ResourceID:   userID,
			Meta:         map[string]interface{}{"ip": ip},
		})
	}()
}

// toUserProfile converts a model.User to the service-layer UserProfile DTO.
func toUserProfile(u *model.User) UserProfile {
	return UserProfile{
		ID:                 u.ID,
		Email:              u.Email,
		FullName:           u.FullName,
		Phone:              helper.DerefString(u.Phone),
		AvatarURL:          helper.DerefString(u.AvatarURL),
		IsActive:           u.IsActive,
		IsSuperAdmin:       u.IsSuperAdmin,
		MustChangePassword: u.MustChangePassword,
	}
}

// toMembershipSummaries converts a slice of model.Membership pointers into
// MembershipSummary DTOs suitable for API responses.
func toMembershipSummaries(ms []*model.Membership) []MembershipSummary {
	summaries := make([]MembershipSummary, 0, len(ms))
	for _, m := range ms {
		roles := m.RoleNames()
		if roles == nil {
			roles = []string{}
		}
		branches := m.BranchIDs()
		if branches == nil {
			branches = []string{}
		}
		summaries = append(summaries, MembershipSummary{
			MembershipID: m.ID,
			TenantID:     m.TenantID,
			TenantName:   m.Tenant.Name,
			TenantSlug:   m.Tenant.Slug,
			Roles:        roles,
			Branches:     branches,
			Status:       string(m.Status),
		})
	}
	return summaries
}
