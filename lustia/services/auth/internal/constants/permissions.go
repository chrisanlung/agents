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

	// Phase 4 — Master Operational Data (ADR 0009).
	PermTherapistRead   = "therapist.read"
	PermTherapistCreate = "therapist.create"
	PermTherapistUpdate = "therapist.update"
	PermTherapistDelete = "therapist.delete"

	PermServiceRead   = "service.read"
	PermServiceCreate = "service.create"
	PermServiceUpdate = "service.update"
	PermServiceDelete = "service.delete"

	PermAvailabilityRead   = "availability.read"
	PermAvailabilityCreate = "availability.create"
	PermAvailabilityUpdate = "availability.update"
	PermAvailabilityDelete = "availability.delete"

	// ADR 0010 — Tenant-wide add-on catalog.
	PermAddonRead   = "addon.read"
	PermAddonCreate = "addon.create"
	PermAddonUpdate = "addon.update"
	PermAddonDelete = "addon.delete"

	// ADR 0012 — Room (Ruangan) catalog.
	PermRoomRead   = "room.read"
	PermRoomCreate = "room.create"
	PermRoomUpdate = "room.update"
	PermRoomDelete = "room.delete"

	// ADR 0014 — Phase 5 Booking Engine.
	PermBookingRead     = "booking.read"
	PermBookingCreate   = "booking.create"
	PermBookingCancel   = "booking.cancel"
	PermBookingCheckin  = "booking.checkin"
	PermBookingComplete = "booking.complete"
	PermBookingNoShow   = "booking.no_show"
)
