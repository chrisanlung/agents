package helper

import (
	"regexp"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// slugPattern: lowercase alnum with optional internal hyphens. No leading or
// trailing hyphen, no consecutive hyphens, no unicode. See SECURITY.md Phase 3
// M-3 — prevents callers from injecting path-like or non-ASCII slugs via the
// explicit RequestedSlug field.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// usernamePattern: lowercase alphanumeric + dot/underscore, 3–50 characters.
// Mirrors the DB CHECK constraint in migration 000033.
var usernamePattern = regexp.MustCompile(`^[a-z0-9._]{3,50}$`)

// ValidateUsername returns true when s satisfies the username format rules:
//   - 3–50 characters
//   - lowercase letters, digits, dots, and underscores only
//   - already lowercased (uppercase inputs must be lowercased by the caller first)
func ValidateUsername(s string) bool {
	return usernamePattern.MatchString(s)
}

// RegisterCustomValidators registers Lustia-specific validator tags on the Gin
// default validator engine. Call once at service startup, before route
// registration.
func RegisterCustomValidators() error {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return nil // non-fatal: not the validator we expect, bindings continue without custom tags
	}
	if err := v.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
		return slugPattern.MatchString(fl.Field().String())
	}); err != nil {
		return err
	}
	return v.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		s := fl.Field().String()
		if s == "" {
			return true // omitempty handles empty — not our concern here
		}
		return usernamePattern.MatchString(s)
	})
}
