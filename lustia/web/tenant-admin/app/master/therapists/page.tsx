import { redirect } from "next/navigation";
import { Suspense } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { Plus, Inbox, AlertCircle, UserRound } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
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
import { TherapistRowActions } from "./therapist-row-actions";

export const metadata: Metadata = {
  title: "Terapis",
};

interface PageProps {
  searchParams: Promise<{ branch_id?: string; is_active?: string }>;
}

function formatDate(iso: string | null): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("id-ID", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  });
}

function TherapistAvatar({ name }: { name: string }) {
  const initials = name
    .split(" ")
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");
  return (
    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
      {initials || <UserRound size={14} aria-hidden="true" />}
    </div>
  );
}

export default async function TherapistsPage({ searchParams }: PageProps) {
  const { branch_id, is_active } = await searchParams;

  let therapists: Therapist[] = [];
  let branches: Branch[] = [];
  let fetchError = false;

  const params = new URLSearchParams({ limit: "50" });
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
    } else {
      const err = therapistRes.reason;
      if (err instanceof ApiError && err.status === 401) redirect("/login");
      fetchError = true;
    }

    if (branchRes.status === "fulfilled") {
      branches = branchRes.value.data;
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) redirect("/login");
    throw err;
  }

  const showBranchColumn = branches.length > 1;

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground">Terapis</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Kelola data terapis di semua cabang Anda
          </p>
        </div>
        <Button asChild>
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
        />
      </Suspense>

      {/* Error state */}
      {fetchError && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
        >
          <AlertCircle size={16} className="mt-0.5 shrink-0 text-red-600" aria-hidden="true" />
          <span>
            Gagal memuat data terapis. Coba lagi.
          </span>
        </div>
      )}

      {/* Table */}
      <Card>
        <CardContent className="p-0">
          {!fetchError && therapists.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <Inbox size={36} className="text-muted-foreground/40" aria-hidden="true" />
              <p className="text-sm text-muted-foreground">
                Belum ada terapis. Tambahkan terapis pertama untuk memulai.
              </p>
              <Button asChild size="sm">
                <Link href="/master/therapists/new">
                  <Plus size={14} aria-hidden="true" />
                  Tambah Terapis
                </Link>
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10"></TableHead>
                  <TableHead>Nama</TableHead>
                  {showBranchColumn && <TableHead>Cabang</TableHead>}
                  <TableHead>Status</TableHead>
                  <TableHead>Bergabung Sejak</TableHead>
                  <TableHead className="text-right">Aksi</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {therapists.map((t) => (
                  <TableRow key={t.id}>
                    <TableCell>
                      <TherapistAvatar name={t.full_name} />
                    </TableCell>
                    <TableCell className="font-medium">{t.full_name}</TableCell>
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
                    <TableCell className="text-right">
                      <TherapistRowActions therapist={t} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

// ─── Filter bar (client would need useRouter; keeping server-rendered links) ─

function TherapistFilters({
  branches,
  showBranchFilter,
  currentBranchId,
  currentStatus,
}: {
  branches: Branch[];
  showBranchFilter: boolean;
  currentBranchId?: string;
  currentStatus?: string;
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

  return (
    <div className="flex flex-wrap items-center gap-2">
      {showBranchFilter && (
        <div className="flex items-center gap-1.5">
          <span className="text-sm text-muted-foreground">Cabang:</span>
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
          {branches.map((b) => (
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
        </div>
      )}

      <div className="flex items-center gap-1.5">
        <span className="text-sm text-muted-foreground">Status:</span>
        {[
          { label: "Semua", value: undefined },
          { label: "Aktif", value: "true" },
          { label: "Nonaktif", value: "false" },
        ].map(({ label, value }) => (
          <Link
            key={label}
            href={buildUrl({ is_active: value })}
            className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
              currentStatus === value || (!currentStatus && value === undefined)
                ? "bg-primary text-primary-foreground"
                : "bg-muted text-muted-foreground hover:bg-muted/80"
            }`}
          >
            {label}
          </Link>
        ))}
      </div>
    </div>
  );
}
