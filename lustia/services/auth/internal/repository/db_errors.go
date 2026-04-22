// Package repository contains the GORM-backed repository implementations.
// These satisfy the consumer-owned interfaces declared in service/interfaces.go.
// GORM errors are translated to constants sentinels at this boundary;
// services never see gorm.ErrRecordNotFound or pgconn.PgError.
package repository

import (
	"errors"
	"strings"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL error codes relevant to this service.
const (
	pgErrUniqueViolation    = "23505" // unique constraint
	pgErrExclusionViolation = "23P01" // exclusion constraint
)

// translateDBError converts infrastructure-level PostgreSQL errors into
// sentinel errors at the adapter boundary.
func translateDBError(err error) error {
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgErrUniqueViolation:
			if strings.Contains(pgErr.ConstraintName, "email") {
				return constants.ErrDuplicateEmail
			}
			return constants.ErrConflict
		case pgErrExclusionViolation:
			return constants.ErrConflict
		}
	}

	return err
}
