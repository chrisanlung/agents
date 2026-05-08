import { redirect } from "next/navigation";
import { Suspense } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { Plus, Users, Inbox } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import type { User, UserListResponse, Role, BranchListResponse } from "@/lib/types";
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
import { Pagination } from "@/components/pagination";
import { UserRowActions } from "./user-row-actions";
import { UserFilters } from "./user-filters";

const PAGE_SIZE = 10;

export const metadata: Metadata = {
  title: "Pengguna",
};

// Readable role name map
const ROLE_LABELS: Record<string, string> = {
  super_admin: "Super Admin",
  tenant_admin: "Admin Tenant",
  branch_admin: "Admin Cabang",
  therapist: "Terapis",
  finance: "Keuangan",
};

function formatDate(iso?: string | null): string {
  if (!iso) return "—";
  try {
    return new Intl.DateTimeFormat("id-ID", {
      day: "2-digit",
      month: "short",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(iso));
  } catch {
    return "—";
  }
}

function UserStatusBadge({ user }: { user: User }) {
  const now = new Date();
  const isLocked =
    user.locked_until != null && new Date(user.locked_until) > now;

  if (isLocked) {
    return <Badge variant="warning">Terkunci</Badge>;
  }
  if (!user.is_active) {
    return <Badge variant="muted">Nonaktif</Badge>;
  }
  return <Badge variant="success">Aktif</Badge>;
}

interface PageProps {
  searchParams: Promise<{
    page?: string;
    role_id?: string;
    branch_id?: string;
    is_active?: string;
  }>;
}

export default async function UsersPage({ searchParams }: PageProps) {
  const params = await searchParams;
  const pageNum = Math.max(1, Number(params.page) || 1);
  const roleId = params.role_id ?? "";
  const branchId = params.branch_id ?? "";
  const isActiveParam = params.is_active ?? "";

  const userParams = new URLSearchParams({
    page: String(pageNum),
    limit: String(PAGE_SIZE),
  });
  if (roleId) userParams.set("role_id", roleId);
  if (branchId) userParams.set("branch_id", branchId);
  if (isActiveParam === "true") userParams.set("is_active", "true");
  if (isActiveParam === "false") userParams.set("is_active", "false");

  let users: User[] = [];
  let totalCount = 0;
  let totalPages = 0;
  let currentPage = pageNum;
  let roles: Role[] = [];
  let branches: { id: string; name: string }[] = [];

  try {
    const [usersRes, rolesRes, branchesRes] = await Promise.allSettled([
      apiFetch<UserListResponse>(
        `/admin/users?${userParams.toString()}`,
        {},
        { auth: true }
      ),
      apiFetch<{ data: Role[] }>("/admin/roles", {}, { auth: true }),
      apiFetch<BranchListResponse>(
        "/tenant/branches?limit=200&status=active",
        {},
        { auth: true }
      ),
    ]);

    if (usersRes.status === "fulfilled") {
      users = usersRes.value.data;
      totalCount = usersRes.value.total_count;
      totalPages = usersRes.value.total_pages;
      currentPage = usersRes.value.page;
    } else if (
      usersRes.reason instanceof ApiError &&
      usersRes.reason.status === 401
    ) {
      redirect("/login");
    } else if (
      usersRes.reason instanceof ApiError &&
      usersRes.reason.status === 403
    ) {
      redirect("/dashboard");
    }

    if (rolesRes.status === "fulfilled") {
      roles = rolesRes.value.data.filter((r) => r.name !== "customer");
    }

    if (branchesRes.status === "fulfilled") {
      branches = branchesRes.value.data.map((b) => ({ id: b.id, name: b.name }));
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) redirect("/login");
    throw err;
  }

  const currentSearchParams = {
    role_id: roleId || undefined,
    branch_id: branchId || undefined,
    is_active: isActiveParam || undefined,
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Users size={22} className="text-primary" aria-hidden="true" />
          <h1 className="text-xl font-semibold text-foreground">Pengguna</h1>
        </div>
        <Button asChild>
          <Link href="/users/new">
            <Plus size={16} aria-hidden="true" />
            Tambah Pengguna
          </Link>
        </Button>
      </div>

      {/* Filter bar — client component so filters update URL reactively */}
      <Suspense fallback={null}>
        <UserFilters
          roles={roles}
          branches={branches}
          currentRoleId={roleId}
          currentBranchId={branchId}
          currentIsActive={isActiveParam}
        />
      </Suspense>

      <Card>
        <CardContent className="p-0">
          {users.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-3 py-16 text-center">
              <Inbox
                size={36}
                className="text-muted-foreground/40"
                aria-hidden="true"
              />
              <p className="text-sm text-muted-foreground">
                Belum ada pengguna. Tambahkan staff untuk mengelola booking dan
                operasional.
              </p>
              <Button asChild size="sm">
                <Link href="/users/new">
                  <Plus size={14} aria-hidden="true" />
                  Tambah Pengguna
                </Link>
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Nama</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead className="hidden md:table-cell">Username</TableHead>
                  <TableHead className="hidden sm:table-cell">Telepon</TableHead>
                  <TableHead className="hidden md:table-cell">Role</TableHead>
                  <TableHead className="hidden lg:table-cell">Login Terakhir</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Aksi</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((user) => (
                  <TableRow key={user.id}>
                    <TableCell className="font-medium">
                      {user.full_name}
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {user.email}
                    </TableCell>
                    <TableCell className="hidden md:table-cell text-sm">
                      {user.username ? (
                        <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
                          {user.username}
                        </code>
                      ) : (
                        <span className="text-muted-foreground/50">—</span>
                      )}
                    </TableCell>
                    <TableCell className="hidden sm:table-cell text-sm text-muted-foreground">
                      {user.phone ?? <span className="text-muted-foreground/50">—</span>}
                    </TableCell>
                    <TableCell className="hidden md:table-cell">
                      {user.role_names && user.role_names.length > 0 ? (
                        <div className="flex flex-wrap gap-1">
                          {user.role_names.map((r) => (
                            <Badge key={r} variant="secondary" className="text-xs">
                              {ROLE_LABELS[r] ?? r}
                            </Badge>
                          ))}
                        </div>
                      ) : (
                        <span className="text-muted-foreground/50 text-sm">—</span>
                      )}
                    </TableCell>
                    <TableCell className="hidden lg:table-cell text-xs text-muted-foreground">
                      {formatDate(user.last_login_at)}
                    </TableCell>
                    <TableCell>
                      <UserStatusBadge user={user} />
                    </TableCell>
                    <TableCell className="text-right">
                      <UserRowActions user={user} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Pagination
        pathname="/users"
        searchParams={Object.fromEntries(
          Object.entries(currentSearchParams).filter(([, v]) => v !== undefined)
        ) as Record<string, string>}
        page={currentPage}
        totalPages={totalPages}
        totalCount={totalCount}
        pageSize={PAGE_SIZE}
      />
    </div>
  );
}
