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
  page: number;
  limit: number;
  total_count: number;
  total_pages: number;
}

// ─── Phase 4 — Master Operational Data (ADR 0009) ────────────────────────────

/** TherapistResponse matches API §11.4 shape exactly (ADR 0011 §2.3). */
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
  /**
   * Resolved public URL for the therapist photo (backend maps photo_key → URL
   * at the controller boundary). Never `photo_key` directly — ADR 0011 §2.4.
   */
  photo_url: string | null;
  /** Height in centimetres. 100–250. ADR 0011 §2.3. */
  height_cm: number;
  /** Weight in kilograms. 30–250. ADR 0011 §2.3. */
  weight_kg: number;
  /** Customer-visible build category. ADR 0011 §2.3. */
  build: "langsing" | "sedang" | "atletis" | "tegap";
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
  page: number;
  limit: number;
  total_count: number;
  total_pages: number;
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
  page: number;
  limit: number;
  total_count: number;
  total_pages: number;
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

// ─── Phase 4 — Tenant-wide Add-on Catalog (ADR 0010 revised) ─────────────────

/** AddonResponse matches API §4.2.2 shape exactly (tenant-wide catalog).
 * Deliberately omits `tenant_id` and `deleted_at` — the backend never emits
 * these fields to the wire. */
export interface Addon {
  id: string;
  name: string;
  description: string | null;
  price_idr: number;
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface AddonListResponse {
  data: Addon[];
  page: number;
  limit: number;
  total_count: number;
  total_pages: number;
}

/** Request DTOs for add-on mutations (tenant-wide, no service_id). */
export interface CreateAddonInput {
  name: string;
  description?: string | null;
  price_idr: number;
  is_active?: boolean;
}

export interface UpdateAddonInput {
  name?: string;
  description?: string | null;
  price_idr?: number;
  is_active?: boolean;
}

/** One item in a bulk reorder request payload. */
export interface AddonSortOrderItem {
  id: string;
  sort_order: number;
}

// ─── Phase 4 — Ruangan (Room) Catalog (ADR 0012) ─────────────────────────────

export type RoomType = "single" | "couple" | "group" | "vip";

/** RoomResponse matches API §13 shape exactly (ADR 0012 §2.3).
 * Deliberately omits `tenant_id` and `photo_key` — the backend never emits
 * these fields to the wire. */
export interface Room {
  id: string;
  branch_id: string;
  name: string;
  description: string | null;
  room_type: RoomType;
  capacity: number;
  amenities: string[];
  /**
   * Resolved public URL for the room photo (backend maps photo_key → URL
   * at the controller boundary). Never `photo_key` directly — ADR 0011 §2.4.
   */
  photo_url: string | null;
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface RoomListResponse {
  data: Room[];
  page: number;
  limit: number;
  total_count: number;
  total_pages: number;
}

/** Request DTOs for room mutations. */
export interface CreateRoomInput {
  branch_id: string;
  name: string;
  description?: string | null;
  room_type: RoomType;
  capacity: number;
  amenities: string[];
  is_active: boolean;
}

export interface UpdateRoomInput {
  name?: string;
  description?: string | null;
  room_type?: RoomType;
  capacity?: number;
  amenities?: string[];
  is_active?: boolean;
}

/** One item in a bulk reorder request payload (rooms are branch-scoped). */
export interface RoomSortOrderItem {
  id: string;
  sort_order: number;
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
