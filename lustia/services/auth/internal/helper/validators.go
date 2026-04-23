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

// RegisterCustomValidators registers Lustia-specific validator tags on the Gin
// default validator engine. Call once at service startup, before route
// registration.
func RegisterCustomValidators() error {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return nil // non-fatal: not the validator we expect, bindings continue without custom tags
	}
	return v.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
		return slugPattern.MatchString(fl.Field().String())
	})
}
