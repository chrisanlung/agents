import { notFound, redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import type { User, Role, BranchListResponse } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { UserForm } from "../user-form";
import { UserDetailActions } from "../user-detail-actions";

export const metadata: Metadata = {
  title: "Edit Pengguna",
};

interface Props {
  params: Promise<{ id: string }>;
}

function formatDate(iso?: string | null): string {
  if (!iso) return "—";
  try {
    return new Intl.DateTimeFormat("id-ID", {
      day: "2-digit",
      month: "long",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(iso));
  } catch {
    return "—";
  }
}

export default async function EditUserPage({ params }: Props) {
  const { id } = await params;

  let user: User;
  let roles: Role[] = [];
  let branches: { id: string; name: string }[] = [];

  try {
    const [userRes, rolesRes, branchesRes] = await Promise.allSettled([
      apiFetch<User>(`/admin/users/${id}`, {}, { auth: true }),
      apiFetch<{ data: Role[] }>("/admin/roles", {}, { auth: true }),
      apiFetch<BranchListResponse>(
        "/tenant/branches?limit=200&status=active",
        {},
        { auth: true }
      ),
    ]);

    if (userRes.status === "rejected") {
      if (userRes.reason instanceof ApiError) {
        if (userRes.reason.status === 401) redirect("/login");
        if (userRes.reason.status === 403) redirect("/dashboard");
        if (userRes.reason.status === 404) notFound();
      }
      throw userRes.reason;
    }

    user = userRes.value;

    if (rolesRes.status === "fulfilled") {
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

  const now = new Date();
  const isLocked =
    user.locked_until != null && new Date(user.locked_until) > now;

  return (
    <div className="space-y-6">
      <Link
        href="/users"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke daftar pengguna
      </Link>

      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-foreground">
            {user.full_name}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">{user.email}</p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {isLocked && <Badge variant="warning">Terkunci</Badge>}
          <Badge variant={user.is_active ? "success" : "muted"}>
            {user.is_active ? "Aktif" : "Nonaktif"}
          </Badge>
        </div>
      </div>

      {/* Detail info panel */}
      <Card>
        <CardHeader className="pb-2">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
            Informasi Akun
          </h2>
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-4 text-sm sm:grid-cols-3">
          <div>
            <p className="text-xs text-muted-foreground">Login Terakhir</p>
            <p className="mt-0.5 font-medium">{formatDate(user.last_login_at)}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Dibuat</p>
            <p className="mt-0.5 font-medium">{formatDate(user.created_at)}</p>
          </div>
          {isLocked && (
            <div>
              <p className="text-xs text-muted-foreground">Terkunci Hingga</p>
              <p className="mt-0.5 font-medium text-amber-700">
                {formatDate(user.locked_until)}
              </p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Danger-zone actions (unlock / toggle status) */}
      {isLocked && (
        <UserDetailActions userId={user.id} userName={user.full_name} isLocked={isLocked} />
      )}

      {/* Edit form */}
      <Card>
        <CardContent className="p-6">
          <UserForm user={user} roles={roles} branches={branches} />
        </CardContent>
      </Card>
    </div>
  );
}
