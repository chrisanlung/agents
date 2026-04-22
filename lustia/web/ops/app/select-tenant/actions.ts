"use server";

import { redirect } from "next/navigation";

import { apiFetch, ApiError } from "@/lib/api";
import { setSessionCookies, clearSessionCookies } from "@/lib/session";

/** ADR 0007 §2.3 — /auth/select-tenant response */
interface SelectTenantResponse {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_at: string;
  scope: "tenant";
  active_membership_id: string;
  membership: {
    tenant_id: string;
    tenant_name: string;
    tenant_slug: string;
    roles: string[];
    branches: string[];
    status: string;
  };
}

// Ops portal accepts any of these roles.
const OPS_ROLES = ["branch_admin", "finance", "therapist"] as const;

/**
 * Server Action: select a tenant workspace for a user-scoped session.
 *
 * Called from /select-tenant page (and from the workspace switcher on dashboard).
 *
 * After the backend returns a new tenant-scoped token pair we re-run the
 * per-portal role check (any ops role required). On failure we clear cookies
 * and redirect to /login with an error query param so the login page can
 * show a toast.
 *
 * ADR 0007 §2.4 — role check happens AFTER tenant selection.
 */
export async function selectTenantAction(tenantId: string): Promise<void> {
  let response: SelectTenantResponse | null = null;

  try {
    response = await apiFetch<SelectTenantResponse>("/auth/select-tenant", {
      method: "POST",
      body: JSON.stringify({ tenant_id: tenantId }),
    }, { auth: true });
  } catch (err) {
    if (err instanceof ApiError) {
      // 403 means the tenant_id didn't match any membership, or tenant inactive.
      await clearSessionCookies();
      redirect("/login?err=role_mismatch");
    }
    // Network / unexpected error — also clear and redirect.
    await clearSessionCookies();
    redirect("/login?err=role_mismatch");
  }

  if (!response) {
    // Should not happen — caught above — but satisfies TS definite-assignment.
    await clearSessionCookies();
    redirect("/login?err=role_mismatch");
  }

  // Per-portal role check on the new membership.
  const hasOpsRole = OPS_ROLES.some((role) =>
    response.membership.roles.includes(role)
  );
  if (!hasOpsRole) {
    await clearSessionCookies();
    redirect("/login?err=role_mismatch");
  }

  // Overwrite cookies with the new tenant-scoped tokens.
  await setSessionCookies(response.access_token, response.refresh_token);
  redirect("/dashboard");
}
