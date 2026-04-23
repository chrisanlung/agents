package constants

// Permission codes for the auth service. These mirror the seeded permission
// table rows from migration 000005_seed_reference.

const (
	PermUserRead   = "user.read"
	PermUserCreate = "user.create"
	PermUserUpdate = "user.update"
	PermUserDelete = "user.delete"
	PermRoleRead   = "role.read"

	// Phase 3 — Tenant Onboarding & Branch Setup (ADR 0008).
	PermTenantRead   = "tenant.read"
	PermTenantCreate = "tenant.create"
	PermTenantUpdate = "tenant.update"
	PermTenantDelete = "tenant.delete"
	PermTenantApprove = "tenant.approve"

	PermBranchRead   = "branch.read"
	PermBranchCreate = "branch.create"
	PermBranchUpdate = "branch.update"
	PermBranchDelete = "branch.delete"
)
