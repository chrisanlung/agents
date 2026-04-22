package constants

// RoleCode mirrors the seeded role names in the database.
// These values are fixed at migration time (000005_seed_reference).
type RoleCode = string

const (
	RoleCodeSuperAdmin  RoleCode = "super_admin"
	RoleCodeTenantAdmin RoleCode = "tenant_admin"
	RoleCodeBranchAdmin RoleCode = "branch_admin"
	RoleCodeFinance     RoleCode = "finance"
	RoleCodeTherapist   RoleCode = "therapist"
	RoleCodeCustomer    RoleCode = "customer"
)
