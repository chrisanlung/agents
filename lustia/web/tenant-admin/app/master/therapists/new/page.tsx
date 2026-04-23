import { redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import type { BranchListResponse, Branch } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/card";
import { TherapistForm } from "../therapist-form";

export const metadata: Metadata = {
  title: "Tambah Terapis",
};

export default async function NewTherapistPage() {
  let branches: Branch[] = [];

  try {
    const res = await apiFetch<BranchListResponse>(
      "/tenant/branches?limit=200",
      {},
      { auth: true }
    );
    branches = res.data;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) redirect("/login");
    // Non-fatal — branch selector just won't have options
  }

  return (
    <div className="space-y-6">
      <Link
        href="/master/therapists"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke Daftar Terapis
      </Link>

      <div>
        <h1 className="text-xl font-semibold text-foreground">Tambah Terapis</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Isi data dasar terapis baru
        </p>
      </div>

      <Card>
        <CardContent className="p-6">
          <TherapistForm
            branches={branches}
            showBranchSelector={branches.length > 0}
          />
        </CardContent>
      </Card>
    </div>
  );
}
