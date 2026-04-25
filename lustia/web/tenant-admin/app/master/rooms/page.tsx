import type { Metadata } from "next";
import Link from "next/link";
import { AlertCircle, DoorOpen, Plus } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import type {
  RoomListResponse,
  Room,
  BranchListResponse,
  Branch,
} from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { FilterSelect } from "@/components/filter-select";
import { FilterBar } from "@/components/filter-bar";
import { Pagination } from "@/components/pagination";
import { RoomTable } from "./room-table";

export const metadata: Metadata = {
  title: "Ruangan",
};

const PAGE_SIZE = 10;

interface PageProps {
  searchParams: Promise<{
    branch_id?: string;
    is_active?: string;
    room_type?: string;
    page?: string;
  }>;
}

export default async function RoomsPage({ searchParams }: PageProps) {
  const { branch_id, is_active, room_type, page: pageParam } = await searchParams;
  const pageNum = Math.max(1, Number(pageParam) || 1);

  let rooms: Room[] = [];
  let totalCount = 0;
  let totalPages = 0;
  let currentPage = pageNum;
  let branches: Branch[] = [];
  let fetchError = false;

  const params = new URLSearchParams({ page: String(pageNum), limit: String(PAGE_SIZE) });
  if (branch_id) params.set("branch_id", branch_id);
  if (is_active) params.set("is_active", is_active);
  if (room_type) params.set("room_type", room_type);

  try {
    const [roomRes, branchRes] = await Promise.allSettled([
      apiFetch<RoomListResponse>(
        `/tenant/rooms?${params.toString()}`,
        {},
        { auth: true }
      ),
      apiFetch<BranchListResponse>("/tenant/branches?limit=200", {}, { auth: true }),
    ]);

    if (roomRes.status === "fulfilled") {
      rooms = roomRes.value.data;
      totalCount = roomRes.value.total_count;
      totalPages = roomRes.value.total_pages;
      currentPage = roomRes.value.page;
    } else {
      const err = roomRes.reason;
      if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
        await handleApiError(err);
      }
      fetchError = true;
    }

    if (branchRes.status === "fulfilled") {
      branches = branchRes.value.data;
    }
  } catch (err) {
    await handleApiError(err);
  }

  // Branch column: show when >1 branch and no single-branch filter active
  const showBranchColumn = branches.length > 1 && !branch_id;

  // Reorder column: only when a single branch is filtered AND all rooms fit on one page
  // (RM-5: sort_order is per-branch; reorder only makes sense within one branch)
  const isSinglePage = totalPages <= 1;
  const showReorderColumn = !!branch_id && isSinglePage;

  // Adaptive subtitle
  let subtitle = "Kelola data ruangan di semua cabang Anda";
  if (branches.length === 1) {
    subtitle = "Kelola data ruangan di cabang Anda";
  } else if (branches.length > 1 && branch_id) {
    const selected = branches.find((b) => b.id === branch_id);
    if (selected) subtitle = `Kelola data ruangan di cabang ${selected.name}`;
  }

  const filterActive = !!(branch_id || is_active || room_type);

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground sm:text-2xl">
            Ruangan
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">{subtitle}</p>
        </div>
        <Button asChild className="w-full sm:w-auto">
          <Link href="/master/rooms/new">
            <Plus size={16} aria-hidden="true" />
            Tambah Ruangan
          </Link>
        </Button>
      </div>

      {/* Filter bar */}
      <FilterBar isActive={filterActive} resetHref="/master/rooms">
        {branches.length > 1 && (
          <FilterSelect
            label="Cabang"
            name="branch_id"
            current={branch_id}
            options={[
              { value: "", label: "Semua" },
              ...branches.map((b) => ({ value: b.id, label: b.name })),
            ]}
          />
        )}
        <FilterSelect
          label="Status"
          name="is_active"
          current={is_active}
          options={[
            { value: "", label: "Semua" },
            { value: "true", label: "Aktif" },
            { value: "false", label: "Nonaktif" },
          ]}
        />
        <FilterSelect
          label="Tipe"
          name="room_type"
          current={room_type}
          options={[
            { value: "", label: "Semua" },
            { value: "single", label: "Single" },
            { value: "couple", label: "Couple" },
            { value: "group", label: "Grup" },
            { value: "vip", label: "VIP" },
          ]}
        />
      </FilterBar>

      {/* Error state */}
      {fetchError && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
        >
          <AlertCircle
            size={16}
            className="mt-0.5 shrink-0 text-red-600"
            aria-hidden="true"
          />
          <span>Gagal memuat data ruangan. Muat ulang halaman.</span>
        </div>
      )}

      {/* Table / Empty state */}
      <Card>
        <CardContent className="overflow-x-auto p-0">
          {!fetchError && rooms.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <DoorOpen
                size={40}
                className="text-muted-foreground/50"
                aria-hidden="true"
              />
              <div className="space-y-1">
                <p className="text-sm font-medium text-muted-foreground">
                  {is_active === "true"
                    ? "Tidak ada ruangan aktif saat ini."
                    : is_active === "false"
                      ? "Tidak ada ruangan nonaktif."
                      : room_type
                        ? `Tidak ada ruangan bertipe ${room_type}.`
                        : branch_id
                          ? "Belum ada ruangan di cabang ini."
                          : "Belum ada ruangan"}
                </p>
                {!is_active && !room_type && !branch_id && (
                  <p className="mx-auto max-w-xs text-xs text-muted-foreground/80">
                    Tambahkan ruangan untuk mulai mengatur alokasi tempat.
                  </p>
                )}
              </div>
              {!filterActive && (
                <Button asChild size="sm">
                  <Link href="/master/rooms/new">
                    <Plus size={14} aria-hidden="true" />
                    Tambah Ruangan
                  </Link>
                </Button>
              )}
            </div>
          ) : (
            !fetchError && (
              <RoomTable
                initialRooms={rooms}
                branches={branches}
                showBranchColumn={showBranchColumn}
                showReorderColumn={showReorderColumn}
                branchId={branch_id}
              />
            )
          )}
        </CardContent>
      </Card>

      {/* Pagination */}
      <Pagination
        pathname="/master/rooms"
        searchParams={{ branch_id, is_active, room_type }}
        page={currentPage}
        totalPages={totalPages}
        totalCount={totalCount}
        pageSize={PAGE_SIZE}
      />
    </div>
  );
}
