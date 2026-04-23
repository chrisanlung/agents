/**
 * API response types for Phase 3 endpoints (ADR 0008 §2.2).
 * These match the Go backend response shapes exactly.
 */

// ─── Branch ──────────────────────────────────────────────────────────────────

export type BranchStatus = "active" | "inactive";

export interface Branch {
  id: string;
  tenant_id: string;
  name: string;
  code: string;
  status: BranchStatus;
  address_line1: string | null;
  address_line2: string | null;
  city: string | null;
  province: string | null;
  postal_code: string | null;
  /** ISO 3166-1 alpha-2, always "ID" for Phase 3 */
  country: string;
  timezone: string;
  contact_phone: string | null;
  contact_email: string | null;
  operational_hours: Record<string, string> | null;
  activated_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface BranchListResponse {
  data: Branch[];
  next_cursor: string | null;
}

// ─── Onboarding state (ADR 0008 §2.3.3) ─────────────────────────────────────

/**
 * GET /api/v1/tenant/onboarding-state
 *
 * NOTE FOR go-expert: this response MUST include max_branches.
 * Without it, the branch limit counter cannot be displayed.
 * If max_branches is absent from the real response, the UI falls back to
 * showing "?" and logs a cross-agent flag.
 */
export interface OnboardingState {
  has_branches: boolean;
  must_change_password: boolean;
  /**
   * Package matrix: starter=1, growth=5, enterprise=999 (unlimited sentinel).
   */
  max_branches: number | null;
  active_branch_count: number;
}
