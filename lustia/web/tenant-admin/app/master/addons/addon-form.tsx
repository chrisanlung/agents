"use client";

import { useTransition } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { useRouter } from "next/navigation";
import Link from "next/link";

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
import { createAddon, updateAddon } from "./actions";
import type { Addon } from "@/lib/types";

// ─── Zod schema ──────────────────────────────────────────────────────────────

const addonFormSchema = z.object({
  name: z
    .string()
    .min(1, "Nama add-on wajib diisi.")
    .max(120, "Nama maksimal 120 karakter."),
  description: z
    .string()
    .max(500, "Deskripsi maksimal 500 karakter.")
    .optional(),
  price_idr: z.coerce
    .number({ invalid_type_error: "Harga harus berupa angka." })
    .int("Harga harus bilangan bulat.")
    .min(0, "Harga tidak boleh negatif."),
  is_active: z.boolean(),
});

type AddonFormValues = z.infer<typeof addonFormSchema>;

// ─── Props ────────────────────────────────────────────────────────────────────

interface AddonFormProps {
  addon?: Addon;
}

// ─── Component ────────────────────────────────────────────────────────────────

export function AddonForm({ addon }: AddonFormProps) {
  const isEdit = !!addon;
  const [isPending, startTransition] = useTransition();
  const router = useRouter();

  const form = useForm<AddonFormValues>({
    resolver: zodResolver(addonFormSchema),
    defaultValues: {
      name: addon?.name ?? "",
      description: addon?.description ?? "",
      price_idr: addon?.price_idr ?? 0,
      is_active: addon?.is_active ?? true,
    },
  });

  function onSubmit(values: AddonFormValues) {
    startTransition(async () => {
      const fd = new FormData();
      fd.set("name", values.name);
      fd.set("description", values.description ?? "");
      fd.set("price_idr", String(values.price_idr));
      fd.set("is_active", String(values.is_active));

      const result = isEdit
        ? await updateAddon(addon!.id, fd)
        : await createAddon(fd);

      if (!result.ok) {
        Object.entries(result.errors).forEach(([field, message]) => {
          form.setError(field as keyof AddonFormValues, { message });
        });
        if (result.error) toast.error(result.error);
        return;
      }

      if (isEdit) {
        toast.success("Perubahan berhasil disimpan.");
        router.refresh();
      } else {
        toast.success("Add-on berhasil ditambahkan.");
        // redirect happens server-side via redirect() in createAddon
      }
    });
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
        {/* Nama Add-on */}
        <FormField
          control={form.control}
          name="name"
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                Nama Add-on <span className="text-destructive">*</span>
              </FormLabel>
              <FormControl>
                <Input
                  placeholder="Aromaterapi"
                  autoFocus
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

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
                  rows={3}
                  maxLength={500}
                  placeholder="Keterangan singkat yang ditampilkan kepada pelanggan."
                  className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                />
              </FormControl>
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
                    step={1}
                    placeholder="0"
                    {...field}
                  />
                </FormControl>
              </div>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Status (is_active) */}
        <FormField
          control={form.control}
          name="is_active"
          render={({ field }) => (
            <FormItem>
              <div className="flex items-center justify-between rounded-md border border-input bg-background px-3 py-2">
                <FormLabel className="cursor-pointer select-none text-sm font-medium text-foreground">
                  Aktif
                </FormLabel>
                <FormControl>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={field.value}
                    onClick={() => field.onChange(!field.value)}
                    className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50 ${
                      field.value ? "bg-primary" : "bg-input"
                    }`}
                  >
                    <span
                      className={`pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform ${
                        field.value ? "translate-x-4" : "translate-x-0"
                      }`}
                    />
                  </button>
                </FormControl>
              </div>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Button row */}
        <div className="flex items-center justify-between gap-3 pt-2">
          <Button
            type="button"
            variant="ghost"
            asChild
            disabled={isPending}
          >
            <Link href="/master/addons">Batal</Link>
          </Button>
          <Button type="submit" disabled={isPending}>
            {isPending && (
              <Loader2 size={14} className="animate-spin" aria-hidden="true" />
            )}
            {isEdit ? "Simpan Perubahan" : "Simpan & Buat Add-on"}
          </Button>
        </div>
      </form>
    </Form>
  );
}
