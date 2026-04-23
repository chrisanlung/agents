import { redirect } from "next/navigation";

import { apiFetch, ApiError } from "@/lib/api";
import { AppBackground } from "@/components/app-background";
import { AppHeader } from "@/components/app-header";
import { type MembershipSummary } from "@/components/workspace-switcher";

interface MeResponse {
  user: { full_name: string; email: string };
  memberships: MembershipSummary[];
  tenant?: { name: string; slug: string };
}

export default async function MasterLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  let me: MeResponse;
  try {
    me = await apiFetch<MeResponse>("/auth/me", {}, { auth: true });
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    throw err;
  }

  return (
    <AppBackground>
      <AppHeader
        fullName={me.user.full_name}
        email={me.user.email}
        activeNav="operasional"
        tenant={me.tenant ? { name: me.tenant.name, slug: me.tenant.slug } : null}
        memberships={me.memberships}
      />
      <main className="mx-auto max-w-5xl px-6 py-8">{children}</main>
    </AppBackground>
  );
}
