package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// BaseURL returns the auth-service base URL from the BASE_URL env var,
// defaulting to localhost:8080.
func BaseURL() string {
	u := os.Getenv("BASE_URL")
	if u == "" {
		u = "http://localhost:8080"
	}
	return u
}

// APIClient is a thin HTTP wrapper that always targets the live service.
type APIClient struct {
	base   string
	client *http.Client
	// ForwardedFor lets tests override the X-Forwarded-For header to simulate
	// different client IPs for rate-limit testing.
	ForwardedFor string
}

// NewClient returns an APIClient targeting BaseURL().
func NewClient() *APIClient {
	return &APIClient{
		base:   BaseURL(),
		client: &http.Client{},
	}
}

// ----- helper ----------------------------------------------------------------

func (c *APIClient) do(method, path string, body any, token string) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, reqBody)
	if err != nil {
		return nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if c.ForwardedFor != "" {
		req.Header.Set("X-Forwarded-For", c.ForwardedFor)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, raw, nil
}

// decodeJSON unmarshals raw bytes into v.
func decodeJSON(raw []byte, v any) error {
	return json.Unmarshal(raw, v)
}

// DecodeJSONPublic is the exported form of decodeJSON for test helpers in the
// same test binary that need to decode a response they obtained via RawDo.
func DecodeJSONPublic(raw []byte, v any) error {
	return decodeJSON(raw, v)
}

// ----- request / response types ----------------------------------------------

type RegisterCompanyRequest struct {
	CompanyName   string `json:"company_name"`
	RequestedSlug string `json:"requested_slug"`
	Package       string `json:"package,omitempty"`
	ContactName   string `json:"contact_name"`
	ContactEmail  string `json:"contact_email"`
	ContactPhone  string `json:"contact_phone,omitempty"`
}

type RegisterCompanyResponse struct {
	RegistrationID string `json:"registration_id"`
	Status         string `json:"status"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken       string          `json:"access_token"`
	RefreshToken      string          `json:"refresh_token"`
	TokenType         string          `json:"token_type"`
	ExpiresAt         string          `json:"expires_at"`
	Scope             string          `json:"scope"`
	ActiveMembershipID *string        `json:"active_membership_id"`
	User              UserResponse    `json:"user"`
	Memberships       []MembershipItem `json:"memberships"`
}

type UserResponse struct {
	ID                 string  `json:"id"`
	Email              string  `json:"email"`
	FullName           string  `json:"full_name"`
	Phone              *string `json:"phone"`
	AvatarURL          *string `json:"avatar_url"`
	IsActive           bool    `json:"is_active"`
	IsSuperAdmin       bool    `json:"is_super_admin"`
	MustChangePassword *bool   `json:"must_change_password"`
}

type MembershipItem struct {
	MembershipID string   `json:"membership_id"`
	TenantID     string   `json:"tenant_id"`
	TenantName   string   `json:"tenant_name"`
	TenantSlug   string   `json:"tenant_slug"`
	Roles        []string `json:"roles"`
	Branches     []string `json:"branches"`
	Status       string   `json:"status"`
}

type ApproveRegistrationRequest struct {
	Package     string `json:"package,omitempty"`
	MaxBranches *int   `json:"max_branches,omitempty"`
}

type ApproveRegistrationResponse struct {
	Tenant       TenantResponse      `json:"tenant"`
	TenantAdmin  TenantAdminResponse `json:"tenant_admin"`
	Registration RegistrationItem    `json:"registration"`
}

type TenantResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Status      string  `json:"status"`
	Package     string  `json:"package"`
	MaxBranches int     `json:"max_branches"`
	ContactEmail string `json:"contact_email"`
	ContactName  string `json:"contact_name"`
	ApprovedAt  *string `json:"approved_at"`
	ApprovedBy  *string `json:"approved_by"`
	CreatedAt   string  `json:"created_at"`
}

type TenantAdminResponse struct {
	UserID            string `json:"user_id"`
	Email             string `json:"email"`
	TemporaryPassword string `json:"temporary_password"`
}

type RegistrationItem struct {
	ID           string  `json:"id"`
	CompanyName  string  `json:"company_name"`
	RequestedSlug string `json:"requested_slug"`
	Package      string  `json:"package"`
	ContactName  string  `json:"contact_name"`
	ContactEmail string  `json:"contact_email"`
	Status       string  `json:"status"`
	ApprovedAt   *string `json:"approved_at"`
	ApprovedBy   *string `json:"approved_by"`
	CreatedAt    string  `json:"created_at"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type CreateBranchRequest struct {
	Name         string `json:"name"`
	Code         string `json:"code,omitempty"`
	AddressLine1 string `json:"address_line1,omitempty"`
	City         string `json:"city,omitempty"`
	Province     string `json:"province,omitempty"`
	PostalCode   string `json:"postal_code,omitempty"`
	Country      string `json:"country,omitempty"`
	Timezone     string `json:"timezone,omitempty"`
	ContactPhone string `json:"contact_phone,omitempty"`
	ContactEmail string `json:"contact_email,omitempty"`
}

type BranchResponse struct {
	ID           string  `json:"id"`
	TenantID     string  `json:"tenant_id"`
	Name         string  `json:"name"`
	Code         *string `json:"code"`
	Status       string  `json:"status"`
	City         *string `json:"city"`
	Timezone     string  `json:"timezone"`
	CreatedAt    string  `json:"created_at"`
	DeletedAt    *string `json:"deleted_at"`
}

type ChangeTenantStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// MeResponse is the shape returned by GET /api/v1/auth/me
type MeResponse struct {
	User               UserResponse     `json:"user"`
	Tenant             *TenantResponse  `json:"tenant"`
	ActiveMembershipID *string          `json:"active_membership_id"`
	Memberships        []MembershipItem `json:"memberships"`
}

type ListRegistrationsResponse struct {
	Data       []RegistrationItem `json:"data"`
	NextCursor *string            `json:"next_cursor"`
}

type ListTenantsResponse struct {
	Data       []TenantResponse `json:"data"`
	NextCursor *string          `json:"next_cursor"`
}

type ListBranchesResponse struct {
	Data       []BranchResponse `json:"data"`
	NextCursor *string          `json:"next_cursor"`
}

// ----- endpoint methods -------------------------------------------------------

// Register submits a POST /api/v1/register/company request.
func (c *APIClient) Register(req RegisterCompanyRequest) (RegisterCompanyResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPost, "/api/v1/register/company", req, "")
	if err != nil {
		return RegisterCompanyResponse{}, nil, err
	}
	var out RegisterCompanyResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// Login sends POST /api/v1/auth/login.
func (c *APIClient) Login(email, password string) (LoginResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{
		Email:    email,
		Password: password,
	}, "")
	if err != nil {
		return LoginResponse{}, nil, err
	}
	var out LoginResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// ChangePassword sends POST /api/v1/auth/me/password.
func (c *APIClient) ChangePassword(token, oldPassword, newPassword string) (*http.Response, []byte, error) {
	return c.do(http.MethodPost, "/api/v1/auth/me/password", ChangePasswordRequest{
		OldPassword: oldPassword,
		NewPassword: newPassword,
	}, token)
}

// GetMe sends GET /api/v1/auth/me.
func (c *APIClient) GetMe(token string) (MeResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodGet, "/api/v1/auth/me", nil, token)
	if err != nil {
		return MeResponse{}, nil, err
	}
	var out MeResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// ListRegistrations sends GET /api/v1/admin/tenant-registrations.
func (c *APIClient) ListRegistrations(token, status string) (ListRegistrationsResponse, *http.Response, error) {
	path := "/api/v1/admin/tenant-registrations"
	if status != "" {
		path += "?status=" + status
	}
	resp, raw, err := c.do(http.MethodGet, path, nil, token)
	if err != nil {
		return ListRegistrationsResponse{}, nil, err
	}
	var out ListRegistrationsResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// Approve sends POST /api/v1/admin/tenant-registrations/:id/approve.
func (c *APIClient) Approve(token, registrationID string, overrides *ApproveRegistrationRequest) (ApproveRegistrationResponse, *http.Response, error) {
	body := ApproveRegistrationRequest{}
	if overrides != nil {
		body = *overrides
	}
	resp, raw, err := c.do(http.MethodPost,
		"/api/v1/admin/tenant-registrations/"+registrationID+"/approve",
		body, token)
	if err != nil {
		return ApproveRegistrationResponse{}, nil, err
	}
	var out ApproveRegistrationResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// CreateBranch sends POST /api/v1/tenant/branches.
func (c *APIClient) CreateBranch(token string, req CreateBranchRequest) (BranchResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPost, "/api/v1/tenant/branches", req, token)
	if err != nil {
		return BranchResponse{}, nil, err
	}
	var out BranchResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// ListBranches sends GET /api/v1/tenant/branches.
func (c *APIClient) ListBranches(token string) (ListBranchesResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodGet, "/api/v1/tenant/branches", nil, token)
	if err != nil {
		return ListBranchesResponse{}, nil, err
	}
	var out ListBranchesResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// UpdateBranch sends PATCH /api/v1/tenant/branches/:id.
func (c *APIClient) UpdateBranch(token, branchID string, updates map[string]any) (BranchResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPatch, "/api/v1/tenant/branches/"+branchID, updates, token)
	if err != nil {
		return BranchResponse{}, nil, err
	}
	var out BranchResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// ChangeBranchStatus sends PATCH /api/v1/tenant/branches/:id/status.
func (c *APIClient) ChangeBranchStatus(token, branchID, status string) (BranchResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPatch, "/api/v1/tenant/branches/"+branchID+"/status",
		map[string]string{"status": status}, token)
	if err != nil {
		return BranchResponse{}, nil, err
	}
	var out BranchResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// DeleteBranch sends DELETE /api/v1/tenant/branches/:id.
func (c *APIClient) DeleteBranch(token, branchID string) (*http.Response, error) {
	resp, _, err := c.do(http.MethodDelete, "/api/v1/tenant/branches/"+branchID, nil, token)
	return resp, err
}

// ChangeTenantStatus sends PATCH /api/v1/admin/tenants/:id/status.
func (c *APIClient) ChangeTenantStatus(token, tenantID, status, reason string) (TenantResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPatch, "/api/v1/admin/tenants/"+tenantID+"/status",
		ChangeTenantStatusRequest{Status: status, Reason: reason}, token)
	if err != nil {
		return TenantResponse{}, nil, err
	}
	var out TenantResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// ErrorCode extracts the error code from a non-2xx response body.
func ErrorCode(raw []byte) string {
	var errResp ErrorResponse
	if err := json.Unmarshal(raw, &errResp); err != nil {
		return ""
	}
	return errResp.Error.Code
}

// RawDo is the escape hatch for tests that need direct access to response bytes.
func (c *APIClient) RawDo(method, path string, body any, token string) (*http.Response, []byte, error) {
	return c.do(method, path, body, token)
}

// LoginWithSlug sends POST /api/v1/auth/login with an explicit tenant_slug.
// Use "acme-spa" for Alice, "__platform__" for super admin.
func (c *APIClient) LoginWithSlug(email, password, tenantSlug string) (LoginResponse, *http.Response, error) {
	body := map[string]string{
		"email":       email,
		"password":    password,
		"tenant_slug": tenantSlug,
	}
	resp, raw, err := c.do(http.MethodPost, "/api/v1/auth/login", body, "")
	if err != nil {
		return LoginResponse{}, nil, err
	}
	var out LoginResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// ============================================================================
// Phase 4 — Therapist helpers
// ============================================================================

// ListTherapists sends GET /api/v1/tenant/therapists with optional query string.
// query may be "" or a raw query string like "branch_id=uuid&is_active=false".
func (c *APIClient) ListTherapists(token, query string) (TherapistListResponse, *http.Response, error) {
	path := "/api/v1/tenant/therapists"
	if query != "" {
		path += "?" + query
	}
	resp, raw, err := c.do(http.MethodGet, path, nil, token)
	if err != nil {
		return TherapistListResponse{}, nil, err
	}
	var out TherapistListResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// CreateTherapist sends POST /api/v1/tenant/therapists.
func (c *APIClient) CreateTherapist(token string, body CreateTherapistRequest) (TherapistResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPost, "/api/v1/tenant/therapists", body, token)
	if err != nil {
		return TherapistResponse{}, nil, err
	}
	var out TherapistResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// GetTherapist sends GET /api/v1/tenant/therapists/:id.
func (c *APIClient) GetTherapist(token, id string) (TherapistResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodGet, "/api/v1/tenant/therapists/"+id, nil, token)
	if err != nil {
		return TherapistResponse{}, nil, err
	}
	var out TherapistResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// UpdateTherapist sends PATCH /api/v1/tenant/therapists/:id.
func (c *APIClient) UpdateTherapist(token, id string, body UpdateTherapistRequest) (TherapistResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPatch, "/api/v1/tenant/therapists/"+id, body, token)
	if err != nil {
		return TherapistResponse{}, nil, err
	}
	var out TherapistResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// ChangeTherapistStatus sends PATCH /api/v1/tenant/therapists/:id/status.
func (c *APIClient) ChangeTherapistStatus(token, id string, isActive bool) (TherapistResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPatch, "/api/v1/tenant/therapists/"+id+"/status",
		TherapistStatusRequest{IsActive: isActive}, token)
	if err != nil {
		return TherapistResponse{}, nil, err
	}
	var out TherapistResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// DeleteTherapist sends DELETE /api/v1/tenant/therapists/:id.
func (c *APIClient) DeleteTherapist(token, id string) (*http.Response, error) {
	resp, _, err := c.do(http.MethodDelete, "/api/v1/tenant/therapists/"+id, nil, token)
	return resp, err
}

// ============================================================================
// Phase 4 — Service helpers
// ============================================================================

// ListServices sends GET /api/v1/tenant/services with optional query string.
func (c *APIClient) ListServices(token, query string) (ServiceListResponse, *http.Response, error) {
	path := "/api/v1/tenant/services"
	if query != "" {
		path += "?" + query
	}
	resp, raw, err := c.do(http.MethodGet, path, nil, token)
	if err != nil {
		return ServiceListResponse{}, nil, err
	}
	var out ServiceListResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// CreateService sends POST /api/v1/tenant/services.
func (c *APIClient) CreateService(token string, body CreateServiceRequest) (ServiceResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPost, "/api/v1/tenant/services", body, token)
	if err != nil {
		return ServiceResponse{}, nil, err
	}
	var out ServiceResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// GetService sends GET /api/v1/tenant/services/:id.
func (c *APIClient) GetService(token, id string) (ServiceResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodGet, "/api/v1/tenant/services/"+id, nil, token)
	if err != nil {
		return ServiceResponse{}, nil, err
	}
	var out ServiceResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// UpdateService sends PATCH /api/v1/tenant/services/:id.
func (c *APIClient) UpdateService(token, id string, body UpdateServiceRequest) (ServiceResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPatch, "/api/v1/tenant/services/"+id, body, token)
	if err != nil {
		return ServiceResponse{}, nil, err
	}
	var out ServiceResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// ChangeServiceStatus sends PATCH /api/v1/tenant/services/:id/status.
func (c *APIClient) ChangeServiceStatus(token, id string, isActive bool) (ServiceResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPatch, "/api/v1/tenant/services/"+id+"/status",
		ServiceStatusRequest{IsActive: isActive}, token)
	if err != nil {
		return ServiceResponse{}, nil, err
	}
	var out ServiceResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// DeleteService sends DELETE /api/v1/tenant/services/:id.
func (c *APIClient) DeleteService(token, id string) (*http.Response, error) {
	resp, _, err := c.do(http.MethodDelete, "/api/v1/tenant/services/"+id, nil, token)
	return resp, err
}

// ============================================================================
// Phase 4 — Service mapping helpers
// ============================================================================

// PutTherapistServices sends PUT /api/v1/tenant/therapists/:id/services.
func (c *APIClient) PutTherapistServices(token, therapistID string, serviceIDs []string) (ServiceMappingResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPut, "/api/v1/tenant/therapists/"+therapistID+"/services",
		PutServicesRequest{ServiceIDs: serviceIDs}, token)
	if err != nil {
		return ServiceMappingResponse{}, nil, err
	}
	var out ServiceMappingResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// GetTherapistServices sends GET /api/v1/tenant/therapists/:id/services.
func (c *APIClient) GetTherapistServices(token, therapistID string) (ServiceMappingResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodGet, "/api/v1/tenant/therapists/"+therapistID+"/services", nil, token)
	if err != nil {
		return ServiceMappingResponse{}, nil, err
	}
	var out ServiceMappingResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// ============================================================================
// Phase 4 — Availability helpers
// ============================================================================

// GetAvailability sends GET /api/v1/tenant/therapists/:id/availability.
func (c *APIClient) GetAvailability(token, therapistID string) (AvailabilityResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodGet, "/api/v1/tenant/therapists/"+therapistID+"/availability", nil, token)
	if err != nil {
		return AvailabilityResponse{}, nil, err
	}
	var out AvailabilityResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}

// PutAvailability sends PUT /api/v1/tenant/therapists/:id/availability.
func (c *APIClient) PutAvailability(token, therapistID string, windows []AvailabilityWindow) (AvailabilityResponse, *http.Response, error) {
	resp, raw, err := c.do(http.MethodPut, "/api/v1/tenant/therapists/"+therapistID+"/availability",
		PutAvailabilityRequest{Windows: windows}, token)
	if err != nil {
		return AvailabilityResponse{}, nil, err
	}
	var out AvailabilityResponse
	_ = decodeJSON(raw, &out)
	return out, resp, nil
}
