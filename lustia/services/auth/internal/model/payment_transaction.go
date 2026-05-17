package model

import "time"

// PaymentTransactionStatus mirrors the status CHECK on payment_transaction.
type PaymentTransactionStatus = string

const (
	PaymentTxnStatusAwaiting  PaymentTransactionStatus = "awaiting"
	PaymentTxnStatusPaid      PaymentTransactionStatus = "paid"
	PaymentTxnStatusSettled   PaymentTransactionStatus = "settled"
	PaymentTxnStatusFailed    PaymentTransactionStatus = "failed"
	PaymentTxnStatusExpired   PaymentTransactionStatus = "expired"
	PaymentTxnStatusDisbursed PaymentTransactionStatus = "disbursed"
	PaymentTxnStatusVoided    PaymentTransactionStatus = "voided"
)

// PaymentProvider mirrors the provider CHECK on payment_transaction.
type PaymentProvider = string

const (
	PaymentProviderDummy    PaymentProvider = "dummy"
	PaymentProviderIPaymu   PaymentProvider = "ipaymu"
	PaymentProviderMidtrans PaymentProvider = "midtrans"
)

// PaymentTransaction maps to the payment_transaction table (migration 000029).
// One row per booking; tracks the full money lifecycle per ADR 0015 §2.1.
//
// Phase 6 invariant: UNIQUE (booking_id) — one transaction per booking.
// Retry / QR refresh uses Reset (UPDATE), not INSERT.
type PaymentTransaction struct {
	ID       string `gorm:"column:id;primaryKey;type:uuid"`
	TenantID string `gorm:"column:tenant_id;not null;type:uuid"`

	// Booking linkage.
	BookingID string `gorm:"column:booking_id;not null;type:uuid;uniqueIndex:pt_booking_id_uidx"`

	// Provider.
	Provider          PaymentProvider `gorm:"column:provider;not null"`
	ProviderReference string          `gorm:"column:provider_reference;not null;uniqueIndex:pt_provider_reference_uidx"`

	// Payment channel (migration 000037). Values: "qris", "va_bca", "va_mandiri",
	// "va_bni", "va_bri", "va_permata", "va_cimb". Existing rows default to "qris".
	Channel string `gorm:"column:channel;not null;default:qris"`

	// QR display data (used when Channel = "qris").
	QRString    *string   `gorm:"column:qr_string"`
	QRImageURL  *string   `gorm:"column:qr_image_url"`
	QRExpiresAt time.Time `gorm:"column:qr_expires_at;not null"`

	// Virtual Account display data (used when Channel starts with "va_").
	VANumber *string `gorm:"column:va_number"`
	VABank   *string `gorm:"column:va_bank"`

	// Amounts (whole IDR).
	ExpectedAmountIDR int64  `gorm:"column:expected_amount_idr;not null"`
	ReceivedAmountIDR *int64 `gorm:"column:received_amount_idr"`
	PlatformFeeIDR    *int64 `gorm:"column:platform_fee_idr"`
	TenantNetIDR      *int64 `gorm:"column:tenant_net_idr"`

	// Lifecycle.
	Status PaymentTransactionStatus `gorm:"column:status;not null;default:awaiting"`

	// Transition timestamps.
	PaidAt      *time.Time `gorm:"column:paid_at"`
	SettledAt   *time.Time `gorm:"column:settled_at"`
	DisbursedAt *time.Time `gorm:"column:disbursed_at"`

	// Cross-table linkage (set on transition).
	SettlementBatchID *string `gorm:"column:settlement_batch_id;type:uuid"`
	DisbursementID    *string `gorm:"column:disbursement_id;type:uuid"`

	// Audit.
	RawWebhook *[]byte   `gorm:"column:raw_webhook;type:jsonb"`
	CreatedAt  time.Time `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

// TableName returns the Postgres table name.
func (PaymentTransaction) TableName() string { return "payment_transaction" }
