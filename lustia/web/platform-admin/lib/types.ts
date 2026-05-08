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

// ─── Phase 6 — Finance / Payout (ADR 0015) ───────────────────────────────────

export type DisbursementStatus =
  | "pending"
  | "processing"
  | "transferred"
  | "failed"
  | "cancelled";

export type PaymentTransactionStatus =
  | "awaiting"
  | "paid"
  | "settled"
  | "disbursed"
  | "failed"
  | "expired"
  | "voided";

/**
 * GET /api/v1/admin/settlement-batches/summary response.
 * API_CONTRACT.md §15.4 — total_volume_idr is the KPI value for
 * "Volume Disetel Minggu Ini" on the platform-admin dashboard.
 */
export interface SettlementBatchSummary {
  from: string;
  to: string;
  batch_count: number;
  total_volume_idr: number;
  total_platform_fee_idr: number;
  total_payout_idr: number;
}

/** GET /api/v1/admin/settlement-batches list item. */
export interface SettlementBatch {
  id: string;
  provider: string;
  settled_at: string;
  total_amount_idr: number;
  transaction_count: number;
  created_at: string;
  created_by_name: string | null;
  mismatch_count: number;
}

export interface SettlementBatchListResponse {
  data: SettlementBatch[];
  total: number;
  page: number;
  total_pages: number;
}

export interface MismatchItem {
  provider_reference: string;
  amount_idr: number;
  issue: "not_in_lustia" | "not_in_provider";
}

export interface SettlementBatchDetail extends SettlementBatch {
  mismatches: MismatchItem[];
  transactions: Array<{
    id: string;
    provider_reference: string;
    tenant_name: string;
    received_amount_idr: number;
    status: PaymentTransactionStatus;
    settled_at: string | null;
  }>;
}

/** GET /api/v1/admin/payout/tenant-summary one item. */
export interface TenantPayoutSummary {
  tenant_id: string;
  tenant_name: string;
  tenant_slug: string;
  branch_count: number;
  settled_amount_idr: number;
  transaction_count: number;
}

export interface TenantPayoutSummaryResponse {
  data: TenantPayoutSummary[];
}

/** POST /api/v1/admin/disbursements request body. */
export interface CreateDisbursementInput {
  tenant_id: string;
  period_start: string;
  period_end: string;
}

/** Disbursement row (list + detail). */
export interface AdminDisbursement {
  id: string;
  tenant_id: string;
  tenant_name: string;
  period_start: string;
  period_end: string;
  gross_amount_idr: number;
  platform_fee_idr: number;
  net_amount_idr: number;
  transaction_count: number;
  status: DisbursementStatus;
  bank_reference: string | null;
  notes: string | null;
  transferred_at: string | null;
  transferred_by_name: string | null;
  created_at: string;
  updated_at: string;
}

export interface AdminDisbursementListResponse {
  data: AdminDisbursement[];
  total: number;
  page: number;
  total_pages: number;
}

export interface AdminDisbursementDetail extends AdminDisbursement {
  transactions: Array<{
    id: string;
    booking_id: string;
    booking_code: string;
    customer_name: string;
    received_amount_idr: number;
    platform_fee_idr: number;
    tenant_net_idr: number;
    paid_at: string | null;
  }>;
}

/** Reconcile response (POST /admin/settlement/reconcile). */
export interface ReconcileResponse {
  batch_id: string;
  settled_at: string;
  transaction_count: number;
  total_amount_idr: number;
  mismatch_count: number;
}
