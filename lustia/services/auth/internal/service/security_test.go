package service_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserModelPasswordHashNotSerialized locks in SECURITY.md §I-1 / §7.3:
// the GORM User model must never include the password hash in any JSON-
// serialised representation, even if someone accidentally passes it through
// a default encoder.
func TestUserModelPasswordHashNotSerialized(t *testing.T) {
	u := model.User{
		ID:                 "test-id",
		Email:              "alice@example.com",
		PasswordHash:       "$argon2id$v=19$m=65536,t=3,p=2$SHOULD_NOT_LEAK$SHOULD_NOT_LEAK",
		FullName:           "Alice",
		IsActive:           true,
		MustChangePassword: true,
	}

	b, err := json.Marshal(u)
	require.NoError(t, err)

	// Key-level check: the "password_hash" key must not appear.
	// json:"-" on the field makes encoding/json skip it entirely.
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.NotContains(t, decoded, "password_hash", "User JSON must not expose password_hash")
	assert.NotContains(t, decoded, "PasswordHash", "User JSON must not expose PasswordHash")

	// Value-level defence: the actual hash value must not appear anywhere.
	assert.NotContains(t, string(b), "SHOULD_NOT_LEAK", "hash value must not appear anywhere in the serialised form")
}

// TestUserProfileHasNoPasswordField locks in SECURITY.md §I-1: the service-
// layer UserProfile DTO that flows to HTTP responses must not carry the
// password hash field at all. This guards against future refactors that
// accidentally add one.
func TestUserProfileHasNoPasswordField(t *testing.T) {
	p := service.UserProfile{}
	tp := reflect.TypeOf(p)

	for i := 0; i < tp.NumField(); i++ {
		name := strings.ToLower(tp.Field(i).Name)
		assert.False(t, strings.Contains(name, "password"),
			"service.UserProfile must not expose any password-related field, found %q", tp.Field(i).Name)
		assert.False(t, strings.Contains(name, "hash"),
			"service.UserProfile must not expose any hash-related field, found %q", tp.Field(i).Name)
	}
}

// TestChangePasswordInputHashFieldsAreInternal documents that the only place a
// password hash exists in the service layer is the internal User model. DTOs
// that flow in/out only carry plaintexts on input and nothing on output.
func TestChangePasswordInputHashFieldsAreInternal(t *testing.T) {
	in := service.ChangePasswordInput{}
	tp := reflect.TypeOf(in)
	sawOld, sawNew := false, false
	for i := 0; i < tp.NumField(); i++ {
		switch tp.Field(i).Name {
		case "OldPassword":
			sawOld = true
		case "NewPassword":
			sawNew = true
		case "PasswordHash", "OldPasswordHash", "NewPasswordHash":
			t.Fatalf("ChangePasswordInput must not carry a hashed field, found %q", tp.Field(i).Name)
		}
	}
	assert.True(t, sawOld, "ChangePasswordInput must carry OldPassword (plaintext)")
	assert.True(t, sawNew, "ChangePasswordInput must carry NewPassword (plaintext)")
}
