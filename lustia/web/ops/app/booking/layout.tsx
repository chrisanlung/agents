import { redirect } from "next/navigation";

import { apiFetch, ApiError } from "@/lib/api";
import { OpsHeader } from "@/components/ops-header";
import type { MembershipSummary } from "@/components/workspace-switcher";

interface MeResponse {
  user: { full_name: string; email: string };
  memberships: MembershipSummary[];
  tenant?: { name: string; slug: string };
}

interface BranchListResponse {
  data: { id: string; name: string }[];
}

export default async function BookingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  let me: MeResponse;
  let branchName: string | null = null;
  try {
    const [meRes, branchesRes] = await Promise.allSettled([
      apiFetch<MeResponse>("/auth/me", {}, { auth: true }),
      apiFetch<BranchListResponse>(
        "/tenant/branches?scope=mine&status=active&limit=200",
        {},
        { auth: true }
      ),
    ]);

    if (meRes.status === "rejected") {
      if (meRes.reason instanceof ApiError && meRes.reason.status === 401) {
        redirect("/login");
      }
      throw meRes.reason;
    }
    me = meRes.value;

    if (branchesRes.status === "fulfilled") {
      const branches = branchesRes.value.data;
      if (branches.length === 1) {
        branchName = branches[0].name;
      } else if (branches.length > 1) {
        branchName = `${branches.length} Cabang`;
      }
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  return (
    <div className="min-h-screen bg-amber-50">
      <OpsHeader
        fullName={me.user.full_name}
        email={me.user.email}
        tenant={me.tenant ?? null}
        branchName={branchName}
        memberships={me.memberships}
      />
      <main className="mx-auto max-w-6xl px-6 py-8">{children}</main>
    </div>
  );
}
