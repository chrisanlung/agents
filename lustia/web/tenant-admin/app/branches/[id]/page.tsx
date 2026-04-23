import { notFound, redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import type { Branch } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/card";
import { BranchForm } from "../branch-form";

export const metadata: Metadata = {
  title: "Edit Cabang",
};

interface Props {
  params: Promise<{ id: string }>;
}

export default async function EditBranchPage({ params }: Props) {
  const { id } = await params;

  let branch: Branch;
  try {
    branch = await apiFetch<Branch>(
      `/tenant/branches/${id}`,
      {},
      { auth: true }
    );
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.status === 401) redirect("/login");
      if (err.status === 404) notFound();
    }
    throw err;
  }

  return (
    <div className="space-y-6">
      <Link
        href="/branches"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke daftar cabang
      </Link>

      <div>
        <h1 className="text-xl font-semibold text-foreground">
          Edit Cabang — {branch.name}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Perbarui informasi cabang. Kode cabang tidak dapat diubah setelah
          dibuat.
        </p>
      </div>

      <Card>
        <CardContent className="p-6">
          <BranchForm branch={branch} />
        </CardContent>
      </Card>
    </div>
  );
}
