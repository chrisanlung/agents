package model

import "time"

// TherapistAvailability maps to the therapist_availability table.
// The table was fully specified in migration 000003. start_time and end_time
// are stored as TIME in Postgres — we serialise them as "HH:MM:SS" strings in
// Go because lib/pq returns a string for TIME columns and cannot Scan directly
// into time.Time. The service layer does all comparisons via string (TIME
// strings are lexicographically comparable) or parses into time.Time when
// arithmetic is needed.
type TherapistAvailability struct {
	ID             string     `gorm:"column:id;primaryKey;type:uuid"`
	TenantID       string     `gorm:"column:tenant_id;not null;type:uuid"`
	TherapistID    string     `gorm:"column:therapist_id;not null;type:uuid"`
	BranchID       string     `gorm:"column:branch_id;not null;type:uuid"`
	DayOfWeek      int        `gorm:"column:day_of_week;not null"` // 0=Sunday … 6=Saturday
	StartTime      string     `gorm:"column:start_time;type:time;not null"`
	EndTime        string     `gorm:"column:end_time;type:time;not null"`
	EffectiveFrom  *time.Time `gorm:"column:effective_from"`
	EffectiveUntil *time.Time `gorm:"column:effective_until"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null;autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`
	CreatedBy      *string    `gorm:"column:created_by;type:uuid"`
	UpdatedBy      *string    `gorm:"column:updated_by;type:uuid"`
}

// TableName returns the Postgres table name.
func (TherapistAvailability) TableName() string { return "therapist_availability" }

// StartHHMM returns the start time formatted as "HH:MM" (strips seconds from
// the stored "HH:MM:SS" representation).
func (a TherapistAvailability) StartHHMM() string { return trimHHMM(a.StartTime) }

// EndHHMM returns the end time formatted as "HH:MM".
func (a TherapistAvailability) EndHHMM() string { return trimHHMM(a.EndTime) }

func trimHHMM(t string) string {
	if len(t) >= 5 {
		return t[:5]
	}
	return t
}
