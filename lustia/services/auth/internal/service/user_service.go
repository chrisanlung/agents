package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

const defaultListLimit = 50

// UserService handles admin-initiated user management.
type UserService struct {
	users       UserRepository
	memberships MembershipRepository
	roles       RoleRepository
	hasher      PasswordHasher
	clock       Clock
	audit       AuditRepository
	tx          TxManager
}

// NewUserService constructs a UserService.
func NewUserService(
	users UserRepository,
	memberships MembershipRepository,
	roles RoleRepository,
	hasher PasswordHasher,
	clock Clock,
	audit AuditRepository,
	tx TxManager,
) *UserService {
	return &UserService{
		users:       users,
		memberships: memberships,
		roles:       roles,
		hasher:      hasher,
		clock:       clock,
		audit:       audit,
		tx:          tx,
	}
}

// CreateUser implements the ADR §2.5 admin create-user flow.
//
// Decision tree:
//  1. Global FindByEmail for the submitted email.
//  2. If found AND active membership in caller's tenant exists → 409 CONFLICT_USER_ALREADY_IN_TENANT.
//  3. If found AND no active membership in caller's tenant → create membership only (invite).
//     must_change_password is NOT set — the existing user keeps their password.
//  4. If not found → create user + membership + roles + branches atomically.
//     must_change_password = true on new-user path only.
//
// Response carries CreatedUser=true when path 4 was taken, CreatedMembership=true
// when paths 3 or 4 were taken.
func (s *UserService) CreateUser(ctx context.Context, in CreateUserInput) (CreateUserOutput, error) {
	if len(in.RoleIDs) > 0 {
		if _, err := s.roles.FindByIDs(ctx, in.RoleIDs); err != nil {
			return CreateUserOutput{}, fmt.Errorf("validate roles: %w", err)
		}
	}

	now := s.clock.Now()

	// Step 1: global email lookup.
	existingUser, err := s.users.FindByEmail(ctx, in.Email)

	switch {
	case err == nil:
		// User found — check for existing membership in caller's tenant.
		existing, mErr := s.memberships.FindByUserAndTenant(ctx, existingUser.ID, in.CallerTenantID)
		if mErr == nil && existing.IsActive() {
			// Step 2: user already in this tenant.
			return CreateUserOutput{}, constants.ErrUserAlreadyInTenant
		}

		// Step 3: user exists globally but has no active membership here — invite.
		membership, createErr := s.createMembership(ctx, existingUser.ID, in, now)
		if createErr != nil {
			return CreateUserOutput{}, createErr
		}

		// Reload to get roles/branches populated on the membership.
		reloaded, loadErr := s.memberships.FindByID(ctx, membership.ID)
		if loadErr != nil {
			return CreateUserOutput{}, fmt.Errorf("reload membership after invite: %w", loadErr)
		}

		s.emitAuditCreateUser(existingUser.ID, in, false, true)

		return CreateUserOutput{
			User:              toUserProfileFromMembership(existingUser, reloaded),
			InitialPassword:   "", // existing user keeps their password
			CreatedUser:       false,
			CreatedMembership: true,
		}, nil

	case errors.Is(err, constants.ErrUserNotFound):
		// Step 4: brand-new user — create user + membership atomically.
		return s.createNewUserWithMembership(ctx, in, now)

	default:
		return CreateUserOutput{}, fmt.Errorf("check existing user: %w", err)
	}
}

// createMembership inserts a membership row (and role/branch assignments) for
// an already-existing user being invited into the caller's tenant.
func (s *UserService) createMembership(ctx context.Context, userID string, in CreateUserInput, now time.Time) (*model.Membership, error) {
	membershipID := uuid.New().String()
	callerID := in.CallerUserID
	membership := &model.Membership{
		ID:        membershipID,
		UserID:    userID,
		TenantID:  in.CallerTenantID,
		Status:    model.MembershipStatusActive,
		JoinedAt:  now,
		Metadata:  []byte("{}"), // membership.metadata is NOT NULL jsonb
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: &callerID,
	}

	if err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.memberships.Save(txCtx, membership); err != nil {
			return fmt.Errorf("save membership: %w", err)
		}
		if len(in.RoleIDs) > 0 {
			if err := s.memberships.AssignRoles(txCtx, membershipID, in.RoleIDs, in.CallerUserID); err != nil {
				return fmt.Errorf("assign roles: %w", err)
			}
		}
		if len(in.BranchIDs) > 0 {
			if err := s.memberships.AssignBranches(txCtx, membershipID, in.BranchIDs, in.CallerUserID); err != nil {
				return fmt.Errorf("assign branches: %w", err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return membership, nil
}

// createNewUserWithMembership handles Step 4: new user row + membership atomically.
func (s *UserService) createNewUserWithMembership(ctx context.Context, in CreateUserInput, now time.Time) (CreateUserOutput, error) {
	plainPassword := in.InitialPassword
	if plainPassword == "" {
		plainPassword = uuid.New().String()
	}

	hash, err := s.hasher.Hash(ctx, plainPassword)
	if err != nil {
		return CreateUserOutput{}, fmt.Errorf("hash initial password: %w", err)
	}

	newUser := &model.User{
		ID:                 uuid.New().String(),
		Email:              in.Email,
		PasswordHash:       hash,
		FullName:           in.FullName,
		IsActive:           true,
		IsSuperAdmin:       false,
		MustChangePassword: true, // force rotation on first login for admin-created users
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if in.Phone != "" {
		newUser.Phone = helper.StrPtr(in.Phone)
	}

	membershipID := uuid.New().String()
	callerID := in.CallerUserID
	membership := &model.Membership{
		ID:        membershipID,
		UserID:    newUser.ID,
		TenantID:  in.CallerTenantID,
		Status:    model.MembershipStatusActive,
		JoinedAt:  now,
		Metadata:  []byte("{}"), // membership.metadata is NOT NULL jsonb
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: &callerID,
	}

	if err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		// User INSERT requires app.current_tenant='__platform__' per the
		// user_write RLS policy (migration 000004). Tenant admins call this
		// path too (POST /admin/users), so temporarily elevate to platform
		// for the user INSERT, then restore the caller's tenant context so
		// membership/role writes pass their tenant-scoped RLS policies.
		prevTenant := in.CallerTenantID
		if err := s.tx.SetTenantContext(txCtx, constants.PlatformTenantSentinel, in.CallerUserID); err != nil {
			return fmt.Errorf("elevate context for user insert: %w", err)
		}
		if err := s.users.Save(txCtx, newUser); err != nil {
			return fmt.Errorf("save user: %w", err)
		}
		if err := s.tx.SetTenantContext(txCtx, prevTenant, in.CallerUserID); err != nil {
			return fmt.Errorf("restore tenant context after user insert: %w", err)
		}
		if err := s.memberships.Save(txCtx, membership); err != nil {
			return fmt.Errorf("save membership: %w", err)
		}
		if len(in.RoleIDs) > 0 {
			if err := s.memberships.AssignRoles(txCtx, membershipID, in.RoleIDs, in.CallerUserID); err != nil {
				return fmt.Errorf("assign roles: %w", err)
			}
		}
		if len(in.BranchIDs) > 0 {
			if err := s.memberships.AssignBranches(txCtx, membershipID, in.BranchIDs, in.CallerUserID); err != nil {
				return fmt.Errorf("assign branches: %w", err)
			}
		}
		return nil
	}); err != nil {
		return CreateUserOutput{}, err
	}

	// Reload membership to get roles/branches populated.
	reloaded, err := s.memberships.FindByID(ctx, membershipID)
	if err != nil {
		return CreateUserOutput{}, fmt.Errorf("reload membership after create: %w", err)
	}

	s.emitAuditCreateUser(newUser.ID, in, true, true)

	return CreateUserOutput{
		User:              toUserProfileFromMembership(newUser, reloaded),
		InitialPassword:   plainPassword,
		CreatedUser:       true,
		CreatedMembership: true,
	}, nil
}

// emitAuditCreateUser fires an audit event in a goroutine.
func (s *UserService) emitAuditCreateUser(userID string, in CreateUserInput, createdUser, createdMembership bool) {
	go func() {
		caller := in.CallerUserID
		tid := in.CallerTenantID
		_ = s.audit.Append(context.Background(), AuditEntry{
			TenantID:    &tid,
			ActorUserID: &caller,
			Action:      "user.created",
			ResourceType: "user",
			ResourceID:  userID,
			Meta: map[string]interface{}{
				"email_prefix":       helper.SHA256Prefix(in.Email, 8),
				"created_user":       createdUser,
				"created_membership": createdMembership,
			},
		})
	}()
}

// ListUsers returns a paginated list of users within the caller's tenant.
func (s *UserService) ListUsers(ctx context.Context, in ListUsersInput) (ListUsersOutput, error) {
	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = defaultListLimit
	}
	page := in.Page
	if page < 1 {
		page = 1
	}

	filter := UserFilter{
		RoleID:   in.RoleID,
		BranchID: in.BranchID,
		IsActive: in.IsActive,
		Page:     page,
		Limit:    limit,
	}

	users, total, err := s.users.FindByTenant(ctx, in.CallerTenantID, filter)
	if err != nil {
		return ListUsersOutput{}, fmt.Errorf("list users: %w", err)
	}

	profiles := make([]UserProfile, len(users))
	for i, u := range users {
		profiles[i] = toUserProfile(u)
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	return ListUsersOutput{Users: profiles, Page: page, TotalCount: total, TotalPages: totalPages}, nil
}

// GetUser fetches a single user who has an active membership in the caller's tenant.
func (s *UserService) GetUser(ctx context.Context, in GetUserInput) (UserProfile, error) {
	user, err := s.users.FindByID(ctx, in.UserID)
	if err != nil {
		return UserProfile{}, err
	}

	// Enforce tenant scope via membership.
	_, err = s.memberships.FindByUserAndTenant(ctx, in.UserID, in.CallerTenantID)
	if err != nil {
		return UserProfile{}, constants.ErrUserNotFound
	}

	return toUserProfile(user), nil
}

// UpdateUser handles profile, activation, role, and branch updates for a user
// who has a membership in the caller's tenant.
func (s *UserService) UpdateUser(ctx context.Context, in UpdateUserInput) (UserProfile, error) {
	user, err := s.users.FindByID(ctx, in.TargetUserID)
	if err != nil {
		return UserProfile{}, err
	}

	membership, err := s.memberships.FindByUserAndTenant(ctx, in.TargetUserID, in.CallerTenantID)
	if err != nil {
		return UserProfile{}, constants.ErrUserNotFound
	}

	if in.FullName != nil {
		user.FullName = *in.FullName
	}
	if in.Phone != nil {
		user.Phone = in.Phone
	}
	if in.AvatarURL != nil {
		user.AvatarURL = in.AvatarURL
	}
	if in.IsActive != nil {
		user.IsActive = *in.IsActive
	}

	if err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.users.Update(txCtx, user); err != nil {
			return fmt.Errorf("update user profile: %w", err)
		}
		// nil slice = no change; non-nil (even empty) = replace assignments.
		if in.RoleIDs != nil {
			if err := s.memberships.AssignRoles(txCtx, membership.ID, in.RoleIDs, in.CallerUserID); err != nil {
				return fmt.Errorf("assign roles: %w", err)
			}
		}
		if in.BranchIDs != nil {
			if err := s.memberships.AssignBranches(txCtx, membership.ID, in.BranchIDs, in.CallerUserID); err != nil {
				return fmt.Errorf("assign branches: %w", err)
			}
		}
		return nil
	}); err != nil {
		return UserProfile{}, err
	}

	updated, err := s.users.FindByID(ctx, user.ID)
	if err != nil {
		return UserProfile{}, fmt.Errorf("reload updated user: %w", err)
	}

	go func() {
		caller := in.CallerUserID
		tid := in.CallerTenantID
		_ = s.audit.Append(context.Background(), AuditEntry{
			TenantID:     &tid,
			ActorUserID:  &caller,
			Action:       "user.updated",
			ResourceType: "user",
			ResourceID:   user.ID,
			Meta:         map[string]interface{}{},
		})
	}()

	return toUserProfile(updated), nil
}

// UnlockUser resets failed_login_count and clears locked_until for a user
// who has a membership in the caller's tenant.
func (s *UserService) UnlockUser(ctx context.Context, in UnlockUserInput) error {
	_, err := s.users.FindByID(ctx, in.TargetUserID)
	if err != nil {
		return err
	}

	// Enforce tenant scope via membership.
	if _, err = s.memberships.FindByUserAndTenant(ctx, in.TargetUserID, in.CallerTenantID); err != nil {
		return constants.ErrUserNotFound
	}

	if err := s.users.ResetFailedLogin(ctx, in.TargetUserID); err != nil {
		return fmt.Errorf("unlock user: %w", err)
	}

	go func() {
		caller := in.CallerUserID
		tid := in.CallerTenantID
		_ = s.audit.Append(context.Background(), AuditEntry{
			TenantID:     &tid,
			ActorUserID:  &caller,
			Action:       "user.unlocked",
			ResourceType: "user",
			ResourceID:   in.TargetUserID,
			Meta:         map[string]interface{}{},
		})
	}()

	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// toUserProfileFromMembership builds a UserProfile from a user + its active
// membership. This is used in CreateUser responses where a specific membership
// context is known. Roles and branches are not exposed on UserProfile in ADR
// 0007 (they belong to MembershipSummary); the profile here is the base
// identity projection.
func toUserProfileFromMembership(u *model.User, _ *model.Membership) UserProfile {
	return toUserProfile(u)
}
