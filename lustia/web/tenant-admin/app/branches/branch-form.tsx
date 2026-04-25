"use client";

import { useTransition } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { OperationalHoursField } from "./operational-hours-field";
import { createBranch, updateBranch } from "./actions";
import type { Branch } from "@/lib/types";

// ─── Zod schema (mirrors server schema) ─────────────────────────────────────

const DEFAULT_OPERATIONAL_HOURS = JSON.stringify({
  mon: "09:00-17:00",
  tue: "09:00-17:00",
  wed: "09:00-17:00",
  thu: "09:00-17:00",
  fri: "09:00-17:00",
  sat: "09:00-14:00",
  sun: null,
});

const branchFormSchema = z.object({
  name: z.string().min(1, "Nama cabang wajib diisi.").max(200),
  code: z
    .string()
    .min(2, "Kode minimal 2 karakter.")
    .max(50)
    .regex(/^[a-z0-9][a-z0-9-]*[a-z0-9]$/, {
      message:
        "Hanya huruf kecil, angka, dan tanda hubung. Tidak boleh diawali/diakhiri tanda hubung.",
    }),
  address_line1: z.string().max(200).optional(),
  address_line2: z.string().max(200).optional(),
  city: z.string().max(100).optional(),
  province: z.string().max(100).optional(),
  postal_code: z.string().max(10).optional(),
  country: z.string().length(2).default("ID"),
  timezone: z.enum(["Asia/Jakarta", "Asia/Makassar", "Asia/Jayapura"]),
  contact_phone: z.string().max(30).optional(),
  contact_email: z
    .string()
    .email("Format email tidak valid.")
    .optional()
    .or(z.literal("")),
  operational_hours: z.string().optional(),
});

type BranchFormValues = z.infer<typeof branchFormSchema>;

// ─── Props ───────────────────────────────────────────────────────────────────

interface BranchFormProps {
  /** Existing branch — if provided, renders an edit form */
  branch?: Branch;
}

// ─── Component ───────────────────────────────────────────────────────────────

export function BranchForm({ branch }: BranchFormProps) {
  const isEdit = !!branch;
  const [isPending, startTransition] = useTransition();

  const form = useForm<BranchFormValues>({
    resolver: zodResolver(branchFormSchema),
    defaultValues: {
      name: branch?.name ?? "",
      code: branch?.code ?? "",
      address_line1: branch?.address_line1 ?? "",
      address_line2: branch?.address_line2 ?? "",
      city: branch?.city ?? "",
      province: branch?.province ?? "",
      postal_code: branch?.postal_code ?? "",
      country: "ID",
      timezone:
        (branch?.timezone as BranchFormValues["timezone"] | undefined) ??
        "Asia/Jakarta",
      contact_phone: branch?.contact_phone ?? "",
      contact_email: branch?.contact_email ?? "",
      operational_hours: branch?.operational_hours
        ? JSON.stringify(branch.operational_hours)
        : DEFAULT_OPERATIONAL_HOURS,
    },
  });

  function onSubmit(values: BranchFormValues) {
    startTransition(async () => {
      const fd = new FormData();
      Object.entries(values).forEach(([k, v]) => {
        if (v !== undefined && v !== null) fd.set(k, String(v));
      });

      const result = isEdit
        ? await updateBranch(branch!.id, fd)
        : await createBranch(fd);

      if (!result.ok) {
        // Surface field errors
        Object.entries(result.errors).forEach(([field, message]) => {
          form.setError(field as keyof BranchFormValues, { message });
        });
        if (result.error) toast.error(result.error);
        return;
      }
      // Server action redirects on success — toast shown via query param on landing page
    });
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
        {/* ── Basic info ── */}
        <section className="space-y-4">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
            Informasi Dasar
          </h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    Nama Cabang <span className="text-destructive">*</span>
                  </FormLabel>
                  <FormControl>
                    <Input placeholder="Cabang Utama Jakarta" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="code"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    Kode <span className="text-destructive">*</span>
                  </FormLabel>
                  <FormControl>
                    <Input
                      placeholder="jakarta-utama"
                      {...field}
                      onChange={(e) =>
                        field.onChange(e.target.value.toLowerCase())
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    Huruf kecil, angka, dan tanda hubung saja.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </section>

        {/* ── Alamat ── */}
        <section className="space-y-4">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
            Alamat
          </h2>
          <FormField
            control={form.control}
            name="address_line1"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Alamat Baris 1</FormLabel>
                <FormControl>
                  <Input placeholder="Jl. Sudirman No. 1" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name="address_line2"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Alamat Baris 2</FormLabel>
                <FormControl>
                  <Input placeholder="Gedung X, Lantai 5" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <FormField
              control={form.control}
              name="city"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Kota</FormLabel>
                  <FormControl>
                    <Input placeholder="Jakarta" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="province"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Provinsi</FormLabel>
                  <FormControl>
                    <Input placeholder="DKI Jakarta" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="postal_code"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Kode Pos</FormLabel>
                  <FormControl>
                    <Input placeholder="10220" maxLength={10} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          {/* Country — read-only for Phase 3 */}
          <FormField
            control={form.control}
            name="country"
            render={({ field }) => (
              <FormItem className="hidden">
                <FormControl>
                  <Input {...field} readOnly />
                </FormControl>
              </FormItem>
            )}
          />
        </section>

        {/* ── Operasional ── */}
        <section className="space-y-4">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
            Operasional
          </h2>
          <FormField
            control={form.control}
            name="timezone"
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  Zona Waktu <span className="text-destructive">*</span>
                </FormLabel>
                <Select
                  value={field.value}
                  onValueChange={field.onChange}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue placeholder="Pilih zona waktu" />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value="Asia/Jakarta">
                      WIB — Asia/Jakarta (UTC+7)
                    </SelectItem>
                    <SelectItem value="Asia/Makassar">
                      WITA — Asia/Makassar (UTC+8)
                    </SelectItem>
                    <SelectItem value="Asia/Jayapura">
                      WIT — Asia/Jayapura (UTC+9)
                    </SelectItem>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name="operational_hours"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Jam Operasional</FormLabel>
                <FormControl>
                  <OperationalHoursField
                    value={field.value}
                    onChange={field.onChange}
                    disabled={isPending}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </section>

        {/* ── Kontak ── */}
        <section className="space-y-4">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
            Kontak Cabang
          </h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormField
              control={form.control}
              name="contact_phone"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Telepon</FormLabel>
                  <FormControl>
                    <Input
                      type="tel"
                      placeholder="+62 21 1234 5678"
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="contact_email"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Email</FormLabel>
                  <FormControl>
                    <Input
                      type="email"
                      placeholder="cabang@perusahaan.com"
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </section>

        {/* ── Submit ── */}
        <div className="flex items-center gap-3 pt-2">
          <Button type="submit" disabled={isPending}>
            {isPending && (
              <Loader2
                size={14}
                className="animate-spin"
                aria-hidden="true"
              />
            )}
            {isEdit ? "Simpan Perubahan" : "Tambah Cabang"}
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
  );
}
