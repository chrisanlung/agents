"use client";

import { useState, useTransition } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { InitialPasswordModal } from "./initial-password-modal";
import { createUser, updateUser } from "./actions";
import type { User, Role } from "@/lib/types";

// ─── Indonesian role labels ───────────────────────────────────────────────────

const ROLE_LABELS: Record<string, string> = {
  super_admin: "Super Admin",
  tenant_admin: "Admin Tenant",
  branch_admin: "Admin Cabang",
  therapist: "Terapis",
  finance: "Keuangan",
};

// ─── Shared Zod schema (email optional — required only on create) ─────────────

const userFormSchema = z.object({
  email: z
    .string()
    .email("Format email tidak valid.")
    .max(320, "Email maksimal 320 karakter.")
    .optional()
    .or(z.literal("")),
  username: z
    .string()
    .min(3, "Username minimal 3 karakter.")
    .max(50, "Username maksimal 50 karakter.")
    .regex(/^[a-z0-9._]+$/, "Hanya huruf kecil, angka, titik, dan garis bawah.")
    .optional()
    .or(z.literal("")),
  full_name: z
    .string()
    .min(1, "Nama lengkap wajib diisi.")
    .max(200, "Nama maksimal 200 karakter."),
  phone: z
    .string()
    .min(5, "Telepon minimal 5 karakter.")
    .max(30, "Telepon maksimal 30 karakter.")
    .optional()
    .or(z.literal("")),
  role_ids: z.array(z.string()),
  branch_ids: z
    .array(z.string())
    .length(1, "Pilih tepat 1 cabang."),
  is_active: z.boolean(),
});

type UserFormValues = z.infer<typeof userFormSchema>;

// ─── Props ────────────────────────────────────────────────────────────────────

interface UserFormProps {
  /** Existing user — renders edit mode if provided */
  user?: User;
  roles: Role[];
  branches: { id: string; name: string }[];
}

// ─── Component ────────────────────────────────────────────────────────────────

export function UserForm({ user, roles, branches }: UserFormProps) {
  const isEdit = !!user;
  const [isPending, startTransition] = useTransition();
  const [createdCredentials, setCreatedCredentials] = useState<{
    email: string;
    username: string | null;
    initialPassword: string;
  } | null>(null);

  // Filter out "customer" role — not selectable for staff
  const staffRoles = roles.filter((r) => r.name !== "customer");

  const form = useForm<UserFormValues>({
    resolver: zodResolver(userFormSchema),
    defaultValues: {
      email: "",
      username: user?.username ?? "",
      full_name: user?.full_name ?? "",
      phone: user?.phone ?? "",
      role_ids: user?.role_ids ?? [],
      branch_ids: user?.branch_ids ?? [],
      is_active: user?.is_active ?? true,
    },
  });

  function onSubmit(values: UserFormValues) {
    startTransition(async () => {
      if (isEdit) {
        // Determine username delta:
        //   empty string → clear (send null)
        //   same as current value → omit (no change)
        //   new non-empty value → send value
        const originalUsername = user?.username ?? "";
        let usernameUpdate: string | null | undefined;
        if (values.username === originalUsername) {
          usernameUpdate = undefined; // omit — no change
        } else if (!values.username) {
          usernameUpdate = null; // clear
        } else {
          usernameUpdate = values.username;
        }

        const result = await updateUser(user!.id, {
          full_name: values.full_name,
          phone: values.phone || undefined,
          is_active: values.is_active,
          role_ids: values.role_ids,
          branch_ids: values.branch_ids,
          username: usernameUpdate,
        });

        // Guard against undefined return — happens when the server action's
        // session cookie expired mid-flight or when Next.js silently aborted
        // the action due to revalidation.
        if (!result) {
          toast.error("Sesi habis. Silakan masuk kembali.");
          return;
        }
        if (!result.ok) {
          Object.entries(result.errors).forEach(([field, message]) => {
            form.setError(field as keyof UserFormValues, { message });
          });
          if (result.error) toast.error(result.error);
          return;
        }
        toast.success("Pengguna berhasil diperbarui.");
      } else {
        if (!values.email) {
          form.setError("email", { message: "Email wajib diisi." });
          return;
        }

        const result = await createUser({
          email: values.email,
          username: values.username || undefined,
          full_name: values.full_name,
          phone: values.phone || undefined,
          role_ids: values.role_ids,
          branch_ids: values.branch_ids,
          is_active: values.is_active,
        });

        if (!result) {
          toast.error("Sesi habis. Silakan masuk kembali.");
          return;
        }
        if (!result.ok) {
          Object.entries(result.errors).forEach(([field, message]) => {
            form.setError(field as keyof UserFormValues, { message });
          });
          if (result.error) toast.error(result.error);
          return;
        }

        // Show the initial password modal — only returned once
        setCreatedCredentials({
          email: values.email,
          username: values.username || null,
          initialPassword: result.initial_password,
        });
      }
    });
  }

  return (
    <>
      {createdCredentials && (
        <InitialPasswordModal
          email={createdCredentials.email}
          username={createdCredentials.username}
          initialPassword={createdCredentials.initialPassword}
          onClose={() => setCreatedCredentials(null)}
        />
      )}

      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
          {/* ── Informasi Akun ── */}
          <section className="space-y-4">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
              Informasi Akun
            </h2>

            {/* Email — only on create; shown read-only on edit */}
            {!isEdit ? (
              <FormField
                control={form.control}
                name="email"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      Email <span className="text-destructive">*</span>
                    </FormLabel>
                    <FormControl>
                      <Input
                        type="email"
                        placeholder="staff@perusahaan.com"
                        autoComplete="off"
                        {...field}
                        value={field.value ?? ""}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ) : (
              <div className="space-y-1.5">
                <label className="text-sm font-medium leading-none">
                  Email
                </label>
                <p className="flex h-9 w-full items-center rounded-md border border-input bg-muted px-3 py-2 text-sm text-muted-foreground">
                  {user!.email}
                </p>
                <p className="text-xs text-muted-foreground">
                  Email tidak dapat diubah setelah akun dibuat.
                </p>
              </div>
            )}

            {/* Username — optional, editable on both create and edit */}
            <FormField
              control={form.control}
              name="username"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Username (opsional)</FormLabel>
                  <FormControl>
                    <Input
                      type="text"
                      placeholder="alice (huruf kecil, angka, titik, garis bawah)"
                      autoComplete="off"
                      {...field}
                      value={field.value ?? ""}
                      onChange={(e) =>
                        field.onChange(e.target.value.toLowerCase())
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    Staff bisa login pakai username atau email. Kosongkan kalau
                    tidak perlu.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <FormField
                control={form.control}
                name="full_name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      Nama Lengkap <span className="text-destructive">*</span>
                    </FormLabel>
                    <FormControl>
                      <Input
                        placeholder="Budi Santoso"
                        {...field}
                        value={field.value ?? ""}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="phone"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Nomor Telepon</FormLabel>
                    <FormControl>
                      <Input
                        type="tel"
                        placeholder="+62 812 3456 7890"
                        {...field}
                        value={field.value ?? ""}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </section>

          {/* ── Role ── */}
          <section className="space-y-4">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
              Role
            </h2>
            <FormField
              control={form.control}
              name="role_ids"
              render={({ field }) => (
                <FormItem>
                  <div className="space-y-2">
                    {staffRoles.map((role) => {
                      const checked = (field.value as string[]).includes(
                        role.id
                      );
                      return (
                        <label
                          key={role.id}
                          className="flex cursor-pointer items-start gap-3 rounded-lg border bg-card p-3 hover:bg-muted/50 transition-colors"
                        >
                          <Checkbox
                            checked={checked}
                            onCheckedChange={(c) => {
                              const current = field.value as string[];
                              if (c) {
                                field.onChange([...current, role.id]);
                              } else {
                                field.onChange(
                                  current.filter((id) => id !== role.id)
                                );
                              }
                            }}
                          />
                          <div className="min-w-0">
                            <p className="text-sm font-medium leading-none">
                              {ROLE_LABELS[role.name] ?? role.name}
                            </p>
                            {role.description && (
                              <p className="mt-1 text-xs text-muted-foreground">
                                {role.description}
                              </p>
                            )}
                          </div>
                        </label>
                      );
                    })}
                    {staffRoles.length === 0 && (
                      <p className="text-sm text-muted-foreground">
                        Tidak ada role tersedia.
                      </p>
                    )}
                  </div>
                  <FormMessage />
                </FormItem>
              )}
            />
          </section>

          {/* ── Cabang ── */}
          {branches.length > 0 && (
            <section className="space-y-4">
              <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                Cabang <span className="text-destructive">*</span>
              </h2>
              <FormField
                control={form.control}
                name="branch_ids"
                render={({ field }) => (
                  <FormItem>
                    <FormDescription>
                      Satu pengguna ditetapkan ke satu cabang.
                    </FormDescription>
                    <div
                      role="radiogroup"
                      aria-label="Cabang"
                      className="grid grid-cols-1 gap-2 sm:grid-cols-2"
                    >
                      {branches.map((branch) => {
                        const current = (field.value as string[])[0];
                        const selected = current === branch.id;
                        return (
                          <label
                            key={branch.id}
                            className={`flex cursor-pointer items-center gap-3 rounded-lg border p-3 transition-colors ${
                              selected
                                ? "border-primary bg-primary/5"
                                : "bg-card hover:bg-muted/50"
                            }`}
                          >
                            <input
                              type="radio"
                              name="branch_id"
                              value={branch.id}
                              checked={selected}
                              onChange={() => field.onChange([branch.id])}
                              className="h-4 w-4 accent-primary"
                            />
                            <span className="text-sm font-medium">
                              {branch.name}
                            </span>
                          </label>
                        );
                      })}
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </section>
          )}

          {/* ── Status ── */}
          <section className="space-y-4">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
              Status
            </h2>
            <FormField
              control={form.control}
              name="is_active"
              render={({ field }) => (
                <FormItem>
                  <label className="flex cursor-pointer items-center gap-3">
                    <FormControl>
                      <Checkbox
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                    <div>
                      <p className="text-sm font-medium leading-none">
                        Akun aktif
                      </p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        Pengguna dapat login bila akun aktif.
                      </p>
                    </div>
                  </label>
                  <FormMessage />
                </FormItem>
              )}
            />
          </section>

          {/* ── Actions ── */}
          <div className="flex items-center gap-3 pt-2">
            <Button type="submit" disabled={isPending}>
              {isPending && (
                <Loader2
                  size={14}
                  className="animate-spin"
                  aria-hidden="true"
                />
              )}
              {isEdit ? "Simpan Perubahan" : "Buat Pengguna"}
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={() => window.history.back()}
              disabled={isPending}
            >
              Batal
            </Button>
          </div>
        </form>
      </Form>
    </>
  );
}
