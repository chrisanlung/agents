package service

import "time"

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

// LoginInput carries the credentials supplied by the caller.
// TenantSlug is removed in ADR 0007: login is now email+password only.
type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IP        string
}

// MembershipSummary is a read-only projection of a membership with its tenant,
// roles, and branches. Used in login responses and /auth/me.
type MembershipSummary struct {
	MembershipID string
	TenantID     string
	TenantName   string
	TenantSlug   string
	Roles        []string
	Branches     []string
	Status       string
}

// LoginOutput carries the tokens returned after a successful login.
// Shape varies by Scope:
//   - scope=platform : super-admin, ActiveMembershipID=nil, Memberships=[]
//   - scope=tenant   : single active membership auto-selected, ActiveMembershipID set
//   - scope=user     : multiple memberships, no tenant selected yet, ActiveMembershipID=nil
type LoginOutput struct {
	AccessToken        string
	RefreshToken       string // opaque bearer; SHA-256 hash is stored in DB
	ExpiresAt          time.Time
	Scope              string
	User               UserProfile
	Memberships        []MembershipSummary
	ActiveMembershipID *string // nil for scope=platform and scope=user
}

// ---------------------------------------------------------------------------
// Refresh
// ---------------------------------------------------------------------------

// RefreshInput carries the opaque refresh token provided by the caller.
type RefreshInput struct {
	RefreshToken string
	UserAgent    string
	IP           string
}

// RefreshOutput carries the new token pair after successful rotation.
// Scope and ActiveMembershipID mirror the scope that was in effect when the
// refresh token was originally issued.
type RefreshOutput struct {
	AccessToken        string
	RefreshToken       string
	ExpiresAt          time.Time
	Scope              string
	ActiveMembershipID *string
}

// ---------------------------------------------------------------------------
// Logout
// ---------------------------------------------------------------------------

// LogoutInput carries the refresh token to revoke.
type LogoutInput struct {
	CallerUserID string
	RefreshToken string // opaque; may be empty (no-op)
}

// ---------------------------------------------------------------------------
// SwitchTenant
// ---------------------------------------------------------------------------

// SwitchTenantInput carries the data for POST /auth/select-tenant.
type SwitchTenantInput struct {
	CallerUserID string
	TenantID     string
	// OldRefreshToken is the refresh token to revoke during rotation.
	OldRefreshToken string
	UserAgent       string
	IP              string
}

// SwitchTenantOutput carries the fresh tokens issued after a tenant switch.
type SwitchTenantOutput struct {
	AccessToken        string
	RefreshToken       string
	ExpiresAt          time.Time
	Scope              string // always "tenant"
	ActiveMembershipID string
	Membership         MembershipSummary
}

// ---------------------------------------------------------------------------
// Me
// ---------------------------------------------------------------------------

// UserProfile is a read-only projection used in GetMe and LoginOutput.
// It carries only identity fields; roles and branches live on MembershipSummary.
type UserProfile struct {
	ID                 string
	Email              string
	FullName           string
	Phone              string
	AvatarURL          string
	IsActive           bool
	IsSuperAdmin       bool
	MustChangePassword bool
}

// GetMeOutput wraps the UserProfile with membership info.
// ActiveMembershipID is set when the caller's token has scope=tenant.
// Tenant is non-nil only when scope=tenant.
type GetMeOutput struct {
	User               UserProfile
	ActiveMembershipID *string
	Tenant             *TenantInfo // nil for scope=platform or scope=user
	Memberships        []MembershipSummary
}

// TenantInfo is a read-only projection of the tenant entity.
type TenantInfo struct {
	ID     string
	Name   string
	Slug   string
	Status string
}

// ---------------------------------------------------------------------------
// UpdateMe
// ---------------------------------------------------------------------------

// UpdateMeInput carries the fields the user may update on their own profile.
type UpdateMeInput struct {
	CallerUserID string
	FullName     *string
	Phone        *string
	AvatarURL    *string
}

// ---------------------------------------------------------------------------
// ChangePassword
// ---------------------------------------------------------------------------

// ChangePasswordInput carries old and new passwords for self-service change.
type ChangePasswordInput struct {
	CallerUserID string
	OldPassword  string
	NewPassword  string
}

// ---------------------------------------------------------------------------
// ForgotPassword / ResetPassword
// ---------------------------------------------------------------------------

// ForgotPasswordInput is the data needed to initiate a password reset.
// TenantSlug is removed in ADR 0007: lookup is now global by email.
// The response is always 204 — the service never signals whether the email
// was found (anti-enumeration).
type ForgotPasswordInput struct {
	Email string
	IP    string
}

// ResetPasswordInput consumes a reset token and sets a new password.
type ResetPasswordInput struct {
	Token       string // opaque bearer
	NewPassword string
}

// ---------------------------------------------------------------------------
// Admin — Users
// ---------------------------------------------------------------------------

// CreateUserInput carries the data for admin-initiated user creation.
type CreateUserInput struct {
	CallerUserID   string
	CallerTenantID string
	Email          string
	FullName       string
	Phone          string
	RoleIDs        []string
	BranchIDs      []string
	InitialPassword string // generated server-side; returned once in response
}

// CreateUserOutput carries the created/invited user and the generated password
// (if a new user row was created). CreatedUser and CreatedMembership signal
// which rows were actually inserted so the caller can return them accurately.
type CreateUserOutput struct {
	User              UserProfile
	InitialPassword   string // returned once; never stored in plaintext
	CreatedUser       bool   // true when a new "user" row was inserted
	CreatedMembership bool   // true when a new "membership" row was inserted
}

// ListUsersInput carries filters and pagination for user listing.
type ListUsersInput struct {
	CallerTenantID string
	RoleID         *string
	BranchID       *string
	IsActive       *bool
	Cursor         string
	Limit          int
}

// ListUsersOutput carries the page of users and the next cursor.
type ListUsersOutput struct {
	Users      []UserProfile
	NextCursor string
}

// GetUserInput identifies a single user to retrieve.
type GetUserInput struct {
	CallerTenantID string
	UserID         string
}

// UpdateUserInput carries fields for admin-initiated user update.
type UpdateUserInput struct {
	CallerUserID   string
	CallerTenantID string
	TargetUserID   string
	FullName       *string
	Phone          *string
	AvatarURL      *string
	IsActive       *bool
	RoleIDs        []string // nil = no change; empty = clear all
	BranchIDs      []string // nil = no change; empty = clear all
}

// UnlockUserInput identifies a user whose lockout should be cleared.
type UnlockUserInput struct {
	CallerUserID   string
	CallerTenantID string
	TargetUserID   string
}

// ---------------------------------------------------------------------------
// Admin — Roles
// ---------------------------------------------------------------------------

// ListRolesOutput carries all roles with their permissions.
type ListRolesOutput struct {
	Roles []RoleDetail
}

// ---------------------------------------------------------------------------
// Phase 3 — Registration
// ---------------------------------------------------------------------------

// RegistrationInput carries the public company-registration submission.
type RegistrationInput struct {
	CompanyName   string
	RequestedSlug string
	Package       string
	ContactName   string
	ContactEmail  string
	ContactPhone  string
	IP            string // used for rate limiting
}

// RegistrationOutput carries the result of a successful registration submission.
type RegistrationOutput struct {
	RegistrationID string
	Status         string
}

// ApproveRegistrationInput carries the admin's approval decision.
type ApproveRegistrationInput struct {
	RegistrationID string
	CallerUserID   string
	// Optional overrides — when nil the defaults from the package matrix apply.
	Package     *string
	MaxBranches *int
}

// ApproveRegistrationOutput carries the objects created during approval.
type ApproveRegistrationOutput struct {
	Tenant            TenantDetail
	TenantAdmin       TenantAdminDetail
	Registration      RegistrationDetail
}

// TenantDetail is a read-only projection of a Tenant used in approval responses.
type TenantDetail struct {
	ID           string
	Name         string
	Slug         string
	Status       string
	Package      string
	MaxBranches  int
	ContactEmail string
	ContactName  string
	ApprovedAt   *string // RFC3339 or nil
	ApprovedBy   *string
	CreatedAt    string
}

// TenantAdminDetail carries the newly-created tenant admin user info.
// TemporaryPassword is returned once in the approval response and also emailed.
type TenantAdminDetail struct {
	UserID            string
	Email             string
	TemporaryPassword string // plaintext; returned once
}

// RegistrationDetail is the read-only projection of a TenantRegistration row.
type RegistrationDetail struct {
	ID              string
	CompanyName     string
	RequestedSlug   string
	Package         string
	ContactName     string
	ContactEmail    string
	ContactPhone    *string
	Status          string
	ApprovedAt      *string
	ApprovedBy      *string
	RejectedAt      *string
	RejectionReason *string
	CreatedAt       string
}

// RejectRegistrationInput carries the admin's rejection decision.
type RejectRegistrationInput struct {
	RegistrationID string
	CallerUserID   string
	Reason         string
}

// RejectRegistrationOutput carries the updated registration row.
type RejectRegistrationOutput struct {
	Registration RegistrationDetail
}

// ListRegistrationsInput carries filter + pagination for the registration queue.
type ListRegistrationsInput struct {
	Status string // pending|approved|rejected|all  (default pending)
	Cursor string
	Limit  int
}

// ListRegistrationsOutput carries a page of registrations.
type ListRegistrationsOutput struct {
	Registrations []RegistrationDetail
	NextCursor    string
}

// ---------------------------------------------------------------------------
// Phase 3 — Tenant management
// ---------------------------------------------------------------------------

// ListTenantsInput carries filter + pagination for the tenant list.
type ListTenantsInput struct {
	Status string // pending_approval|active|suspended|deactivated|all
	Cursor string
	Limit  int
}

// ListTenantsOutput carries a page of tenants with aggregate counts.
type ListTenantsOutput struct {
	Tenants    []TenantSummary
	NextCursor string
}

// TenantSummary is the read-only projection used in the admin tenant list.
type TenantSummary struct {
	ID              string
	Name            string
	Slug            string
	Status          string
	Package         string
	MaxBranches     int
	ContactEmail    string
	ContactName     string
	ApprovedAt      *string
	ApprovedBy      *string
	RejectedAt      *string
	RejectionReason *string
	CreatedAt       string
	MembershipCount int
	BranchCount     int
}

// TransitionTenantStatusInput carries the status-change request.
type TransitionTenantStatusInput struct {
	TenantID     string
	CallerUserID string
	NewStatus    string
	Reason       string // required for deactivation
}

// ---------------------------------------------------------------------------
// Phase 3 — Branches
// ---------------------------------------------------------------------------

// CreateBranchInput carries data for a new branch.
type CreateBranchInput struct {
	CallerUserID   string
	CallerTenantID string
	Name           string
	Code           string
	AddressLine1   *string
	AddressLine2   *string
	City           *string
	Province       *string
	PostalCode     *string
	Country        string
	Timezone       string
	ContactPhone   *string
	ContactEmail   *string
}

// UpdateBranchInput carries the mutable non-status fields for a branch update.
type UpdateBranchInput struct {
	BranchID       string
	CallerUserID   string
	CallerTenantID string
	Name           *string
	AddressLine1   *string
	AddressLine2   *string
	City           *string
	Province       *string
	PostalCode     *string
	Country        *string
	Timezone       *string
	ContactPhone   *string
	ContactEmail   *string
}

// ChangeBranchStatusInput carries the status-transition request.
type ChangeBranchStatusInput struct {
	BranchID       string
	CallerUserID   string
	CallerTenantID string
	NewStatus      string
}

// BranchDetail is the read-only projection of a Branch row.
type BranchDetail struct {
	ID           string
	TenantID     string
	Name         string
	Code         string
	Status       string
	AddressLine1 *string
	AddressLine2 *string
	City         *string
	Province     *string
	PostalCode   *string
	Country      string
	Timezone     string
	ContactPhone *string
	ContactEmail *string
	ActivatedAt  *string // RFC3339 or nil
	CreatedAt    string
	UpdatedAt    string
}

// ListBranchesInput carries filter + pagination for the branch list.
type ListBranchesInput struct {
	CallerTenantID string
	Status         string // active|inactive|all
	Cursor         string
	Limit          int
}

// ListBranchesOutput carries a page of branches.
type ListBranchesOutput struct {
	Branches   []BranchDetail
	NextCursor string
}

// OnboardingStateOutput carries the tenant admin's post-login onboarding state.
type OnboardingStateOutput struct {
	HasBranches        bool
	ActiveBranchCount  int
	MaxBranches        int
	MustChangePassword bool
}

// RoleDetail is a read-only projection of a role with its permissions.
type RoleDetail struct {
	ID          string
	Name        string
	Description string
	Permissions []PermissionDetail
}

// PermissionDetail is a read-only projection of a single permission.
type PermissionDetail struct {
	ID          string
	Code        string
	Description string
}

// ---------------------------------------------------------------------------
// Phase 4 — Master Operational Data (ADR 0009).
// ---------------------------------------------------------------------------

// TherapistDetail is the read-only projection of a Therapist row.
type TherapistDetail struct {
	ID          string
	TenantID    string
	BranchID    string
	UserID      *string
	FullName    string
	Gender      *string
	Phone       *string
	Email       *string
	Bio         *string
	PhotoURL    *string
	Specialties []string
	IsActive    bool
	JoinedAt    *string // RFC3339 or nil
	CreatedAt   string
	UpdatedAt   string
}

// TherapistDetailWithServices extends TherapistDetail with the service mapping
// list. Returned by GET /tenant/therapists/:id (§11.4.3).
type TherapistDetailWithServices struct {
	TherapistDetail
	Services []TherapistServiceItem
}

// TherapistServiceItem is a single entry in the services array on a therapist
// detail or mapping response.
type TherapistServiceItem struct {
	ServiceID       string
	Name            string
	Category        *string
	DurationMinutes int
	PriceIDR        int64
	IsActive        bool
	AssignedAt      string // RFC3339
}

// ServiceDetail is the read-only projection of a ServiceCatalog row.
type ServiceDetail struct {
	ID              string
	TenantID        string
	Name            string
	Description     *string
	Category        *string
	DurationMinutes int
	PriceIDR        int64
	Currency        string
	IsActive        bool
	CreatedAt       string
	UpdatedAt       string
}

// ServiceDetailWithTherapists extends ServiceDetail with active therapist
// mappings. Returned by GET /tenant/services/:id (§11.5.3, flag #6).
type ServiceDetailWithTherapists struct {
	ServiceDetail
	Therapists []ServiceTherapistItem
}

// ServiceTherapistItem is a single entry in the therapists array on a service
// detail response. Only active mappings are included (§11.3 flag #6).
type ServiceTherapistItem struct {
	TherapistID string
	FullName    string
	BranchID    string
	BranchName  string
	IsActive    bool
}

// AvailabilityWindow is a single weekly recurrence window.
type AvailabilityWindow struct {
	ID    string
	DOW   int    // 0=Sunday … 6=Saturday
	Start string // HH:MM
	End   string // HH:MM
}

// TherapistMappingOutput is the response for GET/PUT /therapists/:id/services.
type TherapistMappingOutput struct {
	TherapistID string
	Services    []TherapistServiceItem
}

// AvailabilityOutput is the response for GET/PUT /therapists/:id/availability.
type AvailabilityOutput struct {
	TherapistID string
	Windows     []AvailabilityWindow
}

// ---------- Input types ----------

// CreateTherapistInput carries data for creating a new therapist.
type CreateTherapistInput struct {
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string // from JWT branches claim
	IsAdmin        bool     // true when caller is tenant_admin (no cross-branch check)
	BranchID       string
	FullName       string
	Gender         *string
	Phone          *string
	Email          *string
	Bio            *string
	PhotoURL       *string
	JoinedAt       *string // YYYY-MM-DD date string
	UserID         *string
}

// UpdateTherapistInput carries partial-update fields for a therapist.
type UpdateTherapistInput struct {
	TherapistID    string
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string
	IsAdmin        bool
	FullName       *string
	Gender         *string
	Phone          *string
	Email          *string
	Bio            *string
	PhotoURL       *string
	JoinedAt       *string
	UserID         *string
}

// ChangeTherapistStatusInput carries the activate/deactivate request.
type ChangeTherapistStatusInput struct {
	TherapistID    string
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string
	IsAdmin        bool
	IsActive       bool
}

// ListTherapistsInput carries filter + pagination for the therapist list.
type ListTherapistsInput struct {
	CallerTenantID string
	CallerBranches []string // restricts results for branch_admin
	IsAdmin        bool
	BranchID       *string
	IsActive       *bool // nil = active only
	Cursor         string
	Limit          int
}

// ListTherapistsOutput carries a page of therapists.
type ListTherapistsOutput struct {
	Therapists []TherapistDetail
	NextCursor string
}

// CreateServiceInput carries data for creating a new service.
type CreateServiceInput struct {
	CallerUserID    string
	CallerTenantID  string
	Name            string
	Description     *string
	Category        *string
	DurationMinutes int
	PriceIDR        int64
}

// UpdateServiceInput carries partial-update fields for a service.
type UpdateServiceInput struct {
	ServiceID       string
	CallerUserID    string
	CallerTenantID  string
	Name            *string
	Description     *string
	Category        *string
	DurationMinutes *int
	PriceIDR        *int64
}

// ChangeServiceStatusInput carries the activate/deactivate request for a service.
type ChangeServiceStatusInput struct {
	ServiceID      string
	CallerUserID   string
	CallerTenantID string
	IsActive       bool
}

// ListServicesInput carries filter + pagination for the service list.
type ListServicesInput struct {
	CallerTenantID string
	IsActive       *bool   // nil = active only
	Category       *string // case-sensitive filter
	Cursor         string
	Limit          int
}

// ListServicesOutput carries a page of services.
type ListServicesOutput struct {
	Services   []ServiceDetail
	NextCursor string
}

// ReconcileMappingInput carries the full-replace request for therapist services.
type ReconcileMappingInput struct {
	TherapistID    string
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string
	IsAdmin        bool
	ServiceIDs     []string
}

// ReplaceAvailabilityWindow is a single window in the PUT availability payload.
type ReplaceAvailabilityWindow struct {
	DOW   int    // 0–6
	Start string // HH:MM
	End   string // HH:MM
}

// ReplaceAvailabilityInput carries the full-replace availability request.
type ReplaceAvailabilityInput struct {
	TherapistID    string
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string
	IsAdmin        bool
	Windows        []ReplaceAvailabilityWindow
}
