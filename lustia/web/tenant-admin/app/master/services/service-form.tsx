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
import { createService, updateService } from "./actions";
import type { Service } from "@/lib/types";

// ─── Zod schema ──────────────────────────────────────────────────────────────

const serviceFormSchema = z.object({
  name: z.string().min(1, "Nama layanan wajib diisi.").max(200),
  category: z.string().max(100).optional(),
  duration_minutes: z.coerce
    .number({ invalid_type_error: "Durasi harus berupa angka." })
    .int("Durasi harus bilangan bulat.")
    .min(1, "Durasi minimal 1 menit.")
    .max(1440, "Durasi maksimal 1440 menit."),
  price_idr: z.coerce
    .number({ invalid_type_error: "Harga harus berupa angka." })
    .int("Harga harus bilangan bulat.")
    .min(0, "Harga tidak boleh negatif."),
  description: z.string().max(1000, "Deskripsi maksimal 1000 karakter.").optional(),
});

type ServiceFormValues = z.infer<typeof serviceFormSchema>;

// ─── Props ───────────────────────────────────────────────────────────────────

interface ServiceFormProps {
  service?: Service;
}

// ─── Component ───────────────────────────────────────────────────────────────

export function ServiceForm({ service }: ServiceFormProps) {
  const isEdit = !!service;
  const [isPending, startTransition] = useTransition();
  const router = useRouter();

  const form = useForm<ServiceFormValues>({
    resolver: zodResolver(serviceFormSchema),
    defaultValues: {
      name: service?.name ?? "",
      category: service?.category ?? "",
      duration_minutes: service?.duration_minutes ?? 60,
      price_idr: service?.price_idr ?? 0,
      description: service?.description ?? "",
    },
  });

  function onSubmit(values: ServiceFormValues) {
    startTransition(async () => {
      const fd = new FormData();
      Object.entries(values).forEach(([k, v]) => {
        if (v !== undefined && v !== null) fd.set(k, String(v));
      });

      const result = isEdit
        ? await updateService(service!.id, fd)
        : await createService(fd);

      if (!result.ok) {
        Object.entries(result.errors).forEach(([field, message]) => {
          form.setError(field as keyof ServiceFormValues, { message });
        });
        if (result.error) toast.error(result.error);
        return;
      }

      if (isEdit) {
        toast.success("Perubahan berhasil disimpan.");
        router.refresh();
      } else {
        toast.success("Layanan berhasil ditambahkan.");
        // redirect happens server-side via redirect() in createService
      }
    });
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
        {/* Nama Layanan */}
        <FormField
          control={form.control}
          name="name"
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                Nama Layanan <span className="text-destructive">*</span>
              </FormLabel>
              <FormControl>
                <Input placeholder="Pijat Relaksasi 60 Menit" {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Kategori */}
        <FormField
          control={form.control}
          name="category"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Kategori</FormLabel>
              <FormControl>
                <Input placeholder="Pijat, Facial, Body Treatment..." {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {/* Durasi */}
          <FormField
            control={form.control}
            name="duration_minutes"
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  Durasi <span className="text-destructive">*</span>
                </FormLabel>
                <div className="flex items-center gap-2">
                  <FormControl>
                    <Input
                      type="number"
                      min={1}
                      max={1440}
                      placeholder="60"
                      {...field}
                    />
                  </FormControl>
                  <span className="shrink-0 text-sm text-muted-foreground">menit</span>
                </div>
                <FormMessage />
              </FormItem>
            )}
          />

          {/* Harga */}
          <FormField
            control={form.control}
            name="price_idr"
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  Harga <span className="text-destructive">*</span>
                </FormLabel>
                <div className="flex items-center gap-2">
                  <span className="shrink-0 text-sm text-muted-foreground">Rp</span>
                  <FormControl>
                    <Input
                      type="number"
                      min={0}
                      placeholder="150000"
                      {...field}
                    />
                  </FormControl>
                </div>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        {/* Deskripsi */}
        <FormField
          control={form.control}
          name="description"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Deskripsi</FormLabel>
              <FormControl>
                <textarea
                  {...field}
                  rows={4}
                  maxLength={1000}
                  placeholder="Deskripsi layanan..."
                  className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Submit */}
        <div className="flex items-center gap-3 pt-2">
          <Button type="submit" disabled={isPending}>
            {isPending && (
              <Loader2 size={14} className="animate-spin" aria-hidden="true" />
            )}
            {isEdit ? "Simpan Perubahan" : "Simpan & Buat"}
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
