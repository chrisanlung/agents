/**
 * API response types for Phase 3 endpoints (ADR 0008 §2.2).
 * These match the Go backend response shapes exactly — do not add fields
 * that aren't in the ADR without coordinating with go-expert.
 */

// ─── Tenant ──────────────────────────────────────────────────────────────────

export type TenantStatus =
  | "pending_approval"
  | "active"
  | "suspended"
  | "deactivated";

export type TenantPackage = "starter" | "growth" | "enterprise";

export interface Tenant {
  id: string;
  name: string;
  slug: string;
  status: TenantStatus;
  package: TenantPackage;
  max_branches: number;
  contact_email: string;
  contact_name: string;
  approved_at: string | null;
  approved_by: string | null;
  rejected_at: string | null;
  rejection_reason: string | null;
  created_at: string;
  membership_count: number;
  branch_count: number;
}

export interface TenantListResponse {
  data: Tenant[];
  page: number;
  limit: number;
  total_count: number;
  total_pages: number;
}

// ─── Tenant Registration ─────────────────────────────────────────────────────

export type RegistrationStatus = "pending" | "approved" | "rejected";

export interface TenantRegistration {
  id: string;
  company_name: string;
  requested_slug: string;
  package: TenantPackage;
  contact_name: string;
  contact_email: string;
  contact_phone: string | null;
  status: RegistrationStatus;
  approved_tenant_id: string | null;
  approved_user_id: string | null;
  approved_at: string | null;
  approved_by: string | null;
  rejected_at: string | null;
  rejected_by: string | null;
  rejection_reason: string | null;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface RegistrationListResponse {
  data: TenantRegistration[];
  page: number;
  limit: number;
  total_count: number;
  total_pages: number;
}

// ─── Approve response (ADR 0008 §2.2.4) ─────────────────────────────────────

export interface ApproveRegistrationResponse {
  tenant: Tenant;
  tenant_admin: {
    user_id: string;
    email: string;
    /** Returned ONCE on 200. Must be a non-empty string — validated at runtime. */
    temporary_password: string;
  };
  registration: TenantRegistration;
}
