package harness

// ============================================================================
// Phase 4 — Master Operational Data: response types and request types
//
// These structs mirror the JSON shapes defined in API_CONTRACT.md §11.
// They are consumed by the Phase 4 test files (T9–T14) and by the typed
// helpers added to client.go.
// ============================================================================

// ----- Therapist ------------------------------------------------------------

// CreateTherapistRequest is the body for POST /api/v1/tenant/therapists.
type CreateTherapistRequest struct {
	BranchID   string  `json:"branch_id"`
	FullName   string  `json:"full_name"`
	Gender     string  `json:"gender,omitempty"`
	Phone      string  `json:"phone,omitempty"`
	Email      string  `json:"email,omitempty"`
	Bio        string  `json:"bio,omitempty"`
	PhotoURL   *string `json:"photo_url,omitempty"`
	JoinedAt   string  `json:"joined_at,omitempty"`
	UserID     *string `json:"user_id,omitempty"`
}

// UpdateTherapistRequest is the body for PATCH /api/v1/tenant/therapists/:id.
type UpdateTherapistRequest struct {
	FullName *string `json:"full_name,omitempty"`
	Gender   *string `json:"gender,omitempty"`
	Phone    *string `json:"phone,omitempty"`
	Email    *string `json:"email,omitempty"`
	Bio      *string `json:"bio,omitempty"`
	PhotoURL *string `json:"photo_url,omitempty"`
	JoinedAt *string `json:"joined_at,omitempty"`
}

// TherapistStatusRequest is the body for PATCH /api/v1/tenant/therapists/:id/status.
type TherapistStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// TherapistResponse is the shared response shape for a single therapist
// (API_CONTRACT.md §11.4). The Services field is only populated by the GET
// single-therapist endpoint (§11.4.3).
type TherapistResponse struct {
	ID         string                  `json:"id"`
	TenantID   string                  `json:"tenant_id"`
	BranchID   string                  `json:"branch_id"`
	UserID     *string                 `json:"user_id"`
	FullName   string                  `json:"full_name"`
	Gender     *string                 `json:"gender"`
	Bio        *string                 `json:"bio"`
	PhotoURL   *string                 `json:"photo_url"`
	Specialties []interface{}          `json:"specialties"`
	IsActive   bool                    `json:"is_active"`
	JoinedAt   *string                 `json:"joined_at"`
	CreatedAt  string                  `json:"created_at"`
	UpdatedAt  string                  `json:"updated_at"`
	Services   []TherapistServiceItem  `json:"services,omitempty"`
}

// TherapistServiceItem appears in the services array of GET /therapists/:id.
type TherapistServiceItem struct {
	ServiceID       string `json:"service_id"`
	Name            string `json:"name"`
	Category        string `json:"category"`
	DurationMinutes int    `json:"duration_minutes"`
	PriceIDR        int64  `json:"price_idr"`
	IsActive        bool   `json:"is_active"`
}

// TherapistListResponse is the paginated list response for GET /therapists.
type TherapistListResponse struct {
	Data       []TherapistResponse `json:"data"`
	NextCursor *string             `json:"next_cursor"`
}

// ----- Service --------------------------------------------------------------

// CreateServiceRequest is the body for POST /api/v1/tenant/services.
type CreateServiceRequest struct {
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Category        string `json:"category,omitempty"`
	DurationMinutes int    `json:"duration_minutes"`
	PriceIDR        int64  `json:"price_idr"`
}

// UpdateServiceRequest is the body for PATCH /api/v1/tenant/services/:id.
type UpdateServiceRequest struct {
	Name            *string `json:"name,omitempty"`
	Description     *string `json:"description,omitempty"`
	Category        *string `json:"category,omitempty"`
	DurationMinutes *int    `json:"duration_minutes,omitempty"`
	PriceIDR        *int64  `json:"price_idr,omitempty"`
}

// ServiceStatusRequest is the body for PATCH /api/v1/tenant/services/:id/status.
type ServiceStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// ServiceResponse is the shared response shape for a single service
// (API_CONTRACT.md §11.5). The Therapists field is only populated by
// GET /services/:id (§11.5.3).
type ServiceResponse struct {
	ID              string                `json:"id"`
	TenantID        string                `json:"tenant_id"`
	Name            string                `json:"name"`
	Description     *string               `json:"description"`
	Category        *string               `json:"category"`
	DurationMinutes int                   `json:"duration_minutes"`
	PriceIDR        int64                 `json:"price_idr"`
	Currency        string                `json:"currency"`
	IsActive        bool                  `json:"is_active"`
	CreatedAt       string                `json:"created_at"`
	UpdatedAt       string                `json:"updated_at"`
	Therapists      []ServiceTherapistItem `json:"therapists,omitempty"`
}

// ServiceTherapistItem appears in the therapists array of GET /services/:id.
type ServiceTherapistItem struct {
	TherapistID string `json:"therapist_id"`
	FullName    string `json:"full_name"`
	BranchID    string `json:"branch_id"`
	BranchName  string `json:"branch_name"`
	IsActive    bool   `json:"is_active"`
}

// ServiceListResponse is the paginated list response for GET /services.
type ServiceListResponse struct {
	Data       []ServiceResponse `json:"data"`
	NextCursor *string           `json:"next_cursor"`
}

// ----- Service Mapping ------------------------------------------------------

// PutServicesRequest is the body for PUT /api/v1/tenant/therapists/:id/services.
type PutServicesRequest struct {
	ServiceIDs []string `json:"service_ids"`
}

// ServiceMappingItem is one entry in the services array of the mapping response.
type ServiceMappingItem struct {
	ServiceID       string `json:"service_id"`
	Name            string `json:"name"`
	Category        string `json:"category"`
	DurationMinutes int    `json:"duration_minutes"`
	PriceIDR        int64  `json:"price_idr"`
	IsActive        bool   `json:"is_active"`
	AssignedAt      string `json:"assigned_at"`
}

// ServiceMappingResponse is returned by both GET and PUT
// /api/v1/tenant/therapists/:id/services.
type ServiceMappingResponse struct {
	TherapistID string               `json:"therapist_id"`
	Services    []ServiceMappingItem `json:"services"`
}

// ----- Availability ---------------------------------------------------------

// AvailabilityWindow is a single availability window (flat shape per flag #5).
type AvailabilityWindow struct {
	ID    string `json:"id,omitempty"` // present in GET responses
	DOW   int    `json:"dow"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// PutAvailabilityRequest is the body for PUT /api/v1/tenant/therapists/:id/availability.
type PutAvailabilityRequest struct {
	Windows []AvailabilityWindow `json:"windows"`
}

// AvailabilityResponse is returned by both GET and PUT
// /api/v1/tenant/therapists/:id/availability.
type AvailabilityResponse struct {
	TherapistID string               `json:"therapist_id"`
	Windows     []AvailabilityWindow `json:"windows"`
}
