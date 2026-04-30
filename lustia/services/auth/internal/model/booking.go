package model

import "time"

// BookingStatus mirrors the status CHECK constraint on the booking table.
type BookingStatus = string

const (
	BookingStatusPendingPayment BookingStatus = "pending_payment"
	BookingStatusPaid           BookingStatus = "paid"
	BookingStatusCheckedIn      BookingStatus = "checked_in"
	BookingStatusCompleted      BookingStatus = "completed"
	BookingStatusExpired        BookingStatus = "expired"
	BookingStatusCancelled      BookingStatus = "cancelled"
	BookingStatusNoShow         BookingStatus = "no_show"
)

// BookingPaymentMethod mirrors the payment_method CHECK constraint.
type BookingPaymentMethod = string

const (
	BookingPaymentMidtrans BookingPaymentMethod = "midtrans"
	BookingPaymentAtVenue  BookingPaymentMethod = "paid_at_venue"
	BookingPaymentIPaymu   BookingPaymentMethod = "ipaymu"
	BookingPaymentDummy    BookingPaymentMethod = "dummy"
)

// Booking maps to the booking table (migration 000025).
// No deleted_at: lifecycle is entirely expressed through status.
// See ADR 0014 §3.13 and ADR 0002 (exclusion constraint design).
type Booking struct {
	ID         string  `gorm:"column:id;primaryKey;type:uuid"`
	TenantID   string  `gorm:"column:tenant_id;not null;type:uuid"`
	BranchID   string  `gorm:"column:branch_id;not null;type:uuid"`
	ServiceID  string  `gorm:"column:service_id;not null;type:uuid"`
	RoomID     *string `gorm:"column:room_id;type:uuid"`
	TherapistID *string `gorm:"column:therapist_id;type:uuid"`

	// Customer info captured at booking time — no customer account.
	CustomerName  string `gorm:"column:customer_name;not null"`
	CustomerPhone string `gorm:"column:customer_phone;not null"`
	CustomerEmail string `gorm:"column:customer_email;not null"`

	// Human-readable booking code: "XXXX-XXXX" Crockford Base32. See ADR 0014 §3.4.
	Code string `gorm:"column:code;not null;uniqueIndex"`

	// Scheduled slot.
	ScheduledStart time.Time `gorm:"column:scheduled_start;not null"`
	ScheduledEnd   time.Time `gorm:"column:scheduled_end;not null"`

	// Price snapshot: server-computed at booking creation, never trusted from client.
	// C-1: total_price_idr is ONLY written by the service layer. See SECURITY.md C-1.
	TotalPriceIDR int64 `gorm:"column:total_price_idr;not null"`

	// Payment fields.
	PaymentMethod    *string    `gorm:"column:payment_method"`
	PaymentReference *string    `gorm:"column:payment_reference"`
	PaidAt           *time.Time `gorm:"column:paid_at"`

	// Lifecycle status.
	Status BookingStatus `gorm:"column:status;not null;default:pending_payment"`

	// State-transition audit columns.
	CancelledAt  *time.Time `gorm:"column:cancelled_at"`
	CancelledBy  *string    `gorm:"column:cancelled_by;type:uuid"`
	CancelReason *string    `gorm:"column:cancel_reason"`
	CheckedInAt  *time.Time `gorm:"column:checked_in_at"`
	CheckedInBy  *string    `gorm:"column:checked_in_by;type:uuid"`
	CompletedAt  *time.Time `gorm:"column:completed_at"`
	CompletedBy  *string    `gorm:"column:completed_by;type:uuid"`

	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

// TableName returns the Postgres table name.
func (Booking) TableName() string { return "booking" }

// BookingAddon maps to the booking_addon table (migration 000025).
// price_idr is a point-in-time snapshot of addon.price_idr at booking creation.
// This table is INSERT-only; no UPDATE or DELETE grants.
type BookingAddon struct {
	BookingID string `gorm:"column:booking_id;not null;type:uuid;primaryKey"`
	AddonID   string `gorm:"column:addon_id;not null;type:uuid;primaryKey"`
	PriceIDR  int64  `gorm:"column:price_idr;not null"`
}

// TableName returns the Postgres table name.
func (BookingAddon) TableName() string { return "booking_addon" }
