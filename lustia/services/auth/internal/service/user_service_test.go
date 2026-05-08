package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fakes for UserService tests
// ---------------------------------------------------------------------------

type fakeUserRepoForUserSvc struct {
	findByEmail func(ctx context.Context, email string) (*model.User, error)
	save        func(ctx context.Context, u *model.User) error
	findByID    func(ctx context.Context, id string) (*model.User, error)
	update      func(ctx context.Context, u *model.User) error
}

func (f *fakeUserRepoForUserSvc) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	if f.findByEmail != nil {
		return f.findByEmail(ctx, email)
	}
	return nil, constants.ErrUserNotFound
}
func (f *fakeUserRepoForUserSvc) FindByUsername(_ context.Context, _ string) (*model.User, error) {
	return nil, constants.ErrUserNotFound
}
func (f *fakeUserRepoForUserSvc) FindByID(ctx context.Context, id string) (*model.User, error) {
	if f.findByID != nil {
		return f.findByID(ctx, id)
	}
	return nil, constants.ErrUserNotFound
}
func (f *fakeUserRepoForUserSvc) FindByTenant(_ context.Context, _ string, _ service.UserFilter) ([]*model.User, int64, error) {
	return nil, 0, nil
}
func (f *fakeUserRepoForUserSvc) Save(ctx context.Context, u *model.User) error {
	if f.save != nil {
		return f.save(ctx, u)
	}
	return nil
}
func (f *fakeUserRepoForUserSvc) Update(ctx context.Context, u *model.User) error {
	if f.update != nil {
		return f.update(ctx, u)
	}
	return nil
}
func (f *fakeUserRepoForUserSvc) UpdatePassword(_ context.Context, _ string, _ string) error {
	return nil
}
func (f *fakeUserRepoForUserSvc) IncrementFailedLogin(_ context.Context, _ string, _ *time.Time) error {
	return nil
}
func (f *fakeUserRepoForUserSvc) ResetFailedLogin(_ context.Context, _ string) error { return nil }
func (f *fakeUserRepoForUserSvc) SoftDelete(_ context.Context, _ string) error       { return nil }

type fakeMembershipForUserSvc struct {
	fakeMembershipRepo
	findByUserAndTenant func(ctx context.Context, userID, tenantID string) (*model.Membership, error)
	findByID            func(ctx context.Context, id string) (*model.Membership, error)
}

func (f *fakeMembershipForUserSvc) FindByUserAndTenant(ctx context.Context, userID, tenantID string) (*model.Membership, error) {
	if f.findByUserAndTenant != nil {
		return f.findByUserAndTenant(ctx, userID, tenantID)
	}
	return nil, constants.ErrMembershipNotFound
}
func (f *fakeMembershipForUserSvc) FindByID(ctx context.Context, id string) (*model.Membership, error) {
	if f.findByID != nil {
		return f.findByID(ctx, id)
	}
	return &model.Membership{}, nil
}

type fakeRoleRepoForUserSvc struct{}

func (f *fakeRoleRepoForUserSvc) FindAll(_ context.Context) ([]*model.Role, error) { return nil, nil }
func (f *fakeRoleRepoForUserSvc) FindByIDs(_ context.Context, _ []string) ([]*model.Role, error) {
	return nil, nil
}
func (f *fakeRoleRepoForUserSvc) FindByName(_ context.Context, _ string) (*model.Role, error) {
	return nil, constants.ErrRoleNotFound
}

func newTestUserService(
	users service.UserRepository,
	memberships service.MembershipRepository,
) *service.UserService {
	return service.NewUserService(
		users,
		memberships,
		&fakeRoleRepoForUserSvc{},
		&fakeHasher{},
		&fakeClock{},
		&fakeAuditRepo{},
		&fakeTxManager{},
	)
}

// ---------------------------------------------------------------------------
// Username validation in CreateUser (spec §6)
// ---------------------------------------------------------------------------

func TestCreateUser_Username_Valid_Lowercased(t *testing.T) {
	t.Parallel()
	// A valid, already-lowercase username is accepted and stored on the new user row.

	var savedUser *model.User
	svc := newTestUserService(
		&fakeUserRepoForUserSvc{
			save: func(_ context.Context, u *model.User) error {
				savedUser = u
				return nil
			},
		},
		&fakeMembershipForUserSvc{},
	)

	username := "alice_99"
	_, err := svc.CreateUser(context.Background(), service.CreateUserInput{
		CallerUserID:   "admin1",
		CallerTenantID: "t1",
		Email:          "alice@example.com",
		Username:       &username,
		FullName:       "Alice",
	})

	require.NoError(t, err)
	require.NotNil(t, savedUser)
	require.NotNil(t, savedUser.Username)
	assert.Equal(t, "alice_99", *savedUser.Username)
}

func TestCreateUser_Username_UppercaseNormalized(t *testing.T) {
	t.Parallel()
	// Uppercase username input is lowercased automatically.

	var savedUser *model.User
	svc := newTestUserService(
		&fakeUserRepoForUserSvc{
			save: func(_ context.Context, u *model.User) error {
				savedUser = u
				return nil
			},
		},
		&fakeMembershipForUserSvc{},
	)

	upper := "ALICE"
	_, err := svc.CreateUser(context.Background(), service.CreateUserInput{
		CallerUserID:   "admin1",
		CallerTenantID: "t1",
		Email:          "alice@example.com",
		Username:       &upper,
		FullName:       "Alice",
	})

	require.NoError(t, err)
	require.NotNil(t, savedUser)
	require.NotNil(t, savedUser.Username)
	assert.Equal(t, "alice", *savedUser.Username, "uppercase username must be lowercased")
}

func TestCreateUser_Username_TooShort(t *testing.T) {
	t.Parallel()
	// Username < 3 chars → ErrUsernameInvalid.

	svc := newTestUserService(&fakeUserRepoForUserSvc{}, &fakeMembershipForUserSvc{})

	short := "ab"
	_, err := svc.CreateUser(context.Background(), service.CreateUserInput{
		CallerUserID:   "admin1",
		CallerTenantID: "t1",
		Email:          "alice@example.com",
		Username:       &short,
		FullName:       "Alice",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrUsernameInvalid),
		"username < 3 chars must return ErrUsernameInvalid; got: %v", err)
}

func TestCreateUser_Username_TooLong(t *testing.T) {
	t.Parallel()
	// Username > 50 chars → ErrUsernameInvalid.

	svc := newTestUserService(&fakeUserRepoForUserSvc{}, &fakeMembershipForUserSvc{})

	long := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmno" // 51 chars
	_, err := svc.CreateUser(context.Background(), service.CreateUserInput{
		CallerUserID:   "admin1",
		CallerTenantID: "t1",
		Email:          "alice@example.com",
		Username:       &long,
		FullName:       "Alice",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrUsernameInvalid),
		"username > 50 chars must return ErrUsernameInvalid; got: %v", err)
}

func TestCreateUser_Username_InvalidChars(t *testing.T) {
	t.Parallel()
	// Characters outside [a-z0-9._] are rejected.

	svc := newTestUserService(&fakeUserRepoForUserSvc{}, &fakeMembershipForUserSvc{})

	cases := []struct {
		name     string
		username string
	}{
		{"hyphen", "alice-bob"},
		{"space", "alice bob"},
		{"at sign", "alice@bob"},
		{"slash", "alice/bob"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := tc.username
			_, err := svc.CreateUser(context.Background(), service.CreateUserInput{
				CallerUserID:   "admin1",
				CallerTenantID: "t1",
				Email:          "alice@example.com",
				Username:       &u,
				FullName:       "Alice",
			})
			assert.True(t, errors.Is(err, constants.ErrUsernameInvalid),
				"invalid char %q must return ErrUsernameInvalid; got: %v", tc.username, err)
		})
	}
}

func TestCreateUser_Username_Nil_Allowed(t *testing.T) {
	t.Parallel()
	// Omitting username (nil) is valid — username is optional.

	var savedUser *model.User
	svc := newTestUserService(
		&fakeUserRepoForUserSvc{
			save: func(_ context.Context, u *model.User) error {
				savedUser = u
				return nil
			},
		},
		&fakeMembershipForUserSvc{},
	)

	_, err := svc.CreateUser(context.Background(), service.CreateUserInput{
		CallerUserID:   "admin1",
		CallerTenantID: "t1",
		Email:          "nouser@example.com",
		Username:       nil, // no username
		FullName:       "No User",
	})

	require.NoError(t, err)
	require.NotNil(t, savedUser)
	assert.Nil(t, savedUser.Username, "nil username must remain nil on new user row")
}

func TestCreateUser_Username_DuplicateTaken(t *testing.T) {
	t.Parallel()
	// When the DB returns ErrUsernameAlreadyTaken the service surfaces it unchanged.

	svc := newTestUserService(
		&fakeUserRepoForUserSvc{
			save: func(_ context.Context, _ *model.User) error {
				return constants.ErrUsernameAlreadyTaken
			},
		},
		&fakeMembershipForUserSvc{},
	)

	u := "taken"
	_, err := svc.CreateUser(context.Background(), service.CreateUserInput{
		CallerUserID:   "admin1",
		CallerTenantID: "t1",
		Email:          "new@example.com",
		Username:       &u,
		FullName:       "New",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrUsernameAlreadyTaken))
}

// ---------------------------------------------------------------------------
// Username validation in UpdateUser (spec §6)
// ---------------------------------------------------------------------------

func TestUpdateUser_Username_SetValid(t *testing.T) {
	t.Parallel()
	// UsernameSet=true with a valid value updates the user row.

	var updatedUser *model.User
	existingUser := &model.User{ID: "u1", Email: "u@x.com", FullName: "U", IsActive: true}
	membership := &model.Membership{ID: "m1", UserID: "u1", TenantID: "t1", Status: model.MembershipStatusActive}

	svc := newTestUserService(
		&fakeUserRepoForUserSvc{
			findByID: func(_ context.Context, _ string) (*model.User, error) {
				return existingUser, nil
			},
			update: func(_ context.Context, u *model.User) error {
				updatedUser = u
				return nil
			},
		},
		&fakeMembershipForUserSvc{
			findByUserAndTenant: func(_ context.Context, _, _ string) (*model.Membership, error) {
				return membership, nil
			},
			findByID: func(_ context.Context, _ string) (*model.Membership, error) {
				return membership, nil
			},
		},
	)

	newUsername := "bob.smith"
	_, err := svc.UpdateUser(context.Background(), service.UpdateUserInput{
		CallerUserID:   "admin1",
		CallerTenantID: "t1",
		TargetUserID:   "u1",
		Username:       &newUsername,
		UsernameSet:    true,
	})

	require.NoError(t, err)
	require.NotNil(t, updatedUser)
	require.NotNil(t, updatedUser.Username)
	assert.Equal(t, "bob.smith", *updatedUser.Username)
}

func TestUpdateUser_Username_NotSet_NoChange(t *testing.T) {
	t.Parallel()
	// UsernameSet=false: username field is ignored even if non-nil.

	origUsername := "original"
	existingUser := &model.User{
		ID: "u1", Email: "u@x.com", FullName: "U", IsActive: true,
		Username: &origUsername,
	}
	membership := &model.Membership{ID: "m1", UserID: "u1", TenantID: "t1", Status: model.MembershipStatusActive}

	var updatedUser *model.User
	svc := newTestUserService(
		&fakeUserRepoForUserSvc{
			findByID: func(_ context.Context, _ string) (*model.User, error) {
				return existingUser, nil
			},
			update: func(_ context.Context, u *model.User) error {
				updatedUser = u
				return nil
			},
		},
		&fakeMembershipForUserSvc{
			findByUserAndTenant: func(_ context.Context, _, _ string) (*model.Membership, error) {
				return membership, nil
			},
			findByID: func(_ context.Context, _ string) (*model.Membership, error) {
				return membership, nil
			},
		},
	)

	attempt := "hacker"
	_, err := svc.UpdateUser(context.Background(), service.UpdateUserInput{
		CallerUserID:   "admin1",
		CallerTenantID: "t1",
		TargetUserID:   "u1",
		Username:       &attempt,
		UsernameSet:    false, // key absent from JSON body — must be ignored
	})

	require.NoError(t, err)
	require.NotNil(t, updatedUser)
	// Username must remain at original value — the attempted change was ignored.
	require.NotNil(t, updatedUser.Username)
	assert.Equal(t, "original", *updatedUser.Username)
}

func TestUpdateUser_Username_Invalid_Rejected(t *testing.T) {
	t.Parallel()
	// An invalid username in an update is rejected before any DB access.

	existingUser := &model.User{ID: "u1", Email: "u@x.com", FullName: "U", IsActive: true}
	membership := &model.Membership{ID: "m1", UserID: "u1", TenantID: "t1", Status: model.MembershipStatusActive}

	svc := newTestUserService(
		&fakeUserRepoForUserSvc{
			findByID: func(_ context.Context, _ string) (*model.User, error) {
				return existingUser, nil
			},
		},
		&fakeMembershipForUserSvc{
			findByUserAndTenant: func(_ context.Context, _, _ string) (*model.Membership, error) {
				return membership, nil
			},
		},
	)

	bad := "bad-name!" // hyphen + exclamation mark — invalid
	_, err := svc.UpdateUser(context.Background(), service.UpdateUserInput{
		CallerUserID:   "admin1",
		CallerTenantID: "t1",
		TargetUserID:   "u1",
		Username:       &bad,
		UsernameSet:    true,
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrUsernameInvalid))
}
