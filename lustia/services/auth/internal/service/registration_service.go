package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/helper"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/google/uuid"
)

// slugify converts a free-text company name to a URL-safe slug: lowercase,
// [a-z0-9-] only, collapsed hyphens, trimmed and capped at 80 chars (leaving
// room for a disambiguation suffix up to 100).
func slugify(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	prevDash := true
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 80 {
		out = strings.TrimRight(out[:80], "-")
	}
	return out
}

// resolveUniqueSlug picks a slug that collides with neither a pending
// registration nor an approved tenant. It tries the base first, then retries
// "<base>-<4hex>" up to 5 times before giving up.
//
// Returns (slug, true, nil) when a free slot is found,
//         ("",   false, nil) when all attempts collided (caller translates to ErrTenantSlugTaken),
//         ("",   false, err) on an unexpected lookup error.
func (s *RegistrationService) resolveUniqueSlug(ctx context.Context, base string) (string, error) {
	candidate := base
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			suffix := strings.ReplaceAll(uuid.New().String(), "-", "")[:4]
			candidate = base + "-" + suffix
			if len(candidate) > 100 {
				candidate = candidate[:100]
			}
		}

		taken, err := s.slugTaken(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", constants.ErrTenantSlugTaken
}

// auditConflict records which specific uniqueness rule tripped when a public
// registration is rejected. Ops can inspect audit_log for the specific reason
// without the API itself leaking it to unauthenticated callers.
func (s *RegistrationService) auditConflict(ctx context.Context, reason, email string) {
	_ = s.audit.Append(ctx, AuditEntry{
		Action:       "registration.rejected_conflict",
		ResourceType: "tenant_registration",
		Meta: map[string]interface{}{
			"reason":       reason,
			"email_prefix": helper.SHA256Prefix(email, 8),
		},
	})
}

func (s *RegistrationService) slugTaken(ctx context.Context, slug string) (bool, error) {
	if _, err := s.registrations.FindPendingBySlug(ctx, slug); err == nil {
		return true, nil
	} else if !errors.Is(err, constants.ErrRegistrationNotFound) {
		return false, fmt.Errorf("check pending by slug: %w", err)
	}
	if _, err := s.tenants.FindBySlug(ctx, slug); err == nil {
		return true, nil
	} else if !errors.Is(err, constants.ErrTenantNotFound) {
		return false, fmt.Errorf("check tenant slug: %w", err)
	}
	return false, nil
}

const (
	registrationRateLimitBurst = 3
)

// generateTemporaryPassword returns a 16-character opaque password built from
// two UUID segments. It is not stored in plain text — only its Argon2id hash
// is persisted. The raw value is returned once in the approval response and
// emailed to the contact.
func generateTemporaryPassword() string {
	// Take the first 16 characters of a UUID (no dashes) for a clean token.
	raw := uuid.New().String()
	// Remove dashes: "550e8400-e29b-41d4-a716-446655440000" → 32 hex chars, take 16.
	nodash := ""
	for _, ch := range raw {
		if ch != '-' {
			nodash += string(ch)
		}
	}
	if len(nodash) >= 16 {
		return nodash[:16]
	}
	return nodash
}

// RegistrationService handles public company-registration submissions and the
// platform-admin approval / rejection workflow.
type RegistrationService struct {
	registrations   RegistrationRepository
	tenants         TenantRepository
	users           UserRepository
	memberships     MembershipRepository
	roles           RoleRepository
	tokens          RefreshTokenRepository
	hasher          PasswordHasher
	clock           Clock
	audit           AuditRepository
	email           EmailSender
	rateLimiter     RateLimiter // per-IP and per-email (3/hour)
	globalLimiter   RateLimiter // platform-wide cap (prevents distributed abuse before CAPTCHA lands — SECURITY.md Phase 3 M-4)
	tx              TxManager
	loginURL        string
	senderName      string
}

// NewRegistrationService constructs a RegistrationService.
func NewRegistrationService(
	registrations RegistrationRepository,
	tenants TenantRepository,
	users UserRepository,
	memberships MembershipRepository,
	roles RoleRepository,
	tokens RefreshTokenRepository,
	hasher PasswordHasher,
	clock Clock,
	audit AuditRepository,
	email EmailSender,
	rateLimiter RateLimiter,
	globalLimiter RateLimiter,
	tx TxManager,
	loginURL string,
	senderName string,
) *RegistrationService {
	return &RegistrationService{
		registrations: registrations,
		tenants:       tenants,
		users:         users,
		memberships:   memberships,
		roles:         roles,
		tokens:        tokens,
		hasher:        hasher,
		clock:         clock,
		audit:         audit,
		email:         email,
		globalLimiter: globalLimiter,
		rateLimiter:   rateLimiter,
		tx:            tx,
		loginURL:      loginURL,
		senderName:    senderName,
	}
}

// SubmitRegistration handles POST /register/company.
//
// Rate-limited 3/hour per IP. Validates uniqueness then inserts a pending row.
func (s *RegistrationService) SubmitRegistration(ctx context.Context, in RegistrationInput) (RegistrationOutput, error) {
	// Rate limit per IP (key = "reg:ip:<ip>").
	if !s.rateLimiter.Allow(ctx, "reg:ip:"+in.IP) {
		return RegistrationOutput{}, constants.ErrRateLimited
	}
	// Also per email to prevent enumeration abuse (key = "reg:email:<email>").
	if !s.rateLimiter.Allow(ctx, "reg:email:"+in.ContactEmail) {
		return RegistrationOutput{}, constants.ErrRateLimited
	}
	// Platform-wide cap (SECURITY.md Phase 3 M-4): defends against distributed
	// IP rotation until CAPTCHA ships before public launch.
	if s.globalLimiter != nil && !s.globalLimiter.Allow(ctx, "reg:global") {
		return RegistrationOutput{}, constants.ErrRateLimited
	}

	// Package defaulting.
	pkg := in.Package
	if pkg == "" {
		pkg = model.TenantPackageStarter
	}

	// Dedup: pending registration with the same email.
	// SECURITY.md Phase 3 M-1: collapse every uniqueness failure to a generic
	// ErrConflict so an unauthenticated caller cannot enumerate which specific
	// field (email vs slug, pending vs approved) is taken. The specific reason
	// is captured via audit log (below) for ops observability.
	if _, err := s.registrations.FindPendingByEmail(ctx, in.ContactEmail); err == nil {
		s.auditConflict(ctx, "email_pending", in.ContactEmail)
		return RegistrationOutput{}, constants.ErrConflict
	} else if !errors.Is(err, constants.ErrRegistrationNotFound) {
		return RegistrationOutput{}, fmt.Errorf("check pending by email: %w", err)
	}

	// Resolve slug:
	//   - caller supplied one → strict check, fail loudly on any collision.
	//   - caller left it blank → derive from CompanyName, auto-suffix on collision.
	var resolvedSlug string
	if explicit := strings.TrimSpace(in.RequestedSlug); explicit != "" {
		if _, err := s.registrations.FindPendingBySlug(ctx, explicit); err == nil {
			s.auditConflict(ctx, "slug_pending", in.ContactEmail)
			return RegistrationOutput{}, constants.ErrConflict
		} else if !errors.Is(err, constants.ErrRegistrationNotFound) {
			return RegistrationOutput{}, fmt.Errorf("check pending by slug: %w", err)
		}
		if _, err := s.tenants.FindBySlug(ctx, explicit); err == nil {
			s.auditConflict(ctx, "slug_approved_tenant", in.ContactEmail)
			return RegistrationOutput{}, constants.ErrConflict
		} else if !errors.Is(err, constants.ErrTenantNotFound) {
			return RegistrationOutput{}, fmt.Errorf("check tenant slug: %w", err)
		}
		resolvedSlug = explicit
	} else {
		base := slugify(in.CompanyName)
		if base == "" {
			return RegistrationOutput{}, constants.ErrInvalidInput
		}
		chosen, err := s.resolveUniqueSlug(ctx, base)
		if err != nil {
			return RegistrationOutput{}, err
		}
		resolvedSlug = chosen
	}

	var phone *string
	if in.ContactPhone != "" {
		phone = &in.ContactPhone
	}

	reg := &model.TenantRegistration{
		ID:            uuid.New().String(),
		CompanyName:   in.CompanyName,
		RequestedSlug: resolvedSlug,
		Package:       pkg,
		ContactName:   in.ContactName,
		ContactEmail:  in.ContactEmail,
		ContactPhone:  phone,
		Status:        model.TenantRegistrationStatusPending,
		Metadata:      []byte("{}"),
	}

	if err := s.registrations.Save(ctx, reg); err != nil {
		return RegistrationOutput{}, fmt.Errorf("save registration: %w", err)
	}

	_ = s.audit.Append(ctx, AuditEntry{
		Action:       "registration.submitted",
		ResourceType: "tenant_registration",
		ResourceID:   reg.ID,
		Meta:         map[string]interface{}{"slug": resolvedSlug, "email_prefix": helper.SHA256Prefix(in.ContactEmail, 8)},
	})

	// Fire-and-forget acknowledgement email — non-fatal on SMTP failure.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		body := fmt.Sprintf(
			"Halo %s,\n\n"+
				"Terima kasih telah mendaftarkan %s di Lustia. Registrasi Anda telah\n"+
				"kami terima dan sedang menunggu peninjauan oleh tim Lustia.\n\n"+
				"Detail registrasi:\n"+
				"  ID Registrasi : %s\n"+
				"  Nama          : %s\n"+
				"  Slug          : %s\n"+
				"  Paket         : %s\n\n"+
				"Tim kami akan meninjau permohonan Anda dalam 1×24 jam pada hari\n"+
				"kerja. Anda akan menerima email lanjutan ketika status registrasi\n"+
				"berubah:\n"+
				"  • Disetujui — Anda akan menerima kredensial login admin tenant.\n"+
				"  • Ditolak  — Anda akan menerima alasan penolakan.\n\n"+
				"Jika ada pertanyaan, balas email ini.\n\n"+
				"Salam,\nTim %s",
			reg.ContactName, reg.CompanyName, reg.ID, reg.CompanyName,
			reg.RequestedSlug, reg.Package, s.senderName,
		)
		_ = s.email.Send(bgCtx, EmailMessage{
			To:       reg.ContactEmail,
			Subject:  "Registrasi perusahaan Anda telah diterima",
			TextBody: body,
		})
	}()

	return RegistrationOutput{
		RegistrationID: reg.ID,
		Status:         string(reg.Status),
	}, nil
}

// GetByID returns a single registration by ID (admin only).
func (s *RegistrationService) GetByID(ctx context.Context, id string) (RegistrationDetail, error) {
	reg, err := s.registrations.FindByID(ctx, id)
	if err != nil {
		return RegistrationDetail{}, err
	}
	return toRegistrationDetail(reg), nil
}

// ListPending lists registrations (admin only).
func (s *RegistrationService) ListPending(ctx context.Context, in ListRegistrationsInput) (ListRegistrationsOutput, error) {
	limit := in.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	page := in.Page
	if page < 1 {
		page = 1
	}

	regs, total, err := s.registrations.List(ctx, RegistrationFilter{
		Status: in.Status,
		Page:   page,
		Limit:  limit,
	})
	if err != nil {
		return ListRegistrationsOutput{}, fmt.Errorf("list registrations: %w", err)
	}

	details := make([]RegistrationDetail, len(regs))
	for i, r := range regs {
		details[i] = toRegistrationDetail(r)
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(limit) - 1) / int64(limit))
	}
	return ListRegistrationsOutput{
		Registrations: details,
		Page:          page,
		TotalCount:    total,
		TotalPages:    totalPages,
	}, nil
}

// Approve handles POST /admin/tenant-registrations/:id/approve.
//
// All writes occur inside a single transaction. After commit, a welcome email
// is sent in a fire-and-forget goroutine.
func (s *RegistrationService) Approve(ctx context.Context, in ApproveRegistrationInput) (ApproveRegistrationOutput, error) {
	var out ApproveRegistrationOutput
	var tmpPassword string

	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		// (1) Fetch and lock the registration.
		reg, err := s.registrations.FindByID(ctx, in.RegistrationID)
		if err != nil {
			return err
		}
		if !reg.IsPending() {
			return constants.ErrRegistrationNotPending
		}

		// (2) Re-verify slug is still unclaimed.
		if _, err := s.tenants.FindBySlug(ctx, reg.RequestedSlug); err == nil {
			return constants.ErrTenantSlugTaken
		} else if !errors.Is(err, constants.ErrTenantNotFound) {
			return fmt.Errorf("check slug: %w", err)
		}

		// (3) Resolve package and max_branches.
		pkg := reg.Package
		if in.Package != nil && *in.Package != "" {
			pkg = *in.Package
		}
		maxBranches := model.PackageMaxBranches(pkg)
		if in.MaxBranches != nil && *in.MaxBranches > 0 {
			maxBranches = *in.MaxBranches
		}

		// (4) Create tenant row.
		now := s.clock.Now()
		tenantID := uuid.New().String()
		tenant := &model.Tenant{
			ID:           tenantID,
			Name:         reg.CompanyName,
			Slug:         reg.RequestedSlug,
			Status:       model.TenantStatusActive,
			Package:      pkg,
			MaxBranches:  maxBranches,
			ContactEmail: &reg.ContactEmail,
			ContactName:  &reg.ContactName,
			ApprovedAt:   &now,
			ApprovedBy:   &in.CallerUserID,
		}
		if err := s.tenants.Save(ctx, tenant); err != nil {
			return fmt.Errorf("create tenant: %w", err)
		}

		// (5) Generate temporary password and create the tenant-admin user.
		tmpPassword = generateTemporaryPassword()
		pwdHash, err := s.hasher.Hash(ctx, tmpPassword)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		userID := uuid.New().String()
		newUser := &model.User{
			ID:                 userID,
			Email:              reg.ContactEmail,
			PasswordHash:       pwdHash,
			FullName:           reg.ContactName,
			Phone:              reg.ContactPhone,
			IsActive:           true,
			IsSuperAdmin:       false,
			MustChangePassword: true,
		}
		if err := s.users.Save(ctx, newUser); err != nil {
			return fmt.Errorf("create tenant admin user: %w", err)
		}

		// (6) Create active membership.
		membershipID := uuid.New().String()
		callerRef := in.CallerUserID
		membership := &model.Membership{
			ID:        membershipID,
			UserID:    userID,
			TenantID:  tenantID,
			Status:    model.MembershipStatusActive,
			JoinedAt:  now,
			Metadata:  []byte("{}"),
			CreatedBy: &callerRef,
		}
		if err := s.memberships.Save(ctx, membership); err != nil {
			return fmt.Errorf("create membership: %w", err)
		}

		// (7) Assign tenant_admin role.
		role, err := s.roles.FindByName(ctx, constants.RoleCodeTenantAdmin)
		if err != nil {
			return fmt.Errorf("find tenant_admin role: %w", err)
		}
		if err := s.memberships.AssignRoles(ctx, membershipID, []string{role.ID}, in.CallerUserID); err != nil {
			return fmt.Errorf("assign tenant_admin role: %w", err)
		}

		// (8) Update registration row.
		reg.Status = model.TenantRegistrationStatusApproved
		reg.ApprovedTenantID = &tenantID
		reg.ApprovedUserID = &userID
		reg.ApprovedAt = &now
		reg.ApprovedBy = &in.CallerUserID
		if err := s.registrations.Update(ctx, reg); err != nil {
			return fmt.Errorf("update registration: %w", err)
		}

		// (9) Audit.
		_ = s.audit.Append(ctx, AuditEntry{
			TenantID:     &tenantID,
			ActorUserID:  &in.CallerUserID,
			Action:       "tenant.approved",
			ResourceType: "tenant",
			ResourceID:   tenantID,
			Meta: map[string]interface{}{
				"registration_id": reg.ID,
				"slug":            reg.RequestedSlug,
				"package":         pkg,
			},
		})

		out = ApproveRegistrationOutput{
			Tenant: toTenantDetail(tenant),
			TenantAdmin: TenantAdminDetail{
				UserID:            userID,
				Email:             reg.ContactEmail,
				TemporaryPassword: tmpPassword,
			},
			Registration: toRegistrationDetail(reg),
		}
		return nil
	})
	if err != nil {
		return ApproveRegistrationOutput{}, err
	}

	// Fire-and-forget welcome email. Failures are logged inside the goroutine
	// but must not affect the HTTP response already in flight.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		body := fmt.Sprintf(
			"Selamat datang di %s!\n\n"+
				"Tenant Anda (%s) telah disetujui.\n\n"+
				"Login URL : %s\n"+
				"Email     : %s\n"+
				"Password  : %s\n\n"+
				"Anda akan diminta untuk mengganti password ini saat pertama kali login.\n\n"+
				"Salam,\nTim %s",
			s.senderName,
			out.Tenant.Slug,
			s.loginURL,
			out.TenantAdmin.Email,
			tmpPassword,
			s.senderName,
		)
		if sendErr := s.email.Send(bgCtx, EmailMessage{
			To:       out.TenantAdmin.Email,
			Subject:  "Selamat datang di Lustia — akun Anda siap",
			TextBody: body,
		}); sendErr != nil {
			// Non-fatal — email failure must not break the approval flow.
			_ = sendErr
		}
	}()

	return out, nil
}

// Reject handles POST /admin/tenant-registrations/:id/reject.
func (s *RegistrationService) Reject(ctx context.Context, in RejectRegistrationInput) (RejectRegistrationOutput, error) {
	reg, err := s.registrations.FindByID(ctx, in.RegistrationID)
	if err != nil {
		return RejectRegistrationOutput{}, err
	}
	if !reg.IsPending() {
		return RejectRegistrationOutput{}, constants.ErrRegistrationNotPending
	}

	now := s.clock.Now()
	reg.Status = model.TenantRegistrationStatusRejected
	reg.RejectedAt = &now
	reg.RejectedBy = &in.CallerUserID
	if in.Reason != "" {
		reg.RejectionReason = &in.Reason
	}

	if err := s.registrations.Update(ctx, reg); err != nil {
		return RejectRegistrationOutput{}, fmt.Errorf("update registration: %w", err)
	}

	_ = s.audit.Append(ctx, AuditEntry{
		ActorUserID:  &in.CallerUserID,
		Action:       "registration.rejected",
		ResourceType: "tenant_registration",
		ResourceID:   reg.ID,
		Meta:         map[string]interface{}{"reason": in.Reason},
	})

	// Optional fire-and-forget rejection email.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		reason := in.Reason
		if reason == "" {
			reason = "(tidak ada alasan)"
		}
		body := fmt.Sprintf(
			"Status registrasi perusahaan Anda\n\n"+
				"Registration ID : %s\n"+
				"Status          : Ditolak\n"+
				"Alasan          : %s\n\n"+
				"Jika Anda memiliki pertanyaan, hubungi dukungan kami.\n\n"+
				"Salam,\nTim %s",
			reg.ID, reason, s.senderName,
		)
		_ = s.email.Send(bgCtx, EmailMessage{
			To:       reg.ContactEmail,
			Subject:  "Status registrasi perusahaan Anda",
			TextBody: body,
		})
	}()

	return RejectRegistrationOutput{Registration: toRegistrationDetail(reg)}, nil
}

// ---------------------------------------------------------------------------
// Mapping helpers (registration → DTO)
// ---------------------------------------------------------------------------

func toRegistrationDetail(r *model.TenantRegistration) RegistrationDetail {
	d := RegistrationDetail{
		ID:           r.ID,
		CompanyName:  r.CompanyName,
		RequestedSlug: r.RequestedSlug,
		Package:      r.Package,
		ContactName:  r.ContactName,
		ContactEmail: r.ContactEmail,
		ContactPhone: r.ContactPhone,
		Status:       string(r.Status),
		CreatedAt:    r.CreatedAt.Format(time.RFC3339),
	}
	if r.ApprovedAt != nil {
		s := r.ApprovedAt.Format(time.RFC3339)
		d.ApprovedAt = &s
	}
	if r.ApprovedBy != nil {
		v := *r.ApprovedBy
		d.ApprovedBy = &v
	}
	if r.RejectedAt != nil {
		s := r.RejectedAt.Format(time.RFC3339)
		d.RejectedAt = &s
	}
	d.RejectionReason = r.RejectionReason
	return d
}

func toTenantDetail(t *model.Tenant) TenantDetail {
	d := TenantDetail{
		ID:          t.ID,
		Name:        t.Name,
		Slug:        t.Slug,
		Status:      t.Status,
		Package:     t.Package,
		MaxBranches: t.MaxBranches,
		CreatedAt:   t.CreatedAt.Format(time.RFC3339),
	}
	if t.ContactEmail != nil {
		d.ContactEmail = *t.ContactEmail
	}
	if t.ContactName != nil {
		d.ContactName = *t.ContactName
	}
	if t.ApprovedAt != nil {
		s := t.ApprovedAt.Format(time.RFC3339)
		d.ApprovedAt = &s
	}
	d.ApprovedBy = t.ApprovedBy
	return d
}
