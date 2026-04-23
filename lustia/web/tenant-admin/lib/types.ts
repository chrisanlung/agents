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

// ─── Phase 4 — Master Operational Data (ADR 0009) ────────────────────────────

/** TherapistResponse matches API §11.4 shape exactly. */
export interface Therapist {
  id: string;
  tenant_id: string;
  branch_id: string;
  user_id: string | null;
  full_name: string;
  gender: "male" | "female" | "other" | null;
  phone?: string | null;
  email?: string | null;
  bio: string | null;
  photo_url: string | null;
  specialties: string[];
  is_active: boolean;
  joined_at: string | null;
  created_at: string;
  updated_at: string;
  /** Present on GET /tenant/therapists/:id — all mappings (active + inactive) */
  services?: TherapistServiceMapping[];
}

export interface TherapistListResponse {
  data: Therapist[];
  next_cursor: string | null;
}

/** One entry in the services array on TherapistResponse (§11.4.3). */
export interface TherapistServiceMapping {
  service_id: string;
  name: string;
  category: string | null;
  duration_minutes: number;
  price_idr: number;
  is_active: boolean;
  assigned_at?: string;
}

/** ServiceResponse matches API §11.5 shape exactly. */
export interface Service {
  id: string;
  tenant_id: string;
  name: string;
  description: string | null;
  category: string | null;
  duration_minutes: number;
  price_idr: number;
  currency: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  /** Present on GET /tenant/services/:id — active mappings only */
  therapists?: ServiceTherapistEntry[];
}

export interface ServiceListResponse {
  data: Service[];
  next_cursor: string | null;
}

/** One entry in the therapists array on ServiceResponse (§11.5.3). */
export interface ServiceTherapistEntry {
  therapist_id: string;
  full_name: string;
  branch_id: string;
  branch_name: string;
  is_active: boolean;
}

/** A single availability window — one row in therapist_availability. */
export interface AvailabilityWindow {
  /** id is present on read (GET); omit when writing (PUT) */
  id?: string;
  /** day_of_week: 0=Sun … 6=Sat */
  dow: number;
  start: string; // "HH:MM"
  end: string;   // "HH:MM"
}

export interface AvailabilityResponse {
  therapist_id: string;
  windows: AvailabilityWindow[];
}

export interface ServiceMappingResponse {
  therapist_id: string;
  services: TherapistServiceMapping[];
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
