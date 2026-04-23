"use client";

import { useTransition } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import {
  Form,
  FormControl,
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
import { createTherapist, updateTherapist } from "./actions";
import type { Therapist, Branch } from "@/lib/types";

// ─── Zod schema ──────────────────────────────────────────────────────────────

const therapistFormSchema = z.object({
  full_name: z.string().min(1, "Nama lengkap wajib diisi.").max(200),
  gender: z.enum(["male", "female", "other", ""]).optional(),
  phone: z.string().max(30).optional(),
  email: z
    .string()
    .email("Format email tidak valid.")
    .optional()
    .or(z.literal("")),
  bio: z.string().max(500, "Bio maksimal 500 karakter.").optional(),
  joined_at: z.string().optional(),
  branch_id: z.string().optional(),
});

type TherapistFormValues = z.infer<typeof therapistFormSchema>;

// ─── Props ───────────────────────────────────────────────────────────────────

interface TherapistFormProps {
  therapist?: Therapist;
  /** Branches list — shown as Branch selector on /new for tenant_admin */
  branches?: Branch[];
  /** If true, show the branch selector */
  showBranchSelector?: boolean;
}

// ─── Component ───────────────────────────────────────────────────────────────

export function TherapistForm({
  therapist,
  branches = [],
  showBranchSelector = false,
}: TherapistFormProps) {
  const isEdit = !!therapist;
  const [isPending, startTransition] = useTransition();
  const router = useRouter();

  const form = useForm<TherapistFormValues>({
    resolver: zodResolver(therapistFormSchema),
    defaultValues: {
      full_name: therapist?.full_name ?? "",
      gender: (therapist?.gender as TherapistFormValues["gender"]) ?? "",
      phone: therapist?.phone ?? "",
      email: therapist?.email ?? "",
      bio: therapist?.bio ?? "",
      joined_at: therapist?.joined_at
        ? therapist.joined_at.split("T")[0]
        : "",
      branch_id: therapist?.branch_id ?? "",
    },
  });

  function onSubmit(values: TherapistFormValues) {
    startTransition(async () => {
      const fd = new FormData();
      Object.entries(values).forEach(([k, v]) => {
        if (v !== undefined && v !== null && v !== "") fd.set(k, String(v));
      });

      const result = isEdit
        ? await updateTherapist(therapist!.id, fd)
        : await createTherapist(fd);

      if (!result.ok) {
        Object.entries(result.errors).forEach(([field, message]) => {
          form.setError(field as keyof TherapistFormValues, { message });
        });
        if (result.error) toast.error(result.error);
        return;
      }

      if (isEdit) {
        toast.success("Perubahan berhasil disimpan.");
        router.refresh();
      } else {
        toast.success("Terapis berhasil ditambahkan.");
        // redirect happens server-side via redirect() in createTherapist
      }
    });
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
        {/* Branch selector — tenant_admin on /new only */}
        {showBranchSelector && (
          <FormField
            control={form.control}
            name="branch_id"
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  Cabang <span className="text-destructive">*</span>
                </FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue placeholder="Pilih cabang" />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {branches.map((b) => (
                      <SelectItem key={b.id} value={b.id}>
                        {b.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />
        )}

        {/* Nama Lengkap */}
        <FormField
          control={form.control}
          name="full_name"
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                Nama Lengkap <span className="text-destructive">*</span>
              </FormLabel>
              <FormControl>
                <Input placeholder="Siti Rahmawati" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {/* No. Telepon */}
          <FormField
            control={form.control}
            name="phone"
            render={({ field }) => (
              <FormItem>
                <FormLabel>No. Telepon</FormLabel>
                <FormControl>
                  <Input type="tel" placeholder="+62812..." {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          {/* Email */}
          <FormField
            control={form.control}
            name="email"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Email</FormLabel>
                <FormControl>
                  <Input type="email" placeholder="terapis@contoh.com" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {/* Gender */}
          <FormField
            control={form.control}
            name="gender"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Jenis Kelamin</FormLabel>
                <Select value={field.value ?? ""} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue placeholder="Pilih jenis kelamin" />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value="">Tidak disebutkan</SelectItem>
                    <SelectItem value="male">Laki-laki</SelectItem>
                    <SelectItem value="female">Perempuan</SelectItem>
                    <SelectItem value="other">Lainnya</SelectItem>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          {/* Bergabung Sejak */}
          <FormField
            control={form.control}
            name="joined_at"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Bergabung Sejak</FormLabel>
                <FormControl>
                  <Input
                    type="date"
                    placeholder="dd/mm/yyyy"
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        {/* Bio */}
        <FormField
          control={form.control}
          name="bio"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Bio</FormLabel>
              <FormControl>
                <textarea
                  {...field}
                  rows={4}
                  maxLength={500}
                  placeholder="Ceritakan sedikit tentang terapis ini..."
                  className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Read-only linked account */}
        {isEdit && (
          <div className="rounded-md border bg-muted/30 px-3 py-2">
            <p className="text-xs text-muted-foreground">Akun terhubung</p>
            <p className="mt-0.5 text-sm text-foreground">
              {therapist?.user_id ? therapist.email ?? therapist.user_id : "Belum terhubung"}
            </p>
          </div>
        )}

        {/* Submit */}
        <div className="flex items-center gap-3 pt-2">
          <Button type="submit" disabled={isPending}>
            {isPending && (
              <Loader2 size={14} className="animate-spin" aria-hidden="true" />
            )}
            {isEdit ? "Simpan Perubahan" : "Simpan & Buat Terapis"}
          </Button>
          <Button
            type="button"
            variant="ghost"
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
