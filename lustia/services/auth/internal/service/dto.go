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
	Page           int
	Limit          int
}

// ListUsersOutput carries a page of users and pagination metadata.
type ListUsersOutput struct {
	Users      []UserProfile
	Page       int
	TotalCount int64
	TotalPages int
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
	Page   int
	Limit  int
}

// ListRegistrationsOutput carries a page of registrations and pagination metadata.
type ListRegistrationsOutput struct {
	Registrations []RegistrationDetail
	Page          int
	TotalCount    int64
	TotalPages    int
}

// ---------------------------------------------------------------------------
// Phase 3 — Tenant management
// ---------------------------------------------------------------------------

// ListTenantsInput carries filter + pagination for the tenant list.
type ListTenantsInput struct {
	Status  string // pending_approval|active|suspended|deactivated|all
	Q       string // search query for name/slug (min 2 chars enforced at controller)
	Package string // starter|growth|enterprise; empty = all
	Page    int
	Limit   int
}

// ListTenantsOutput carries a page of tenants with aggregate counts and pagination metadata.
type ListTenantsOutput struct {
	Tenants    []TenantSummary
	Page       int
	TotalCount int64
	TotalPages int
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
	Page           int
	Limit          int
}

// ListBranchesOutput carries a page of branches and pagination metadata.
type ListBranchesOutput struct {
	Branches   []BranchDetail
	Page       int
	TotalCount int64
	TotalPages int
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
// PhotoKey is the opaque storage key (ADR 0011 §2.1); controllers resolve it
// to a URL via Storage.URL before building the HTTP response.
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
	PhotoKey    *string // opaque storage key; never exposed directly over the wire
	HeightCm    int16
	WeightKg    int16
	Build       string
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
// photo_key is NOT settable via JSON — only through the upload endpoint
// (ADR 0011 §2.3.2).
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
	HeightCm       int16
	WeightKg       int16
	Build          string
	JoinedAt       *string // YYYY-MM-DD date string
	UserID         *string
}

// UpdateTherapistInput carries partial-update fields for a therapist.
// photo_key is NOT settable here — only through the upload endpoint.
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
	HeightCm       *int16
	WeightKg       *int16
	Build          *string
	JoinedAt       *string
	UserID         *string
}

// UploadTherapistPhotoInput carries data for the photo-upload operation.
type UploadTherapistPhotoInput struct {
	TherapistID    string
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string
	IsAdmin        bool
	NewKey         string // generated by controller before calling service
}

// RemoveTherapistPhotoInput carries data for the photo-removal operation.
type RemoveTherapistPhotoInput struct {
	TherapistID    string
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string
	IsAdmin        bool
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
	Page           int
	Limit          int
}

// ListTherapistsOutput carries a page of therapists and pagination metadata.
type ListTherapistsOutput struct {
	Therapists []TherapistDetail
	Page       int
	TotalCount int64
	TotalPages int
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
	Page           int
	Limit          int
}

// ListServicesOutput carries a page of services and pagination metadata.
type ListServicesOutput struct {
	Services   []ServiceDetail
	Page       int
	TotalCount int64
	TotalPages int
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

// ---------------------------------------------------------------------------
// ADR 0010 — Tenant-wide add-on catalog (rewritten 2026-04-24).
// ---------------------------------------------------------------------------

// AddonDetail is the read-only projection of an addon row for controller
// consumption. `tenant_id` is deliberately absent: controllers must never emit
// it over the wire. Service-layer tenant-isolation checks use model.Addon
// directly (via the repository) before mapping to AddonDetail.
type AddonDetail struct {
	ID          string
	Name        string
	Description *string
	PriceIDR    int64
	IsActive    bool
	SortOrder   int
	CreatedAt   string // RFC3339
	UpdatedAt   string // RFC3339
}

// CreateAddonInput carries data for creating a new tenant-wide add-on.
type CreateAddonInput struct {
	CallerUserID   string
	CallerTenantID string
	Name           string
	Description    *string
	PriceIDR       int64
	SortOrder      int
}

// UpdateAddonInput carries partial-update fields for an add-on.
type UpdateAddonInput struct {
	AddonID        string
	CallerUserID   string
	CallerTenantID string
	Name           *string
	Description    *string
	PriceIDR       *int64
	SortOrder      *int
}

// ChangeAddonStatusInput carries the activate/deactivate request for an add-on.
type ChangeAddonStatusInput struct {
	AddonID        string
	CallerUserID   string
	CallerTenantID string
	IsActive       bool
}

// ListAddonsInput carries the list query parameters for tenant-wide add-ons.
type ListAddonsInput struct {
	CallerTenantID string
	IsActive       *bool // nil = all non-deleted
	Page           int
	Limit          int
}

// ListAddonsOutput carries an offset-paginated page of add-ons and pagination metadata.
type ListAddonsOutput struct {
	Addons     []AddonDetail
	Page       int
	TotalCount int64
	TotalPages int
}

// ReorderAddonsInput carries the bulk sort_order update request.
type ReorderAddonsInput struct {
	CallerUserID   string
	CallerTenantID string
	Items          []AddonSortOrderItem
}

// ---------------------------------------------------------------------------
// ADR 0012 — Room (Ruangan) catalog.
// ---------------------------------------------------------------------------

// RoomDetail is the read-only projection of a Room row for controller
// consumption. `tenant_id` is deliberately absent: controllers must never emit
// it over the wire (lesson from addon code-review). PhotoKey is the opaque
// storage key; controllers resolve it to a URL via Storage.URL before building
// the HTTP response.
type RoomDetail struct {
	ID          string
	BranchID    string
	Name        string
	Description *string
	RoomType    string
	Capacity    int16
	Amenities   []string
	PhotoKey    *string // opaque storage key; never exposed directly over the wire
	IsActive    bool
	SortOrder   int
	CreatedAt   string // RFC3339
	UpdatedAt   string // RFC3339
}

// CreateRoomInput carries data for creating a new room.
// photo_key is NOT settable via JSON — only through the upload endpoint
// (ADR 0012 §2.5, mirrors ADR 0011 §2.3.2).
type CreateRoomInput struct {
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string // from JWT branches claim
	IsAdmin        bool     // true when caller is tenant_admin or super_admin
	BranchID       string
	Name           string
	Description    *string
	RoomType       string
	Capacity       int16
	Amenities      []string
	SortOrder      int
}

// UpdateRoomInput carries partial-update fields for a room.
// photo_key is NOT settable here — only through the upload endpoint.
// BranchID is intentionally NOT a pointer: it must be absent from the JSON
// body; any supplied value is ignored after the immutability check.
type UpdateRoomInput struct {
	RoomID         string
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string
	IsAdmin        bool
	// BranchIDAttempt is set when the caller supplied branch_id in the request;
	// the service rejects with ErrRoomBranchImmutable if it differs from the
	// room's existing branch_id.
	BranchIDAttempt *string
	Name            *string
	Description     *string
	RoomType        *string
	Capacity        *int16
	Amenities       []string // nil = no change; empty slice = clear amenities
	AmenitiesSet    bool     // true when Amenities was explicitly provided
	SortOrder       *int
}

// ChangeRoomStatusInput carries the activate/deactivate request for a room.
type ChangeRoomStatusInput struct {
	RoomID         string
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string
	IsAdmin        bool
	IsActive       bool
}

// ListRoomsInput carries filter + pagination for the room list.
type ListRoomsInput struct {
	CallerTenantID string
	CallerBranches []string // restricts results for branch_admin
	IsAdmin        bool
	BranchID       *string
	IsActive       *bool   // nil = all non-deleted
	RoomType       *string // nil = all types
	Page           int
	Limit          int
}

// ListRoomsOutput carries an offset-paginated page of rooms and pagination metadata.
type ListRoomsOutput struct {
	Rooms      []RoomDetail
	Page       int
	TotalCount int64
	TotalPages int
}

// ReorderRoomsInput carries the bulk sort_order update request.
// All items must belong to a single branch (validated in the service layer).
type ReorderRoomsInput struct {
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string
	IsAdmin        bool
	BranchID       string // all items must belong to this branch
	Items          []RoomSortOrderItem
}

// UploadRoomPhotoInput carries data for the photo-upload operation.
type UploadRoomPhotoInput struct {
	RoomID         string
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string
	IsAdmin        bool
	NewKey         string // generated by controller before calling service
}

// RemoveRoomPhotoInput carries data for the photo-removal operation.
type RemoveRoomPhotoInput struct {
	RoomID         string
	CallerUserID   string
	CallerTenantID string
	CallerBranches []string
	IsAdmin        bool
}
