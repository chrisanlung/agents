package model

import (
	"time"

	"github.com/lib/pq"
)

// Room maps to the room table (migration 000021).
// Branch-scoped physical room catalog. Each room can be allocated to one
// booking at a time; the booking-engine double-booking constraint (Phase 5)
// will enforce this via a tstzrange exclusion constraint on the booking table.
// See ADR 0012 (docs/DECISIONS/0012-rooms.md).
type Room struct {
	ID          string              `gorm:"column:id;primaryKey;type:uuid"`
	TenantID    string              `gorm:"column:tenant_id;not null;type:uuid"`
	BranchID    string              `gorm:"column:branch_id;not null;type:uuid"`
	Name        string              `gorm:"column:name;not null"`
	Description *string             `gorm:"column:description"`
	RoomType    string              `gorm:"column:room_type;not null"`
	Capacity    int16          `gorm:"column:capacity;not null;default:1"`
	Amenities   pq.StringArray `gorm:"column:amenities;not null;default:'{}';type:text[]"`
	PhotoKey    *string        `gorm:"column:photo_key"`
	IsActive    bool                `gorm:"column:is_active;not null;default:true"`
	SortOrder   int                 `gorm:"column:sort_order;not null;default:0"`
	CreatedAt   time.Time           `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt   time.Time           `gorm:"column:updated_at;not null;autoUpdateTime"`
	CreatedBy   *string             `gorm:"column:created_by;type:uuid"`
	UpdatedBy   *string             `gorm:"column:updated_by;type:uuid"`
	DeletedAt   *time.Time          `gorm:"column:deleted_at"`
}

// TableName returns the Postgres table name.
func (Room) TableName() string { return "room" }

// IsSoftDeleted returns true when the room has been soft-deleted.
func (r Room) IsSoftDeleted() bool { return r.DeletedAt != nil }

// AmenitiesSlice returns the amenities as a plain []string.
func (r *Room) AmenitiesSlice() []string {
	if r.Amenities == nil {
		return []string{}
	}
	return []string(r.Amenities)
}
