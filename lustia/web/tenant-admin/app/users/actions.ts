"use server";

import { revalidateTag } from "next/cache";
import { z } from "zod";

import { apiFetch, ApiError } from "@/lib/api";
import type { User } from "@/lib/types";

// ─── Schemas ─────────────────────────────────────────────────────────────────

const usernameSchema = z
  .string()
  .min(3, "Username minimal 3 karakter.")
  .max(50, "Username maksimal 50 karakter.")
  .regex(/^[a-z0-9._]+$/, "Hanya huruf kecil, angka, titik, dan garis bawah.")
  .optional()
  .or(z.literal(""));

const createUserSchema = z.object({
  email: z.string().email("Format email tidak valid.").max(320),
  username: usernameSchema,
  full_name: z.string().min(1, "Nama lengkap wajib diisi.").max(200),
  phone: z.string().min(5).max(30).optional().or(z.literal("")),
  role_ids: z.array(z.string().uuid()).optional(),
  branch_ids: z.array(z.string().uuid()).optional(),
  is_active: z.boolean().optional(),
});

const updateUserSchema = z.object({
  full_name: z.string().min(1, "Nama lengkap wajib diisi.").max(200).optional(),
  // Three-valued: undefined = no change, null = clear, string = set
  username: z.string().min(3).max(50).regex(/^[a-z0-9._]+$/).optional().nullable(),
  phone: z.string().min(5).max(30).optional().or(z.literal("")),
  is_active: z.boolean().optional(),
  role_ids: z.array(z.string().uuid()).optional(),
  branch_ids: z.array(z.string().uuid()).optional(),
});

// ─── Backend response from POST /admin/users ──────────────────────────────

interface CreateUserApiResponse {
  user: User;
  initial_password?: string;
  created_user: boolean;
  created_membership: boolean;
}

// ─── Return types ─────────────────────────────────────────────────────────────

export type CreateUserResult =
  | { ok: true; user: User; initial_password: string; created_user: boolean }
  | { ok: false; errors: Record<string, string>; error?: string };

export type UpdateUserResult =
  | { ok: true; user: User }
  | { ok: false; errors: Record<string, string>; error?: string };

export type ToggleUserStatusResult =
  | { ok: true }
  | { ok: false; error: string };

export type UnlockUserResult =
  | { ok: true }
  | { ok: false; error: string };

// ─── Helpers ──────────────────────────────────────────────────────────────────

function parseZodErrors(err: z.ZodError): Record<string, string> {
  const errors: Record<string, string> = {};
  err.errors.forEach((e) => {
    const field = e.path.join(".");
    errors[field] = e.message;
  });
  return errors;
}

function parseApiError(err: ApiError): { errors: Record<string, string>; error?: string } {
  if (err.code === "DUPLICATE_EMAIL") {
    return { errors: { email: "Email sudah digunakan oleh pengguna lain." } };
  }
  if (err.code === "USERNAME_ALREADY_TAKEN") {
    return { errors: { username: "Username sudah dipakai user lain." } };
  }
  if (err.code === "USERNAME_INVALID") {
    return { errors: { username: "Format username tidak valid." } };
  }
  if (err.code === "USER_NOT_FOUND") {
    return { errors: {}, error: "Pengguna tidak ditemukan." };
  }
  if (err.code === "FORBIDDEN") {
    return { errors: {}, error: "Anda tidak memiliki izin untuk melakukan tindakan ini." };
  }
  return { errors: {}, error: err.message };
}

// ─── Create ───────────────────────────────────────────────────────────────────

/**
 * Creates a new staff user.
 * Returns the initial_password ONCE — show it in a modal and never store it.
 */
export async function createUser(input: {
  email: string;
  username?: string;
  full_name: string;
  phone?: string;
  role_ids: string[];
  branch_ids: string[];
  is_active: boolean;
}): Promise<CreateUserResult> {
  const parsed = createUserSchema.safeParse(input);

  if (!parsed.success) {
    return { ok: false, errors: parseZodErrors(parsed.error) };
  }

  try {
    const body: Record<string, unknown> = {
      email: parsed.data.email,
      full_name: parsed.data.full_name,
    };
    if (parsed.data.username) body.username = parsed.data.username;
    if (parsed.data.phone) body.phone = parsed.data.phone;
    if (parsed.data.role_ids?.length) body.role_ids = parsed.data.role_ids;
    if (parsed.data.branch_ids?.length) body.branch_ids = parsed.data.branch_ids;

    const res = await apiFetch<CreateUserApiResponse>(
      "/admin/users",
      { method: "POST", body: JSON.stringify(body) },
      { auth: true }
    );

    revalidateTag("admin-users", { expire: 0 });

    return {
      ok: true,
      user: res.user,
      initial_password: res.initial_password ?? "",
      created_user: res.created_user,
    };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, ...parseApiError(err) };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Update ───────────────────────────────────────────────────────────────────

export async function updateUser(
  id: string,
  input: {
    full_name?: string;
    /** undefined = omit (no change), null = clear, string = set */
    username?: string | null;
    phone?: string;
    is_active?: boolean;
    role_ids?: string[];
    branch_ids?: string[];
  }
): Promise<UpdateUserResult> {
  const parsed = updateUserSchema.safeParse(input);

  if (!parsed.success) {
    return { ok: false, errors: parseZodErrors(parsed.error) };
  }

  try {
    const body: Record<string, unknown> = {};
    if (parsed.data.full_name !== undefined) body.full_name = parsed.data.full_name;
    // Three-valued: only include username in body if caller provided it
    if (input.username !== undefined) body.username = parsed.data.username ?? null;
    if (parsed.data.phone !== undefined) body.phone = parsed.data.phone || null;
    if (parsed.data.is_active !== undefined) body.is_active = parsed.data.is_active;
    if (parsed.data.role_ids !== undefined) body.role_ids = parsed.data.role_ids;
    if (parsed.data.branch_ids !== undefined) body.branch_ids = parsed.data.branch_ids;

    const user = await apiFetch<User>(
      `/admin/users/${id}`,
      { method: "PATCH", body: JSON.stringify(body) },
      { auth: true }
    );

    revalidateTag("admin-users", { expire: 0 });
    revalidateTag(`admin-user-${id}`, { expire: 0 });

    return { ok: true, user };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, ...parseApiError(err) };
    }
    return { ok: false, errors: {}, error: "Terjadi kesalahan. Coba lagi." };
  }
}

// ─── Toggle active status ─────────────────────────────────────────────────────

export async function toggleUserStatus(
  id: string,
  isActive: boolean
): Promise<ToggleUserStatusResult> {
  try {
    await apiFetch<User>(
      `/admin/users/${id}`,
      { method: "PATCH", body: JSON.stringify({ is_active: isActive }) },
      { auth: true }
    );

    revalidateTag("admin-users", { expire: 0 });
    revalidateTag(`admin-user-${id}`, { expire: 0 });

    return { ok: true };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}

// ─── Unlock ───────────────────────────────────────────────────────────────────

export async function unlockUser(id: string): Promise<UnlockUserResult> {
  try {
    await apiFetch<void>(
      `/admin/users/${id}/unlock`,
      { method: "POST" },
      { auth: true }
    );

    revalidateTag("admin-users", { expire: 0 });
    revalidateTag(`admin-user-${id}`, { expire: 0 });

    return { ok: true };
  } catch (err) {
    if (err instanceof ApiError) {
      return { ok: false, error: err.message };
    }
    return { ok: false, error: "Terjadi kesalahan." };
  }
}
