"use server";

import { redirect } from "next/navigation";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import { setSessionCookies } from "@/lib/session";

const loginSchema = z.object({
  email: z.string().email("Masukkan email yang valid"),
  password: z.string().min(1, "Kata sandi wajib diisi"),
});

export type LoginFormState =
  | { status: "idle" }
  | { status: "error"; fieldErrors?: Record<string, string[]>; toast?: string };

/** ADR 0007 §2.3 — login response shape */
interface LoginResponse {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_at: string;
  scope: "platform" | "tenant" | "user";
  memberships: MembershipSummary[];
  active_membership_id: string | null;
}

export interface MembershipSummary {
  tenant_id: string;
  tenant_name: string;
  tenant_slug: string;
  roles: string[];
  branches: string[];
  status: string;
}

/**
 * Server Action: login for the tenant-admin portal.
 *
 * Decision tree (ADR 0007 §2.4):
 *   scope=tenant + roles includes tenant_admin → set cookies, redirect /dashboard
 *   scope=user → set cookies (user-only token), redirect /select-tenant
 *     (role check deferred until tenant is selected in selectTenantAction)
 *   scope=platform → role-mismatch toast, no cookie
 */
export async function loginAction(
  _prevState: LoginFormState,
  formData: FormData
): Promise<LoginFormState> {
  const raw = {
    email: formData.get("email"),
    password: formData.get("password"),
  };

  const parsed = loginSchema.safeParse(raw);
  if (!parsed.success) {
    const fieldErrors: Record<string, string[]> = {};
    for (const [field, errors] of Object.entries(
      parsed.error.flatten().fieldErrors
    )) {
      fieldErrors[field] = errors ?? [];
    }
    return { status: "error", fieldErrors };
  }

  let loginResponse: LoginResponse;
  try {
    loginResponse = await apiFetch<LoginResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify({
        email: parsed.data.email,
        password: parsed.data.password,
      }),
    });
  } catch (err) {
    if (err instanceof ApiError) {
      return mapApiErrorToState(err);
    }
    return { status: "error", toast: "Layanan tidak tersedia. Silakan coba lagi." };
  }

  // scope=platform → super admin portal, reject here.
  if (loginResponse.scope === "platform") {
    return {
      status: "error",
      toast: "Portal ini hanya untuk admin tenant.",
    };
  }

  // scope=user → multiple memberships, store user-scoped token and redirect to picker.
  if (loginResponse.scope === "user") {
    await setSessionCookies(loginResponse.access_token, loginResponse.refresh_token);
    redirect("/select-tenant");
  }

  // scope=tenant → single membership, check for tenant_admin role before setting cookies.
  const activeMembership = loginResponse.memberships[0];
  if (!activeMembership?.roles.includes("tenant_admin")) {
    return {
      status: "error",
      toast: "Portal ini hanya untuk admin tenant.",
    };
  }

  await setSessionCookies(loginResponse.access_token, loginResponse.refresh_token);
  redirect("/dashboard");
}

function mapApiErrorToState(err: ApiError): LoginFormState {
  const credentialErrorCodes = [
    "INVALID_CREDENTIALS",
    "ACCOUNT_LOCKED",
    "ACCOUNT_INACTIVE",
    "TENANT_INACTIVE",
    "TENANT_NOT_FOUND",
    "VALIDATION",
  ];

  if (credentialErrorCodes.includes(err.code)) {
    return { status: "error", toast: "Email atau kata sandi salah." };
  }

  if (err.code === "RATE_LIMITED") {
    return {
      status: "error",
      toast: "Terlalu banyak percobaan. Mohon tunggu sebentar.",
    };
  }

  if (err.status >= 500) {
    return { status: "error", toast: "Layanan tidak tersedia. Silakan coba lagi." };
  }

  return { status: "error", toast: "Email atau kata sandi salah." };
}
