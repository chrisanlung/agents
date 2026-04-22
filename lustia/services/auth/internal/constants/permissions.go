package constants

// Permission codes for the auth service. These mirror the seeded permission
// table rows from migration 000005_seed_reference.

const (
	PermUserRead   = "user.read"
	PermUserCreate = "user.create"
	PermUserUpdate = "user.update"
	PermUserDelete = "user.delete"
	PermRoleRead   = "role.read"
)
