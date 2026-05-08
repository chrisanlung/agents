import { redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import type { Role, BranchListResponse } from "@/lib/types";
import { Card, CardContent } from "@/components/ui/card";
import { UserForm } from "../user-form";

export const metadata: Metadata = {
  title: "Tambah Pengguna",
};

export default async function NewUserPage() {
  let roles: Role[] = [];
  let branches: { id: string; name: string }[] = [];

  try {
    const [rolesRes, branchesRes] = await Promise.allSettled([
      apiFetch<{ data: Role[] }>("/admin/roles", {}, { auth: true }),
      apiFetch<BranchListResponse>(
        "/tenant/branches?limit=200&status=active",
        {},
        { auth: true }
      ),
    ]);

    if (rolesRes.status === "rejected") {
      if (
        rolesRes.reason instanceof ApiError &&
        rolesRes.reason.status === 401
      ) {
        redirect("/login");
      }
    } else {
      roles = rolesRes.value.data.filter((r) => r.name !== "customer");
    }

    if (branchesRes.status === "fulfilled") {
      branches = branchesRes.value.data.map((b) => ({
        id: b.id,
        name: b.name,
      }));
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) redirect("/login");
    throw err;
  }

  return (
    <div className="space-y-6">
      <Link
        href="/users"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke daftar pengguna
      </Link>

      <div>
        <h1 className="text-xl font-semibold text-foreground">
          Tambah Pengguna
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Buat akun staff baru. Password awal akan ditampilkan sekali setelah
          pengguna berhasil dibuat.
        </p>
      </div>

      <Card>
        <CardContent className="p-6">
          <UserForm roles={roles} branches={branches} />
        </CardContent>
      </Card>
    </div>
  );
}
