/**
 * API response types for Phase 5 booking endpoints (ADR 0014 §3.16).
 * These match the Go backend response shapes in API_CONTRACT §14 exactly.
 */

// ─── Booking Status ──────────────────────────────────────────────────────────

export type BookingStatus =
  | "pending_payment"
  | "paid"
  | "checked_in"
  | "completed"
  | "cancelled"
  | "expired"
  | "no_show";

// ─── Booking ─────────────────────────────────────────────────────────────────

export interface BookingAddon {
  addon_id: string;
  name: string;
  price_idr: number;
}

/** Full operator booking response (API §14.5). */
export interface Booking {
  id: string;
  branch_id: string;
  branch_name: string;
  service_id: string;
  service_name: string;
  room_id: string | null;
  room_name: string | null;
  therapist_id: string | null;
  therapist_name: string | null;
  customer_name: string;
  customer_phone: string;
  customer_email: string;
  code: string;
  scheduled_start: string;
  scheduled_end: string;
  total_price_idr: number;
  payment_method: string | null;
  payment_reference: string | null;
  paid_at: string | null;
  status: BookingStatus;
  cancelled_at: string | null;
  cancelled_by: string | null;
  cancel_reason: string | null;
  checked_in_at: string | null;
  checked_in_by: string | null;
  completed_at: string | null;
  completed_by: string | null;
  addons: BookingAddon[];
  created_at: string;
  updated_at: string;
}

export interface BookingListResponse {
  data: Booking[];
  page: number;
  limit: number;
  total_count: number;
  total_pages: number;
}

// ─── Availability ─────────────────────────────────────────────────────────────

export interface AvailabilitySlot {
  start: string;
  end: string;
  therapists_available_count: number;
  rooms_available_count: number;
}

// ─── Branch / Therapist (ops-scoped) ─────────────────────────────────────────

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

export interface Therapist {
  id: string;
  full_name: string;
  is_active: boolean;
  branch_id: string;
}

export interface TherapistListResponse {
  data: Therapist[];
  page: number;
  limit: number;
  total_count: number;
  total_pages: number;
}

export interface Service {
  id: string;
  name: string;
  description: string | null;
  category: string | null;
  duration_minutes: number;
  price_idr: number;
  currency: string;
  is_active: boolean;
}

export interface ServiceListResponse {
  data: Service[];
  page: number;
  limit: number;
  total_count: number;
  total_pages: number;
}

export interface Addon {
  id: string;
  name: string;
  description: string | null;
  price_idr: number;
  is_active: boolean;
  sort_order: number;
}

export interface AddonListResponse {
  data: Addon[];
  page: number;
  limit: number;
  total_count: number;
  total_pages: number;
}
