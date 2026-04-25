import { Suspense } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { Plus, AlertCircle, UserRound } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import type {
  TherapistListResponse,
  Therapist,
  BranchListResponse,
  Branch,
} from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { FilterSelect } from "@/components/filter-select";
import { FilterBar } from "@/components/filter-bar";
import { Pagination } from "@/components/pagination";
import { BranchOverflowSelect } from "./branch-overflow-select";
import { TherapistRowActions } from "./therapist-row-actions";

const PAGE_SIZE = 10;

export const metadata: Metadata = {
  title: "Terapis",
};

interface PageProps {
  searchParams: Promise<{
    branch_id?: string;
    is_active?: string;
    page?: string;
  }>;
}

function formatDate(iso: string | null): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  });
}

// TEP-5 — avatar shows photo when available, falls back to initials circle.
function TherapistAvatar({ name, photoUrl }: { name: string; photoUrl: string | null }) {
  if (photoUrl) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={photoUrl}
        alt={`Foto ${name}`}
        className="h-8 w-8 shrink-0 rounded-full object-cover"
      />
    );
  }
  const initials = name
    .split(" ")
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");
  return (
    <div
      aria-label={name}
      className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary"
    >
      {initials || <UserRound size={14} aria-hidden="true" />}
    </div>
  );
}

/**
 * TEP-7 — detect placeholder / migration-default data.
 * height_cm=160, weight_kg=60, build="sedang", photo_url=null simultaneously.
 */
function isPlaceholderProfile(t: {
  height_cm: number;
  weight_kg: number;
  build: string;
  photo_url: string | null;
}): boolean {
  return (
    t.height_cm === 160 &&
    t.weight_kg === 60 &&
    t.build === "sedang" &&
    t.photo_url === null
  );
}

export default async function TherapistsPage({ searchParams }: PageProps) {
  const { branch_id, is_active, page: pageParam } = await searchParams;
  const pageNum = Math.max(1, Number(pageParam) || 1);

  let therapists: Therapist[] = [];
  let totalCount = 0;
  let totalPages = 0;
  let currentPage = pageNum;
  let branches: Branch[] = [];
  let fetchError = false;

  const params = new URLSearchParams({ page: String(pageNum), limit: String(PAGE_SIZE) });
  if (branch_id) params.set("branch_id", branch_id);
  if (is_active) params.set("is_active", is_active);

  try {
    const [therapistRes, branchRes] = await Promise.allSettled([
      apiFetch<TherapistListResponse>(
        `/tenant/therapists?${params.toString()}`,
        {},
        { auth: true }
      ),
      apiFetch<BranchListResponse>("/tenant/branches?limit=200", {}, { auth: true }),
    ]);

    if (therapistRes.status === "fulfilled") {
      therapists = therapistRes.value.data;
      totalCount = therapistRes.value.total_count;
      totalPages = therapistRes.value.total_pages;
      currentPage = therapistRes.value.page;
    } else {
      const err = therapistRes.reason;
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

  const showBranchColumn = branches.length > 1;

  // Subtitle copy adapts to what the caller actually sees:
  //   - 1 branch (starter tenant or branch_admin): "di cabang Anda"
  //   - >1 branch + filter: "di cabang <Name>"
  //   - >1 branch + no filter: "di semua cabang"
  let subtitle = "Kelola data terapis";
  if (branches.length === 1) {
    subtitle = "Kelola data terapis di cabang Anda";
  } else if (branches.length > 1) {
    const selected = branch_id && branches.find((b) => b.id === branch_id);
    subtitle = selected
      ? `Kelola data terapis di cabang ${selected.name}`
      : "Kelola data terapis di semua cabang Anda";
  }

  const filterActive = !!(branch_id || is_active);

  return (
    <div className="space-y-6">
      {/* Page header — collapses vertically on narrow viewports so the CTA
           doesn't clip out of view on mobile/tablet (UX review 2026-04-24). */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground sm:text-2xl">Terapis</h1>
          <p className="mt-1 text-sm text-muted-foreground">{subtitle}</p>
        </div>
        <Button asChild className="w-full sm:w-auto">
          <Link href="/master/therapists/new">
            <Plus size={16} aria-hidden="true" />
            Tambah Terapis
          </Link>
        </Button>
      </div>

      {/* Filter bar */}
      <Suspense fallback={null}>
        <TherapistFilters
          branches={branches}
          showBranchFilter={showBranchColumn}
          currentBranchId={branch_id}
          currentStatus={is_active}
          filterActive={filterActive}
        />
      </Suspense>

      {/* Error state */}
      {fetchError && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
        >
          <AlertCircle size={16} className="mt-0.5 shrink-0 text-red-600" aria-hidden="true" />
          <span>Gagal memuat data terapis. Muat ulang halaman.</span>
        </div>
      )}

      {/* Table */}
      <Card>
        <CardContent className="overflow-x-auto p-0">
          {!fetchError && therapists.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <UserRound size={40} className="text-muted-foreground/50" aria-hidden="true" />
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">
                  {is_active === "false"
                    ? "Tidak ada terapis nonaktif."
                    : is_active === "true"
                      ? "Tidak ada terapis aktif saat ini."
                      : branch_id
                        ? "Belum ada terapis di cabang ini."
                        : "Belum ada terapis."}
                </p>
                {!is_active && !branch_id && (
                  <p className="text-xs text-muted-foreground/80">
                    Tambahkan terapis untuk mulai mengatur jadwal dan layanan.
                  </p>
                )}
              </div>
              {!is_active && (
                <Button asChild size="sm">
                  <Link href="/master/therapists/new">
                    <Plus size={14} aria-hidden="true" />
                    Tambah Terapis
                  </Link>
                </Button>
              )}
            </div>
          ) : (
            <Table className="min-w-[560px]">
              <TableHeader className="bg-muted/30">
                <TableRow>
                  <TableHead>Nama</TableHead>
                  {showBranchColumn && <TableHead>Cabang</TableHead>}
                  <TableHead>Status</TableHead>
                  <TableHead>Bergabung</TableHead>
                  <TableHead>Aksi</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {therapists.map((t) => (
                  <TableRow key={t.id} className="h-14">
                    <TableCell>
                      <div className="flex items-center gap-2.5">
                        <TherapistAvatar name={t.full_name} photoUrl={t.photo_url} />
                        <span className="font-medium">{t.full_name}</span>
                        {/* TEP-7 — amber dot for incomplete profile */}
                        {isPlaceholderProfile(t) && (
                          <span
                            aria-label="Profil belum dilengkapi"
                            title="Profil belum dilengkapi"
                            className="inline-block h-2 w-2 shrink-0 rounded-full bg-amber-400"
                          />
                        )}
                      </div>
                    </TableCell>
                    {showBranchColumn && (
                      <TableCell>
                        {branches.find((b) => b.id === t.branch_id)?.name ?? (
                          <span className="text-muted-foreground">—</span>
                        )}
                      </TableCell>
                    )}
                    <TableCell>
                      <Badge variant={t.is_active ? "success" : "muted"}>
                        {t.is_active ? "Aktif" : "Nonaktif"}
                      </Badge>
                    </TableCell>
                    <TableCell className="tabular-nums text-sm text-muted-foreground">
                      {formatDate(t.joined_at)}
                    </TableCell>
                    <TableCell>
                      <TherapistRowActions therapist={t} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Pagination */}
      <Pagination
        pathname="/master/therapists"
        searchParams={{ branch_id, is_active }}
        page={currentPage}
        totalPages={totalPages}
        totalCount={totalCount}
        pageSize={PAGE_SIZE}
      />
    </div>
  );
}

// ─── Filter bar (client would need useRouter; keeping server-rendered links) ─

function TherapistFilters({
  branches,
  showBranchFilter,
  currentBranchId,
  currentStatus,
  filterActive,
}: {
  branches: Branch[];
  showBranchFilter: boolean;
  currentBranchId?: string;
  currentStatus?: string;
  filterActive: boolean;
}) {
  function buildUrl(patch: Record<string, string | undefined>) {
    const p = new URLSearchParams();
    const merged = {
      branch_id: currentBranchId,
      is_active: currentStatus,
      ...patch,
    };
    Object.entries(merged).forEach(([k, v]) => {
      if (v !== undefined && v !== "") p.set(k, v);
    });
    const qs = p.toString();
    return `/master/therapists${qs ? `?${qs}` : ""}`;
  }

  // Pill overflow rule: show at most 3 pills inline. When there are more
  // branches, the overflow collapses into a native <select> (server-rendered,
  // no client JS needed) so the filter bar stays one line on growth/enterprise
  // tenants with 5–20 branches.
  const PILL_CAP = 3;
  const pillBranches = branches.slice(0, PILL_CAP);
  const overflowBranches = branches.slice(PILL_CAP);
  const activePillIDs = new Set(pillBranches.map((b) => b.id));
  const overflowActive = !!currentBranchId && !activePillIDs.has(currentBranchId);
  const overflowSelected = overflowActive
    ? branches.find((b) => b.id === currentBranchId)
    : undefined;

  return (
    <FilterBar isActive={filterActive} resetHref="/master/therapists">
      {showBranchFilter && (
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="text-sm text-muted-foreground">Cabang</span>
          <Link
            href={buildUrl({ branch_id: undefined })}
            className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
              !currentBranchId
                ? "bg-primary text-primary-foreground"
                : "bg-muted text-muted-foreground hover:bg-muted/80"
            }`}
          >
            Semua
          </Link>
          {pillBranches.map((b) => (
            <Link
              key={b.id}
              href={buildUrl({ branch_id: b.id })}
              className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
                currentBranchId === b.id
                  ? "bg-primary text-primary-foreground"
                  : "bg-muted text-muted-foreground hover:bg-muted/80"
              }`}
            >
              {b.name}
            </Link>
          ))}
          {overflowBranches.length > 0 && (
            <BranchOverflowSelect
              branches={overflowBranches}
              currentBranchId={currentBranchId}
              currentStatus={currentStatus}
              label={
                overflowSelected
                  ? overflowSelected.name
                  : `Lainnya (${overflowBranches.length})`
              }
            />
          )}
        </div>
      )}

      <FilterSelect
        label="Status"
        name="is_active"
        current={currentStatus}
        options={[
          { value: "", label: "Semua" },
          { value: "true", label: "Aktif" },
          { value: "false", label: "Nonaktif" },
        ]}
      />
    </FilterBar>
  );
}
