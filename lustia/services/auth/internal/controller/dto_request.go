// Package controller contains the HTTP handlers (thin controllers) and
// request/response DTOs for the auth service. Controllers call services;
// they never call repositories directly.
package controller

import "encoding/json"

// LoginRequest is the JSON body for POST /auth/login.
// tenant_slug is removed in ADR 0007: login is email+password only.
//
// Identifier accepts either an email address or a username. It is the
// preferred field and takes precedence over Email. For backward compatibility
// with Phase 1–6 clients that still send only "email", the Email field is
// preserved but deprecated.
type LoginRequest struct {
	// Identifier accepts either an email address ("alice@example.com") or a
	// username ("alice"). Required unless Email is provided.
	Identifier string `json:"identifier" binding:"omitempty,min=3,max=320"`
	// Email is deprecated — use Identifier instead. Preserved for backward compat.
	Email    string `json:"email"    binding:"omitempty,email,max=320"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

// RefreshRequest is the JSON body for POST /auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest is the JSON body for POST /auth/logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"` // optional
}

// SelectTenantRequest is the JSON body for POST /auth/select-tenant.
type SelectTenantRequest struct {
	TenantID string `json:"tenant_id" binding:"required,uuid"`
}

// ForgotPasswordRequest is the JSON body for POST /auth/password/forgot.
// tenant_slug is removed in ADR 0007: lookup is global by email.
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email,max=320"`
}

// ResetPasswordRequest is the JSON body for POST /auth/password/reset.
type ResetPasswordRequest struct {
	Token       string `json:"token"        binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}

// UpdateMeRequest is the JSON body for PATCH /auth/me.
type UpdateMeRequest struct {
	FullName  *string `json:"full_name"  binding:"omitempty,min=1,max=200"`
	Phone     *string `json:"phone"      binding:"omitempty,max=30"`
	AvatarURL *string `json:"avatar_url" binding:"omitempty,url,max=2048"`
}

// ChangePasswordRequest is the JSON body for POST /auth/me/password.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=8,max=128"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}

// CreateUserRequest is the JSON body for POST /admin/users.
type CreateUserRequest struct {
	Email     string   `json:"email"      binding:"required,email,max=320"`
	Username  *string  `json:"username"   binding:"omitempty,username"`
	FullName  string   `json:"full_name"  binding:"required,min=1,max=200"`
	Phone     string   `json:"phone"      binding:"omitempty,max=30"`
	RoleIDs   []string `json:"role_ids"   binding:"omitempty,dive,uuid"`
	BranchIDs []string `json:"branch_ids" binding:"omitempty,dive,uuid"`
}

// ListUsersQuery are the query parameters for GET /admin/users.
type ListUsersQuery struct {
	RoleID   string `form:"role_id"   binding:"omitempty,uuid"`
	BranchID string `form:"branch_id" binding:"omitempty,uuid"`
	IsActive *bool  `form:"is_active"`
	Page     int    `form:"page"  binding:"omitempty,min=1"`
	Limit    int    `form:"limit" binding:"omitempty,min=1,max=200"`
}

// UpdateUserRequest is the JSON body for PATCH /admin/users/:id.
// Username uses a three-valued semantic:
//   - field absent from JSON body → UsernameSet=false, no change
//   - "username": null or ""      → UsernameSet=true, Username=nil → clears it
//   - "username": "alice"         → UsernameSet=true, Username=&"alice" → sets it
//
// The binding tag uses omitempty so that absent/empty values pass validation;
// format checking is handled by the service layer via ValidateUsername.
type UpdateUserRequest struct {
	FullName  *string  `json:"full_name"  binding:"omitempty,min=1,max=200"`
	Username  *string  `json:"username"   binding:"omitempty,username"`
	Phone     *string  `json:"phone"      binding:"omitempty,max=30"`
	AvatarURL *string  `json:"avatar_url" binding:"omitempty,url,max=2048"`
	IsActive  *bool    `json:"is_active"`
	RoleIDs   []string `json:"role_ids"   binding:"omitempty,dive,uuid"`
	BranchIDs []string `json:"branch_ids" binding:"omitempty,dive,uuid"`
}

// ---------------------------------------------------------------------------
// Phase 3 — Registration
// ---------------------------------------------------------------------------

// CompanyRegistrationRequest is the JSON body for POST /register/company.
// RequestedSlug is optional — when omitted, the service generates one from
// CompanyName and retries on collision. When supplied, it MUST match the
// slugify() output format ([a-z0-9] with single-hyphen separators, no leading
// or trailing hyphen) per SECURITY.md Phase 3 M-3.
type CompanyRegistrationRequest struct {
	CompanyName   string `json:"company_name"    binding:"required,min=2,max=200"`
	RequestedSlug string `json:"requested_slug"  binding:"omitempty,min=2,max=100,slug"`
	Package       string `json:"package"         binding:"omitempty,oneof=starter growth enterprise"`
	ContactName   string `json:"contact_name"    binding:"required,min=1,max=200"`
	ContactEmail  string `json:"contact_email"   binding:"required,email,max=320"`
	ContactPhone  string `json:"contact_phone"   binding:"omitempty,min=5,max=30"`
}

// ApproveTenantRegistrationRequest is the JSON body for POST /admin/tenant-registrations/:id/approve.
// All fields are optional overrides.
type ApproveTenantRegistrationRequest struct {
	Package     *string `json:"package"      binding:"omitempty,oneof=starter growth enterprise"`
	MaxBranches *int    `json:"max_branches" binding:"omitempty,min=1"`
}

// RejectTenantRegistrationRequest is the JSON body for POST /admin/tenant-registrations/:id/reject.
type RejectTenantRegistrationRequest struct {
	Reason string `json:"reason" binding:"omitempty,max=1000"`
}

// ListRegistrationsQuery are query params for GET /admin/tenant-registrations.
type ListRegistrationsQuery struct {
	Status string `form:"status" binding:"omitempty,oneof=pending approved rejected all"`
	Page   int    `form:"page"   binding:"omitempty,min=1"`
	Limit  int    `form:"limit"  binding:"omitempty,min=1,max=200"`
}

// ---------------------------------------------------------------------------
// Phase 3 — Tenant management
// ---------------------------------------------------------------------------

// ListTenantsQuery are query params for GET /admin/tenants.
type ListTenantsQuery struct {
	Status  string `form:"status"  binding:"omitempty,oneof=active suspended deactivated pending_approval all"`
	Q       string `form:"q"       binding:"omitempty,min=2,max=200"`
	Package string `form:"package" binding:"omitempty,oneof=starter growth enterprise"`
	Page    int    `form:"page"    binding:"omitempty,min=1"`
	Limit   int    `form:"limit"   binding:"omitempty,min=1,max=200"`
}

// ChangeTenantStatusRequest is the JSON body for PATCH /admin/tenants/:id/status.
// SECURITY.md Phase 3 M-2: DTO-layer allowlist matches valid tenant transitions.
type ChangeTenantStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active suspended deactivated"`
	Reason string `json:"reason" binding:"omitempty,max=1000"`
}

// ---------------------------------------------------------------------------
// Phase 3 — Branch management
// ---------------------------------------------------------------------------

// BranchRequest is the JSON body for POST /tenant/branches.
type BranchRequest struct {
	Name         string  `json:"name"          binding:"required,min=1,max=200"`
	Code         string  `json:"code"          binding:"required,min=1,max=50"`
	AddressLine1 *string `json:"address_line1" binding:"omitempty,max=500"`
	AddressLine2 *string `json:"address_line2" binding:"omitempty,max=500"`
	City         *string `json:"city"          binding:"omitempty,max=100"`
	Province     *string `json:"province"      binding:"omitempty,max=100"`
	PostalCode   *string `json:"postal_code"   binding:"omitempty,max=20"`
	Country      *string `json:"country"       binding:"omitempty,len=2"`
	Timezone     *string `json:"timezone"      binding:"omitempty,max=100"`
	ContactPhone *string `json:"contact_phone" binding:"omitempty,min=5,max=30"`
	ContactEmail *string `json:"contact_email" binding:"omitempty,email,max=320"`
}

// UpdateBranchRequest is the JSON body for PATCH /tenant/branches/:id.
// All fields are optional — only provided fields are updated.
type UpdateBranchRequest struct {
	Name         *string `json:"name"          binding:"omitempty,min=1,max=200"`
	AddressLine1 *string `json:"address_line1" binding:"omitempty,max=500"`
	AddressLine2 *string `json:"address_line2" binding:"omitempty,max=500"`
	City         *string `json:"city"          binding:"omitempty,max=100"`
	Province     *string `json:"province"      binding:"omitempty,max=100"`
	PostalCode   *string `json:"postal_code"   binding:"omitempty,max=20"`
	Country      *string `json:"country"       binding:"omitempty,len=2"`
	Timezone     *string `json:"timezone"      binding:"omitempty,max=100"`
	ContactPhone *string `json:"contact_phone" binding:"omitempty,min=5,max=30"`
	ContactEmail *string `json:"contact_email" binding:"omitempty,email,max=320"`
	// OperationalHours: raw JSON object e.g. {"mon":"09:00-17:00","sun":null}.
	// Stored verbatim as JSONB; nil = no change.
	OperationalHours json.RawMessage `json:"operational_hours" binding:"omitempty"`
}

// ChangeBranchStatusRequest is the JSON body for PATCH /tenant/branches/:id/status.
type ChangeBranchStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active inactive"`
}

// ListBranchesQuery are query params for GET /tenant/branches.
type ListBranchesQuery struct {
	Status string `form:"status" binding:"omitempty,oneof=active inactive all"`
	Page   int    `form:"page"   binding:"omitempty,min=1"`
	Limit  int    `form:"limit"  binding:"omitempty,min=1,max=200"`
}

// ---------------------------------------------------------------------------
// Phase 4 — Master Operational Data (ADR 0009).
// ---------------------------------------------------------------------------

// CreateTherapistRequest is the JSON body for POST /tenant/therapists.
// photo_key is intentionally absent — set only via POST …/therapists/:id/photo
// (ADR 0011 §2.3.2).
type CreateTherapistRequest struct {
	BranchID  string  `json:"branch_id"  binding:"required,uuid"`
	FullName  string  `json:"full_name"  binding:"required,min=1,max=200"`
	Gender    *string `json:"gender"     binding:"omitempty,oneof=male female other"`
	Phone     *string `json:"phone"      binding:"omitempty,max=30"`
	Email     *string `json:"email"      binding:"omitempty,email,max=320"`
	Bio       *string `json:"bio"        binding:"omitempty,max=500"`
	HeightCm  int16   `json:"height_cm"  binding:"required,min=100,max=250"`
	WeightKg  int16   `json:"weight_kg"  binding:"required,min=30,max=250"`
	Build     string  `json:"build"      binding:"required,oneof=langsing sedang atletis tegap"`
	JoinedAt  *string `json:"joined_at"  binding:"omitempty"`
	UserID    *string `json:"user_id"    binding:"omitempty,uuid"`
}

// UpdateTherapistRequest is the JSON body for PATCH /tenant/therapists/:id.
// All fields are optional — only provided fields are updated.
// photo_key is intentionally absent — use POST …/therapists/:id/photo.
type UpdateTherapistRequest struct {
	FullName    *string `json:"full_name"    binding:"omitempty,min=1,max=200"`
	Gender      *string `json:"gender"       binding:"omitempty,oneof=male female other"`
	Phone       *string `json:"phone"        binding:"omitempty,max=30"`
	Email       *string `json:"email"        binding:"omitempty,email,max=320"`
	Bio         *string `json:"bio"          binding:"omitempty,max=500"`
	HeightCm    *int16  `json:"height_cm"    binding:"omitempty,min=100,max=250"`
	WeightKg    *int16  `json:"weight_kg"    binding:"omitempty,min=30,max=250"`
	Build       *string `json:"build"        binding:"omitempty,oneof=langsing sedang atletis tegap"`
	JoinedAt    *string `json:"joined_at"    binding:"omitempty"`
	UserID      *string `json:"user_id"      binding:"omitempty,uuid"`
	PrepMinutes *int    `json:"prep_minutes" binding:"omitempty,min=0,max=60"`
}

// ChangeTherapistStatusRequest is the JSON body for PATCH /tenant/therapists/:id/status.
type ChangeTherapistStatusRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

// ListTherapistsQuery are query params for GET /tenant/therapists.
type ListTherapistsQuery struct {
	BranchID string `form:"branch_id" binding:"omitempty,uuid"`
	IsActive *bool  `form:"is_active"`
	Page     int    `form:"page"  binding:"omitempty,min=1"`
	Limit    int    `form:"limit" binding:"omitempty,min=1,max=200"`
}

// CreateServiceRequest is the JSON body for POST /tenant/services.
type CreateServiceRequest struct {
	Name            string  `json:"name"             binding:"required,min=1,max=200"`
	Description     *string `json:"description"      binding:"omitempty,max=1000"`
	Category        *string `json:"category"         binding:"omitempty,max=100"`
	DurationMinutes int     `json:"duration_minutes" binding:"required,min=1,max=1440"`
	PriceIDR        int64   `json:"price_idr"        binding:"min=0"`
}

// UpdateServiceRequest is the JSON body for PATCH /tenant/services/:id.
// All fields are optional — only provided fields are updated.
type UpdateServiceRequest struct {
	Name            *string `json:"name"             binding:"omitempty,min=1,max=200"`
	Description     *string `json:"description"      binding:"omitempty,max=1000"`
	Category        *string `json:"category"         binding:"omitempty,max=100"`
	DurationMinutes *int    `json:"duration_minutes" binding:"omitempty,min=1,max=1440"`
	PriceIDR        *int64  `json:"price_idr"        binding:"omitempty,min=0"`
}

// ChangeServiceStatusRequest is the JSON body for PATCH /tenant/services/:id/status.
type ChangeServiceStatusRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

// ListServicesQuery are query params for GET /tenant/services.
type ListServicesQuery struct {
	IsActive *bool   `form:"is_active"`
	Category *string `form:"category"`
	Page     int     `form:"page"  binding:"omitempty,min=1"`
	Limit    int     `form:"limit" binding:"omitempty,min=1,max=200"`
}

// PutTherapistServicesRequest is the JSON body for PUT /tenant/therapists/:id/services.
type PutTherapistServicesRequest struct {
	ServiceIDs []string `json:"service_ids" binding:"required,max=200,dive,uuid"`
}

// AvailabilityWindowRequest is a single window in the PUT availability payload.
type AvailabilityWindowRequest struct {
	DOW   int    `json:"dow"   binding:"min=0,max=6"`
	Start string `json:"start" binding:"required"`
	End   string `json:"end"   binding:"required"`
}

// PutAvailabilityRequest is the JSON body for PUT /tenant/therapists/:id/availability.
type PutAvailabilityRequest struct {
	Windows []AvailabilityWindowRequest `json:"windows" binding:"required"`
}

// ---------------------------------------------------------------------------
// ADR 0010 — Tenant-wide add-on catalog (rewritten 2026-04-24).
// ---------------------------------------------------------------------------

// CreateAddonRequest is the JSON body for POST /tenant/addons.
type CreateAddonRequest struct {
	Name        string  `json:"name"        binding:"required,min=1,max=120"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	PriceIDR    int64   `json:"price_idr"   binding:"min=0"`
	SortOrder   int     `json:"sort_order"  binding:"min=0,max=9999"`
}

// UpdateAddonRequest is the JSON body for PATCH /tenant/addons/:id.
// All fields are optional — only provided fields are updated.
type UpdateAddonRequest struct {
	Name        *string `json:"name"        binding:"omitempty,min=1,max=120"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	PriceIDR    *int64  `json:"price_idr"   binding:"omitempty,min=0"`
	SortOrder   *int    `json:"sort_order"  binding:"omitempty,min=0,max=9999"`
}

// ChangeAddonStatusRequest is the JSON body for
// PATCH /tenant/addons/:id/status.
type ChangeAddonStatusRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

// ListAddonsQuery are query parameters for GET /tenant/addons.
type ListAddonsQuery struct {
	IsActive *bool `form:"is_active"`
	Page     int   `form:"page"  binding:"omitempty,min=1"`
	Limit    int   `form:"limit" binding:"omitempty,min=1,max=200"`
}

// AddonSortOrderItemRequest is a single (id, sort_order) pair in the reorder
// request body.
type AddonSortOrderItemRequest struct {
	ID        string `json:"id"         binding:"required,uuid"`
	SortOrder int    `json:"sort_order" binding:"min=0,max=9999"`
}

// ReorderAddonsRequest is the JSON body for PUT /tenant/addons/reorder.
type ReorderAddonsRequest struct {
	Items []AddonSortOrderItemRequest `json:"items" binding:"required,min=1,max=200,dive"`
}

// ---------------------------------------------------------------------------
// ADR 0012 — Room (Ruangan) catalog.
// ---------------------------------------------------------------------------

// CreateRoomRequest is the JSON body for POST /tenant/rooms.
// photo_key is intentionally absent — set only via POST …/rooms/:id/photo
// (ADR 0012 §2.5, mirrors ADR 0011 §2.3.2).
type CreateRoomRequest struct {
	BranchID    string   `json:"branch_id"   binding:"required,uuid"`
	Name        string   `json:"name"        binding:"required,min=1,max=120"`
	Description *string  `json:"description" binding:"omitempty,max=500"`
	RoomType    string   `json:"room_type"   binding:"required,oneof=single couple group vip"`
	Capacity    int16    `json:"capacity"    binding:"required,min=1,max=20"`
	Amenities   []string `json:"amenities"   binding:"omitempty,max=20,dive,max=80"`
	SortOrder   int      `json:"sort_order"  binding:"min=0,max=9999"`
}

// UpdateRoomRequest is the JSON body for PATCH /tenant/rooms/:id.
// All fields are optional — only provided fields are updated.
// branch_id is intentionally included so the service can detect and reject
// an attempt to change it (immutability check in RoomSvc.Update).
type UpdateRoomRequest struct {
	BranchID    *string  `json:"branch_id"   binding:"omitempty,uuid"`
	Name        *string  `json:"name"        binding:"omitempty,min=1,max=120"`
	Description *string  `json:"description" binding:"omitempty,max=500"`
	RoomType    *string  `json:"room_type"   binding:"omitempty,oneof=single couple group vip"`
	Capacity    *int16   `json:"capacity"    binding:"omitempty,min=1,max=20"`
	Amenities   []string `json:"amenities"   binding:"omitempty,max=20,dive,max=80"`
	SortOrder   *int     `json:"sort_order"  binding:"omitempty,min=0,max=9999"`
}

// ChangeRoomStatusRequest is the JSON body for PATCH /tenant/rooms/:id/status.
type ChangeRoomStatusRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

// ListRoomsQuery are query parameters for GET /tenant/rooms.
type ListRoomsQuery struct {
	BranchID *string `form:"branch_id" binding:"omitempty,uuid"`
	IsActive *bool   `form:"is_active"`
	RoomType *string `form:"room_type" binding:"omitempty,oneof=single couple group vip"`
	Page     int     `form:"page"  binding:"omitempty,min=1"`
	Limit    int     `form:"limit" binding:"omitempty,min=1,max=200"`
}

// RoomSortOrderItemRequest is a single (id, sort_order) pair in the reorder
// request body.
type RoomSortOrderItemRequest struct {
	ID        string `json:"id"         binding:"required,uuid"`
	SortOrder int    `json:"sort_order" binding:"min=0,max=9999"`
}

// ReorderRoomsRequest is the JSON body for PUT /tenant/rooms/reorder.
// BranchID scopes the reorder operation — all items must belong to this branch.
type ReorderRoomsRequest struct {
	BranchID string                     `json:"branch_id" binding:"required,uuid"`
	Items    []RoomSortOrderItemRequest `json:"items"     binding:"required,min=1,max=200,dive"`
}

// ---------------------------------------------------------------------------
// ADR 0014 — Phase 5 Booking Engine.
// ---------------------------------------------------------------------------

// CreateBookingRequest is the JSON body for POST /api/v1/public/bookings.
//
// C-1 (SECURITY.md): total_price_idr is deliberately ABSENT from this DTO.
// The server computes the total from DB-fetched service + addon prices. Any
// client-supplied total would be ignored — but having no field makes it
// impossible to accidentally trust a client value.
//
// Also absent: status, paid_at, payment_reference, tenant_id — all server-set.
type CreateBookingRequest struct {
	BranchID       string   `json:"branch_id"       binding:"required,uuid"`
	ServiceID      string   `json:"service_id"      binding:"required,uuid"`
	AddonIDs       []string `json:"addon_ids"       binding:"omitempty,max=20,dive,uuid"`
	RoomID         *string  `json:"room_id"         binding:"omitempty,uuid"`
	TherapistID    *string  `json:"therapist_id"    binding:"omitempty,uuid"`
	ScheduledStart string   `json:"scheduled_start" binding:"required"`
	CustomerName   string   `json:"customer_name"   binding:"required,min=1,max=200"`
	CustomerPhone  string   `json:"customer_phone"  binding:"required,min=5,max=30"`
	CustomerEmail  string   `json:"customer_email"  binding:"required,email,max=320"`
	// Migration 000037: payment channel selected at checkout. Empty = qris (default).
	PaymentChannel string `json:"payment_channel" binding:"omitempty,oneof=qris va_bca va_mandiri va_bni va_bri va_permata va_cimb"`
}

// ConciergeCreateBookingRequest is the JSON body for POST /api/v1/tenant/bookings.
// Payment method is always paid_at_venue for operator-created bookings.
//
// C-1: total_price_idr is also absent here for the same reason.
type ConciergeCreateBookingRequest struct {
	BranchID       string   `json:"branch_id"       binding:"required,uuid"`
	ServiceID      string   `json:"service_id"      binding:"required,uuid"`
	AddonIDs       []string `json:"addon_ids"       binding:"omitempty,max=20,dive,uuid"`
	RoomID         *string  `json:"room_id"         binding:"omitempty,uuid"`
	TherapistID    *string  `json:"therapist_id"    binding:"omitempty,uuid"`
	ScheduledStart string   `json:"scheduled_start" binding:"required"`
	CustomerName   string   `json:"customer_name"   binding:"required,min=1,max=200"`
	CustomerPhone  string   `json:"customer_phone"  binding:"required,min=5,max=30"`
	CustomerEmail  string   `json:"customer_email"  binding:"required,email,max=320"`
}

// CheckInRequest is the JSON body for POST /api/v1/tenant/bookings/:id/checkin.
type CheckInRequest struct {
	Code string `json:"code" binding:"omitempty,max=9"`
}

// CancelBookingRequest is the JSON body for POST /api/v1/tenant/bookings/:id/cancel.
type CancelBookingRequest struct {
	Reason string `json:"reason" binding:"required,min=1,max=1000"`
}

// ListBookingsQuery are query parameters for GET /api/v1/tenant/bookings.
type ListBookingsQuery struct {
	BranchID  string `form:"branch_id"  binding:"omitempty,uuid"`
	Status    string `form:"status"     binding:"omitempty"`
	ServiceID string `form:"service_id" binding:"omitempty,uuid"`
	FromDate  string `form:"from"       binding:"omitempty"`
	ToDate    string `form:"to"         binding:"omitempty"`
	Page      int    `form:"page"       binding:"omitempty,min=1"`
	Limit     int    `form:"limit"      binding:"omitempty,min=1,max=200"`
}

// ReportBookingQuery are query parameters for GET /api/v1/tenant/reports/bookings/summary.
type ReportBookingQuery struct {
	BranchID string `form:"branch_id" binding:"omitempty,uuid"`
	From     string `form:"from"      binding:"required"`
	To       string `form:"to"        binding:"required"`
}

// AvailabilityQuery are query parameters for GET /api/v1/public/branches/:id/availability.
// TherapistID is optional — when present, every slot gains a boolean
// `therapist_available` that indicates whether THAT therapist is free at the
// slot (no overlapping booking + works that day-of-week per their schedule).
// Aggregate counts (therapists_available_count, rooms_available_count) are
// unaffected by this filter and always reflect branch-wide totals.
type AvailabilityQuery struct {
	ServiceID   string  `form:"service_id"   binding:"required,uuid"`
	Date        string  `form:"date"         binding:"required"`
	TherapistID *string `form:"therapist_id" binding:"omitempty,uuid"`
}

// PublicBranchListQuery are query parameters for GET /api/v1/public/branches.
type PublicBranchListQuery struct {
	Q        string   `form:"q"`
	Lat      *float64 `form:"lat"       binding:"omitempty,min=-90,max=90"`
	Lng      *float64 `form:"lng"       binding:"omitempty,min=-180,max=180"`
	Category string   `form:"category"`
	OpenNow  bool     `form:"open_now"`
	Page     int      `form:"page"      binding:"omitempty,min=1"`
	Limit    int      `form:"limit"     binding:"omitempty,min=1,max=50"`
}

// WebhookNotificationRequest is the JSON body for POST /api/v1/public/payments/webhook.
// This mirrors the Midtrans notification shape at the HTTP layer.
type WebhookNotificationRequest struct {
	OrderID           string `json:"order_id"`
	TransactionStatus string `json:"transaction_status"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	PaymentType       string `json:"payment_type"`
}
