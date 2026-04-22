package repository

import (
	"context"
	"fmt"

	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditRepository implements service.AuditRepository.
// Errors are logged but never propagated — audit failures must not block
// business operations.
type AuditRepository struct {
	db *gorm.DB
}

// NewAuditRepository constructs an AuditRepository.
func NewAuditRepository(db *gorm.DB) *AuditRepository { return &AuditRepository{db: db} }

func (r *AuditRepository) Append(ctx context.Context, entry service.AuditEntry) error {
	// Audit log uses the root DB (not a transaction-scoped one) so that
	// audit entries survive even if the calling transaction rolls back.
	m := model.AuditLog{
		ID:           uuid.New().String(),
		Action:       entry.Action,
		ResourceType: entry.ResourceType,
		Meta:         entry.Meta,
	}
	if entry.TenantID != nil {
		m.TenantID = entry.TenantID
	}
	if entry.ActorUserID != nil {
		m.ActorUserID = entry.ActorUserID
	}
	if entry.ResourceID != "" {
		m.ResourceID = &entry.ResourceID
	}

	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		// Swallow — audit must not block callers.
		return fmt.Errorf("audit append (non-fatal): %w", err)
	}
	return nil
}
