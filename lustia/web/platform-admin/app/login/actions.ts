"use server";

import { redirect } from "next/navigation";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import { setSessionCookies } from "@/lib/session";
import { decodeJWTPayload, type JWTClaims } from "@/lib/jwt";

const loginSchema = z.object({
  identifier: z
    .string()
    .min(3, "Masukkan email atau username yang valid")
    .max(320, "Masukkan email atau username yang valid")
    .refine(
      (v) =>
        /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v) ||
        /^[a-zA-Z0-9._]{3,50}$/.test(v),
      "Masukkan email atau username yang valid"
    ),
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
 * Server Action: login for the platform-admin portal.
 *
 * Decision tree (ADR 0007 §2.4):
 *   scope=platform + roles includes super_admin → set cookies, redirect /dashboard
 *   scope=tenant / scope=user → role-mismatch toast, no cookie
 *   scope=platform without super_admin → role-mismatch toast, no cookie
 */
export async function loginAction(
  _prevState: LoginFormState,
  formData: FormData
): Promise<LoginFormState> {
  const raw = {
    identifier: formData.get("identifier"),
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
        identifier: parsed.data.identifier,
        password: parsed.data.password,
      }),
    });
  } catch (err) {
    if (err instanceof ApiError) {
      return mapApiErrorToState(err);
    }
    return { status: "error", toast: "Layanan tidak tersedia. Silakan coba lagi." };
  }

  // scope=tenant or scope=user → this portal is platform-only, reject immediately.
  if (loginResponse.scope !== "platform") {
    return {
      status: "error",
      toast: "Portal ini hanya untuk admin platform.",
    };
  }

  // scope=platform: decode JWT to confirm super_admin role.
  let claims: JWTClaims;
  try {
    claims = decodeJWTPayload<JWTClaims>(loginResponse.access_token);
  } catch {
    return { status: "error", toast: "Layanan tidak tersedia. Silakan coba lagi." };
  }

  if (!claims.roles?.includes("super_admin")) {
    return {
      status: "error",
      toast: "Portal ini hanya untuk admin platform.",
    };
  }

  // All checks passed — establish session.
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
    // Deliberately generic — anti-enumeration (SECURITY.md §3.5)
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
