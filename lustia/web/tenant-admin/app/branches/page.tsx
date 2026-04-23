import { redirect } from "next/navigation";
import { Suspense } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { Plus, MapPin, Inbox, AlertCircle } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import type { BranchListResponse, Branch, OnboardingState } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { BranchRowActions } from "./branch-row-actions";
import { BranchSuccessToast } from "./branch-success-toast";

export const metadata: Metadata = {
  title: "Manajemen Cabang",
};

const STATUS_LABELS: Record<string, string> = {
  active: "Aktif",
  inactive: "Nonaktif",
};

const STATUS_VARIANTS: Record<
  string,
  "success" | "muted" | "secondary"
> = {
  active: "success",
  inactive: "muted",
};

export default async function BranchesPage() {
  let branches: Branch[] = [];
  let onboardingState: OnboardingState | null = null;

  try {
    const [branchRes, onboardingRes] = await Promise.allSettled([
      apiFetch<BranchListResponse>(
        "/tenant/branches?limit=50",
        {},
        { auth: true }
      ),
      apiFetch<OnboardingState>(
        "/tenant/onboarding-state",
        {},
        { auth: true }
      ),
    ]);

    if (branchRes.status === "fulfilled") {
      branches = branchRes.value.data;
    } else if (
      branchRes.reason instanceof ApiError &&
      branchRes.reason.status === 401
    ) {
      redirect("/login");
    }

    if (onboardingRes.status === "fulfilled") {
      onboardingState = onboardingRes.value;
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  const maxBranches = onboardingState?.max_branches ?? null;
  const activeBranchCount = branches.filter((b) => b.status === "active").length;
  const atLimit =
    maxBranches !== null &&
    maxBranches !== 999 &&
    branches.length >= maxBranches;

  return (
    <div className="space-y-6">
      {/* Success toast (reads query param — wrapped in Suspense) */}
      <Suspense fallback={null}>
        <BranchSuccessToast />
      </Suspense>

      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <MapPin size={22} className="text-primary" aria-hidden="true" />
          <div>
            <h1 className="text-xl font-semibold text-foreground">
              Manajemen Cabang
            </h1>
            {maxBranches !== null && (
              <p className="text-sm text-muted-foreground">
                {activeBranchCount} /{" "}
                {maxBranches === 999 ? "tidak terbatas" : maxBranches}{" "}
                cabang digunakan
                {/* Cross-agent flag: max_branches must be present on onboarding-state response */}
              </p>
            )}
          </div>
        </div>
        {atLimit ? (
          <span
            title={`Cabang sudah mencapai batas paket (${maxBranches}). Tingkatkan paket untuk menambah cabang.`}
            className="inline-flex"
          >
            <Button
              disabled
              aria-disabled="true"
              className="pointer-events-none cursor-not-allowed"
            >
              <AlertCircle size={16} aria-hidden="true" />
              Cabang Baru
            </Button>
          </span>
        ) : (
          <Button asChild>
            <Link href="/branches/new">
              <Plus size={16} aria-hidden="true" />
              Cabang Baru
            </Link>
          </Button>
        )}
      </div>

      {atLimit && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800"
        >
          <AlertCircle
            size={16}
            className="mt-0.5 shrink-0 text-amber-600"
            aria-hidden="true"
          />
          <span>
            Batas cabang tercapai ({maxBranches}). Tingkatkan paket untuk
            menambah lebih banyak cabang.
          </span>
        </div>
      )}

      <Card>
        <CardContent className="p-0">
          {branches.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <Inbox size={36} className="text-muted-foreground/40" aria-hidden="true" />
              <p className="text-sm text-muted-foreground">
                Belum ada cabang. Tambah cabang pertama Anda.
              </p>
              <Button asChild size="sm">
                <Link href="/branches/new">
                  <Plus size={14} aria-hidden="true" />
                  Tambah Cabang
                </Link>
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Nama</TableHead>
                  <TableHead>Kode</TableHead>
                  <TableHead>Kota</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Aksi</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {branches.map((branch) => (
                  <TableRow key={branch.id}>
                    <TableCell className="font-medium">
                      {branch.name}
                    </TableCell>
                    <TableCell>
                      <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
                        {branch.code}
                      </code>
                    </TableCell>
                    <TableCell>
                      {branch.city ?? (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={STATUS_VARIANTS[branch.status] ?? "secondary"}
                      >
                        {STATUS_LABELS[branch.status] ?? branch.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <BranchRowActions branch={branch} />
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
