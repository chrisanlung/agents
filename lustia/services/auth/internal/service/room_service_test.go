package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chrisanlung/lustia-auth/internal/constants"
	"github.com/chrisanlung/lustia-auth/internal/model"
	"github.com/chrisanlung/lustia-auth/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type stubRoomRepo struct {
	rows map[string]*model.Room
}

func newStubRoomRepo() *stubRoomRepo {
	return &stubRoomRepo{rows: make(map[string]*model.Room)}
}

func (r *stubRoomRepo) Save(_ context.Context, rm *model.Room) error {
	// Enforce uniqueness: (branch_id, name) for non-deleted rows.
	for _, existing := range r.rows {
		if existing.DeletedAt == nil && existing.BranchID == rm.BranchID && existing.Name == rm.Name {
			return constants.ErrDuplicateRoomName
		}
	}
	r.rows[rm.ID] = rm
	return nil
}

func (r *stubRoomRepo) FindByID(_ context.Context, id string) (*model.Room, error) {
	rm, ok := r.rows[id]
	if !ok || rm.DeletedAt != nil {
		return nil, constants.ErrRoomNotFound
	}
	return rm, nil
}

func (r *stubRoomRepo) FindByIDs(_ context.Context, ids []string) ([]*model.Room, error) {
	var out []*model.Room
	for _, id := range ids {
		rm, ok := r.rows[id]
		if ok && rm.DeletedAt == nil {
			out = append(out, rm)
		}
	}
	return out, nil
}

func (r *stubRoomRepo) FindByTenant(_ context.Context, tenantID string, f service.RoomFilter) ([]*model.Room, int64, error) {
	var out []*model.Room
	for _, rm := range r.rows {
		if rm.TenantID != tenantID {
			continue
		}
		if rm.DeletedAt != nil {
			continue
		}
		if f.BranchID != nil && rm.BranchID != *f.BranchID {
			continue
		}
		if f.IsActive != nil && rm.IsActive != *f.IsActive {
			continue
		}
		if f.RoomType != nil && rm.RoomType != *f.RoomType {
			continue
		}
		out = append(out, rm)
	}
	return out, int64(len(out)), nil
}

func (r *stubRoomRepo) Update(_ context.Context, rm *model.Room) error {
	existing, ok := r.rows[rm.ID]
	if !ok || existing.DeletedAt != nil {
		return constants.ErrRoomNotFound
	}
	// Check duplicate name within branch (excluding self).
	for id, other := range r.rows {
		if id == rm.ID {
			continue
		}
		if other.DeletedAt == nil && other.BranchID == rm.BranchID && other.Name == rm.Name {
			return constants.ErrDuplicateRoomName
		}
	}
	r.rows[rm.ID] = rm
	return nil
}

func (r *stubRoomRepo) UpdateStatus(_ context.Context, id string, isActive bool, _ string) error {
	rm, ok := r.rows[id]
	if !ok || rm.DeletedAt != nil {
		return constants.ErrRoomNotFound
	}
	rm.IsActive = isActive
	return nil
}

func (r *stubRoomRepo) SoftDelete(_ context.Context, id string, _ string) error {
	rm, ok := r.rows[id]
	if !ok || rm.DeletedAt != nil {
		return constants.ErrRoomNotFound
	}
	now := time.Now()
	rm.DeletedAt = &now
	rm.IsActive = false
	return nil
}

func (r *stubRoomRepo) BulkUpdateSortOrder(_ context.Context, items []service.RoomSortOrderItem) error {
	for _, it := range items {
		rm, ok := r.rows[it.ID]
		if !ok || rm.DeletedAt != nil {
			return constants.ErrRoomNotFound
		}
		rm.SortOrder = it.SortOrder
	}
	return nil
}

func (r *stubRoomRepo) UpdatePhotoKey(_ context.Context, id string, newKey *string, _ string) (*string, error) {
	rm, ok := r.rows[id]
	if !ok || rm.DeletedAt != nil {
		return nil, constants.ErrRoomNotFound
	}
	old := rm.PhotoKey
	rm.PhotoKey = newKey
	return old, nil
}

// stubBranchRepoForRoom is a minimal branch repo stub for RoomSvc tests.
type stubBranchRepoForRoom struct {
	rows map[string]*model.Branch
}

func newStubBranchRepoForRoom() *stubBranchRepoForRoom {
	return &stubBranchRepoForRoom{rows: make(map[string]*model.Branch)}
}

func (r *stubBranchRepoForRoom) FindByID(_ context.Context, id string) (*model.Branch, error) {
	b, ok := r.rows[id]
	if !ok {
		return nil, constants.ErrBranchNotFound
	}
	return b, nil
}
func (r *stubBranchRepoForRoom) FindByTenant(_ context.Context, _ string, _ service.BranchFilter) ([]*model.Branch, int64, error) {
	return nil, 0, nil
}
func (r *stubBranchRepoForRoom) Save(_ context.Context, _ *model.Branch) error         { return nil }
func (r *stubBranchRepoForRoom) Update(_ context.Context, _ *model.Branch) error       { return nil }
func (r *stubBranchRepoForRoom) UpdateStatus(_ context.Context, _, _ string, _ *time.Time) error {
	return nil
}
func (r *stubBranchRepoForRoom) SoftDelete(_ context.Context, _ string) error { return nil }

// ---------------------------------------------------------------------------
// Minimal shared stubs (room-specific names to avoid redeclaration in the
// same test package — each *_test.go file in service_test defines its own).
// ---------------------------------------------------------------------------

type noopAuditRoom struct{}

func (noopAuditRoom) Append(_ context.Context, _ service.AuditEntry) error { return nil }

type noopTxRoom struct{}

func (noopTxRoom) WithTx(_ context.Context, fn func(context.Context) error) error {
	return fn(context.Background())
}
func (noopTxRoom) SetTenantContext(_ context.Context, _, _ string) error { return nil }

type stubClockRoom struct{}

func (stubClockRoom) Now() time.Time { return time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC) }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const (
	roomTenantA = "tenant-aaa"
	roomTenantB = "tenant-bbb"
	roomBranchA = "branch-aaa"
	roomBranchB = "branch-bbb"
	roomUserA   = "user-aaa"
)

func makeRoomSvc(t *testing.T) (*service.RoomSvc, *stubRoomRepo, *stubBranchRepoForRoom) {
	t.Helper()
	roomRepo := newStubRoomRepo()
	branchRepo := newStubBranchRepoForRoom()
	audit := noopAuditRoom{}
	tx := noopTxRoom{}
	svc := service.NewRoomSvc(roomRepo, branchRepo, audit, stubClockRoom{}, tx)
	return svc, roomRepo, branchRepo
}

func makeRoomAmenities(elems []string) pq.StringArray {
	return pq.StringArray(elems)
}

func seedRoom(repo *stubRoomRepo, id, tenantID, branchID, name string) *model.Room {
	rm := &model.Room{
		ID:        id,
		TenantID:  tenantID,
		BranchID:  branchID,
		Name:      name,
		RoomType:  "single",
		Capacity:  1,
		Amenities: makeRoomAmenities([]string{}),
		IsActive:  true,
		SortOrder: 0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.rows[id] = rm
	return rm
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRoomSvc_Create_HappyPath(t *testing.T) {
	t.Parallel()
	svc, _, branchRepo := makeRoomSvc(t)
	branchRepo.rows[roomBranchA] = &model.Branch{ID: roomBranchA, TenantID: roomTenantA}

	out, err := svc.Create(context.Background(), service.CreateRoomInput{
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		IsAdmin:        true,
		BranchID:       roomBranchA,
		Name:           "VIP 1",
		RoomType:       "vip",
		Capacity:       2,
		Amenities:      []string{"shower", "tv"},
		SortOrder:      0,
	})
	require.NoError(t, err)
	assert.Equal(t, "VIP 1", out.Name)
	assert.Equal(t, "vip", out.RoomType)
	assert.Equal(t, int16(2), out.Capacity)
	assert.Equal(t, []string{"shower", "tv"}, out.Amenities)
	assert.Equal(t, roomBranchA, out.BranchID)
	assert.Empty(t, out.PhotoKey)
}

func TestRoomSvc_Create_ValidationBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modify  func(*service.CreateRoomInput)
		wantErr error
	}{
		{
			name:    "name empty",
			modify:  func(in *service.CreateRoomInput) { in.Name = "" },
			wantErr: constants.ErrInvalidInput,
		},
		{
			name:    "name too long",
			modify:  func(in *service.CreateRoomInput) { in.Name = string(make([]byte, 121)) },
			wantErr: constants.ErrInvalidInput,
		},
		{
			name:    "invalid room_type",
			modify:  func(in *service.CreateRoomInput) { in.RoomType = "penthouse" },
			wantErr: constants.ErrInvalidInput,
		},
		{
			name:    "capacity zero",
			modify:  func(in *service.CreateRoomInput) { in.Capacity = 0 },
			wantErr: constants.ErrInvalidInput,
		},
		{
			name:    "capacity above 20",
			modify:  func(in *service.CreateRoomInput) { in.Capacity = 21 },
			wantErr: constants.ErrInvalidInput,
		},
		{
			name: "amenities count above 20",
			modify: func(in *service.CreateRoomInput) {
				in.Amenities = make([]string, 21)
				for i := range in.Amenities {
					in.Amenities[i] = "ok"
				}
			},
			wantErr: constants.ErrInvalidInput,
		},
		{
			name: "amenity too long",
			modify: func(in *service.CreateRoomInput) {
				in.Amenities = []string{string(make([]byte, 81))}
			},
			wantErr: constants.ErrInvalidInput,
		},
		{
			name:    "sort_order negative",
			modify:  func(in *service.CreateRoomInput) { in.SortOrder = -1 },
			wantErr: constants.ErrInvalidInput,
		},
		{
			name:    "sort_order above 9999",
			modify:  func(in *service.CreateRoomInput) { in.SortOrder = 10000 },
			wantErr: constants.ErrInvalidInput,
		},
		{
			name:    "valid max capacity",
			modify:  func(in *service.CreateRoomInput) { in.Capacity = 20 },
			wantErr: nil,
		},
		{
			name:    "valid max sort_order",
			modify:  func(in *service.CreateRoomInput) { in.SortOrder = 9999 },
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Each subtest gets its own isolated svc + repos to avoid parallel
			// write conflicts on the in-memory stub store.
			svc, _, branchRepo := makeRoomSvc(t)
			branchRepo.rows[roomBranchA] = &model.Branch{ID: roomBranchA, TenantID: roomTenantA}

			in := service.CreateRoomInput{
				CallerUserID:   roomUserA,
				CallerTenantID: roomTenantA,
				IsAdmin:        true,
				BranchID:       roomBranchA,
				Name:           "Room",
				RoomType:       "single",
				Capacity:       1,
				SortOrder:      0,
			}
			tc.modify(&in)
			_, err := svc.Create(context.Background(), in)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr), "want %v, got %v", tc.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRoomSvc_Create_DuplicateNameWithinBranch(t *testing.T) {
	t.Parallel()
	svc, roomRepo, branchRepo := makeRoomSvc(t)
	branchRepo.rows[roomBranchA] = &model.Branch{ID: roomBranchA, TenantID: roomTenantA}
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchA, "VIP 1")

	_, err := svc.Create(context.Background(), service.CreateRoomInput{
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		IsAdmin:        true,
		BranchID:       roomBranchA,
		Name:           "VIP 1",
		RoomType:       "vip",
		Capacity:       2,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrDuplicateRoomName))
}

func TestRoomSvc_Create_DuplicateNameAllowedAcrossBranches(t *testing.T) {
	t.Parallel()
	svc, roomRepo, branchRepo := makeRoomSvc(t)
	branchRepo.rows[roomBranchA] = &model.Branch{ID: roomBranchA, TenantID: roomTenantA}
	branchRepo.rows[roomBranchB] = &model.Branch{ID: roomBranchB, TenantID: roomTenantA}
	// Seed "VIP 1" in branch A.
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchA, "VIP 1")

	// Create "VIP 1" in branch B — should succeed.
	out, err := svc.Create(context.Background(), service.CreateRoomInput{
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		IsAdmin:        true,
		BranchID:       roomBranchB,
		Name:           "VIP 1",
		RoomType:       "vip",
		Capacity:       2,
	})
	require.NoError(t, err)
	assert.Equal(t, "VIP 1", out.Name)
	assert.Equal(t, roomBranchB, out.BranchID)
}

func TestRoomSvc_Create_TenantAdminCanManageAllBranches(t *testing.T) {
	t.Parallel()
	svc, _, branchRepo := makeRoomSvc(t)
	branchRepo.rows[roomBranchB] = &model.Branch{ID: roomBranchB, TenantID: roomTenantA}

	// tenant_admin with no CallerBranches — should succeed because IsAdmin=true.
	out, err := svc.Create(context.Background(), service.CreateRoomInput{
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		CallerBranches: []string{},
		IsAdmin:        true,
		BranchID:       roomBranchB,
		Name:           "Standard Room",
		RoomType:       "single",
		Capacity:       1,
	})
	require.NoError(t, err)
	assert.Equal(t, roomBranchB, out.BranchID)
}

func TestRoomSvc_Create_BranchAdminBlockedOnForeignBranch(t *testing.T) {
	t.Parallel()
	svc, _, branchRepo := makeRoomSvc(t)
	branchRepo.rows[roomBranchB] = &model.Branch{ID: roomBranchB, TenantID: roomTenantA}

	// branch_admin only assigned to roomBranchA, trying to create in roomBranchB.
	_, err := svc.Create(context.Background(), service.CreateRoomInput{
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		CallerBranches: []string{roomBranchA},
		IsAdmin:        false,
		BranchID:       roomBranchB,
		Name:           "Room",
		RoomType:       "single",
		Capacity:       1,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrCrossBranchForbidden))
}

func TestRoomSvc_Update_BranchIdImmutable(t *testing.T) {
	t.Parallel()
	svc, roomRepo, _ := makeRoomSvc(t)
	rm := seedRoom(roomRepo, "room-1", roomTenantA, roomBranchA, "Room A")
	_ = rm

	differentBranch := roomBranchB
	_, err := svc.Update(context.Background(), service.UpdateRoomInput{
		RoomID:          "room-1",
		CallerUserID:    roomUserA,
		CallerTenantID:  roomTenantA,
		IsAdmin:         true,
		BranchIDAttempt: &differentBranch,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrRoomBranchImmutable))
}

func TestRoomSvc_Update_BranchAdminBlockedOnForeignBranch(t *testing.T) {
	t.Parallel()
	svc, roomRepo, _ := makeRoomSvc(t)
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchB, "Room B")

	newName := "Updated"
	_, err := svc.Update(context.Background(), service.UpdateRoomInput{
		RoomID:         "room-1",
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		CallerBranches: []string{roomBranchA}, // caller only in branchA
		IsAdmin:        false,
		Name:           &newName,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrCrossBranchForbidden))
}

func TestRoomSvc_Update_SameBranchIdAllowed(t *testing.T) {
	t.Parallel()
	svc, roomRepo, _ := makeRoomSvc(t)
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchA, "Room A")

	// Supplying the same branch_id should NOT cause immutability error.
	sameBranch := roomBranchA
	newName := "Updated"
	out, err := svc.Update(context.Background(), service.UpdateRoomInput{
		RoomID:          "room-1",
		CallerUserID:    roomUserA,
		CallerTenantID:  roomTenantA,
		IsAdmin:         true,
		BranchIDAttempt: &sameBranch,
		Name:            &newName,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated", out.Name)
}

func TestRoomSvc_SoftDelete_BranchAdminBlockedOnForeignBranch(t *testing.T) {
	t.Parallel()
	svc, roomRepo, _ := makeRoomSvc(t)
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchB, "Room B")

	err := svc.SoftDelete(context.Background(), roomTenantA, roomUserA, []string{roomBranchA}, false, "room-1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrCrossBranchForbidden))
}

func TestRoomSvc_SoftDelete_ExcludedFromList(t *testing.T) {
	t.Parallel()
	svc, roomRepo, _ := makeRoomSvc(t)
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchA, "Room A")

	err := svc.SoftDelete(context.Background(), roomTenantA, roomUserA, nil, true, "room-1")
	require.NoError(t, err)

	out, err := svc.List(context.Background(), service.ListRoomsInput{
		CallerTenantID: roomTenantA,
		IsAdmin:        true,
	})
	require.NoError(t, err)
	assert.Empty(t, out.Rooms)
}

func TestRoomSvc_ChangeStatus(t *testing.T) {
	t.Parallel()
	svc, roomRepo, _ := makeRoomSvc(t)
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchA, "Room A")

	out, err := svc.ChangeStatus(context.Background(), service.ChangeRoomStatusInput{
		RoomID:         "room-1",
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		IsAdmin:        true,
		IsActive:       false,
	})
	require.NoError(t, err)
	assert.False(t, out.IsActive)
}

func TestRoomSvc_ChangeStatus_BranchAdminBlockedOnForeignBranch(t *testing.T) {
	t.Parallel()
	svc, roomRepo, _ := makeRoomSvc(t)
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchB, "Room B")

	_, err := svc.ChangeStatus(context.Background(), service.ChangeRoomStatusInput{
		RoomID:         "room-1",
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		CallerBranches: []string{roomBranchA},
		IsAdmin:        false,
		IsActive:       false,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrCrossBranchForbidden))
}

func TestRoomSvc_Reorder_HappyPath(t *testing.T) {
	t.Parallel()
	svc, roomRepo, _ := makeRoomSvc(t)
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchA, "Room 1")
	seedRoom(roomRepo, "room-2", roomTenantA, roomBranchA, "Room 2")

	err := svc.Reorder(context.Background(), service.ReorderRoomsInput{
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		IsAdmin:        true,
		BranchID:       roomBranchA,
		Items: []service.RoomSortOrderItem{
			{ID: "room-1", SortOrder: 1},
			{ID: "room-2", SortOrder: 0},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, roomRepo.rows["room-1"].SortOrder)
	assert.Equal(t, 0, roomRepo.rows["room-2"].SortOrder)
}

func TestRoomSvc_Reorder_MixedBranchRejected(t *testing.T) {
	t.Parallel()
	svc, roomRepo, _ := makeRoomSvc(t)
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchA, "Room 1")
	seedRoom(roomRepo, "room-2", roomTenantA, roomBranchB, "Room 2") // different branch

	err := svc.Reorder(context.Background(), service.ReorderRoomsInput{
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		IsAdmin:        true,
		BranchID:       roomBranchA, // declared branch
		Items: []service.RoomSortOrderItem{
			{ID: "room-1", SortOrder: 0},
			{ID: "room-2", SortOrder: 1}, // belongs to different branch
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidInput))
}

func TestRoomSvc_Reorder_BranchAdminBlockedOnForeignBranch(t *testing.T) {
	t.Parallel()
	svc, roomRepo, _ := makeRoomSvc(t)
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchB, "Room 1")

	err := svc.Reorder(context.Background(), service.ReorderRoomsInput{
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		CallerBranches: []string{roomBranchA}, // caller only in branchA
		IsAdmin:        false,
		BranchID:       roomBranchB,
		Items: []service.RoomSortOrderItem{
			{ID: "room-1", SortOrder: 0},
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrCrossBranchForbidden))
}

func TestRoomSvc_Reorder_Atomicity(t *testing.T) {
	t.Parallel()
	svc, roomRepo, _ := makeRoomSvc(t)
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchA, "Room 1")
	// room-2 does not exist — batch should fail atomically.

	err := svc.Reorder(context.Background(), service.ReorderRoomsInput{
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		IsAdmin:        true,
		BranchID:       roomBranchA,
		Items: []service.RoomSortOrderItem{
			{ID: "room-1", SortOrder: 5},
			{ID: "room-nonexistent", SortOrder: 0},
		},
	})
	require.Error(t, err)
	// room-1 should NOT have been updated (atomicity: noopTx still runs fn in
	// series, but the fn returned an error before BulkUpdateSortOrder).
	assert.Equal(t, 0, roomRepo.rows["room-1"].SortOrder)
}

func TestRoomSvc_Photo_BranchAdminBlockedOnForeignBranch(t *testing.T) {
	t.Parallel()
	svc, roomRepo, _ := makeRoomSvc(t)
	seedRoom(roomRepo, "room-1", roomTenantA, roomBranchB, "Room B")

	_, err := svc.UpdatePhotoKey(context.Background(), service.UploadRoomPhotoInput{
		RoomID:         "room-1",
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		CallerBranches: []string{roomBranchA},
		IsAdmin:        false,
		NewKey:         "rooms/room-1/abc.jpg",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrCrossBranchForbidden))
}

func TestRoomSvc_AmenitiesNormalization(t *testing.T) {
	t.Parallel()
	svc, _, branchRepo := makeRoomSvc(t)
	branchRepo.rows[roomBranchA] = &model.Branch{ID: roomBranchA, TenantID: roomTenantA}

	out, err := svc.Create(context.Background(), service.CreateRoomInput{
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		IsAdmin:        true,
		BranchID:       roomBranchA,
		Name:           "Test Room",
		RoomType:       "single",
		Capacity:       1,
		// Include duplicates, uppercase, whitespace, empty strings.
		Amenities: []string{"Shower", "  TV  ", "shower", "", "  tv  ", "WIFI"},
	})
	require.NoError(t, err)
	// After normalization: lowercase, deduped, no empty, order preserved.
	assert.Equal(t, []string{"shower", "tv", "wifi"}, out.Amenities)
}

func TestRoomSvc_EmptyAmenitiesValid(t *testing.T) {
	t.Parallel()
	svc, _, branchRepo := makeRoomSvc(t)
	branchRepo.rows[roomBranchA] = &model.Branch{ID: roomBranchA, TenantID: roomTenantA}

	out, err := svc.Create(context.Background(), service.CreateRoomInput{
		CallerUserID:   roomUserA,
		CallerTenantID: roomTenantA,
		IsAdmin:        true,
		BranchID:       roomBranchA,
		Name:           "Bare Room",
		RoomType:       "single",
		Capacity:       1,
		Amenities:      []string{},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{}, out.Amenities)
}
