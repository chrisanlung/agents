import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import type { BranchListResponse, Branch } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/card";
import { RoomForm } from "../room-form";

export const metadata: Metadata = {
  title: "Tambah Ruangan",
};

export default async function NewRoomPage() {
  let branches: Branch[] = [];

  try {
    const res = await apiFetch<BranchListResponse>(
      "/tenant/branches?limit=200",
      {},
      { auth: true }
    );
    branches = res.data;
  } catch (err) {
    if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
      await handleApiError(err);
    }
    // Non-fatal: form can still render; backend will reject foreign branches
  }

  return (
    <div className="space-y-6">
      <Link
        href="/master/rooms"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke Daftar Ruangan
      </Link>

      <div>
        <h1 className="text-xl font-semibold text-foreground">
          Tambah Ruangan
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Isi detail ruangan baru
        </p>
      </div>

      <Card>
        <CardContent className="p-6">
          <RoomForm branches={branches} />
        </CardContent>
      </Card>
    </div>
  );
}
