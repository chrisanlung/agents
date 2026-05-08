"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Loader2, AlertCircle } from "lucide-react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {
  Branch,
  Service,
  Addon,
  Therapist,
  AvailabilitySlot,
} from "@/lib/types";
import { createConciergeBooking } from "../actions";

const schema = z.object({
  branch_id: z.string().min(1, "Cabang wajib dipilih."),
  service_id: z.string().min(1, "Layanan wajib dipilih."),
  addon_ids: z.array(z.string()).default([]),
  room_id: z.string().optional(),
  therapist_id: z.string().optional(),
  scheduled_start: z.string().min(1, "Slot waktu wajib dipilih."),
  date: z.string().min(1, "Tanggal wajib diisi."),
  customer_name: z.string().min(1, "Nama pelanggan wajib diisi."),
  customer_phone: z.string().min(7, "Nomor WhatsApp tidak valid."),
  customer_email: z.string().email("Email tidak valid."),
});

type FormValues = z.infer<typeof schema>;

interface Props {
  branches: Branch[];
  services: Service[];
  addons: Addon[];
  therapists: Therapist[];
}

function formatPrice(idr: number) {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(idr);
}

function formatSlotLabel(iso: string) {
  return new Date(iso).toLocaleTimeString("id-ID", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

export function ConciergeBookingForm({
  branches,
  services,
  addons,
  therapists,
}: Props) {
  const router = useRouter();
  const [slots, setSlots] = useState<AvailabilitySlot[]>([]);
  const [loadingSlots, setLoadingSlots] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const {
    register,
    control,
    watch,
    handleSubmit,
    setValue,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      // Auto-fill when caller has access to exactly one branch (staff scope).
      // For multi-branch admins (tenant admin), the dropdown remains.
      branch_id: branches.length === 1 ? branches[0].id : "",
      addon_ids: [],
      date: new Date().toISOString().slice(0, 10),
    },
  });

  const branchId = watch("branch_id");
  const serviceId = watch("service_id");
  const date = watch("date");
  const addonIds = watch("addon_ids");

  // Load slots when service + date change
  useEffect(() => {
    if (!branchId || !serviceId || !date) {
      setSlots([]);
      return;
    }

    let cancelled = false;
    setLoadingSlots(true);
    setValue("scheduled_start", "");

    fetch(
      `/api/v1/public/branches/${branchId}/availability?service_id=${serviceId}&date=${date}`,
      { cache: "no-store" }
    )
      .then((r) => r.json())
      .then((data) => {
        if (!cancelled) {
          setSlots(Array.isArray(data) ? data : (data.slots ?? []));
        }
      })
      .catch(() => {
        if (!cancelled) setSlots([]);
      })
      .finally(() => {
        if (!cancelled) setLoadingSlots(false);
      });

    return () => {
      cancelled = true;
    };
  }, [branchId, serviceId, date, setValue]);

  // Compute price summary
  const selectedService = services.find((s) => s.id === serviceId);
  const selectedAddons = addons.filter((a) => addonIds.includes(a.id));
  const total =
    (selectedService?.price_idr ?? 0) +
    selectedAddons.reduce((sum, a) => sum + a.price_idr, 0);

  async function onSubmit(values: FormValues) {
    setServerError(null);
    setIsSubmitting(true);

    const fd = new FormData();
    fd.set("branch_id", values.branch_id);
    fd.set("service_id", values.service_id);
    fd.set("addon_ids", JSON.stringify(values.addon_ids));
    fd.set("room_id", values.room_id ?? "");
    fd.set("therapist_id", values.therapist_id ?? "");
    fd.set("scheduled_start", values.scheduled_start);
    fd.set("customer_name", values.customer_name);
    fd.set("customer_phone", values.customer_phone);
    fd.set("customer_email", values.customer_email);

    const result = await createConciergeBooking(fd);
    setIsSubmitting(false);

    if (!result.ok) {
      setServerError(result.error ?? "Terjadi kesalahan. Coba lagi.");
      return;
    }

    toast.success("Booking berhasil dibuat.");
    router.push(`/booking/${result.bookingId}`);
  }

  // Branch-scoped therapists
  const branchTherapists = therapists.filter(
    (t) => !branchId || t.branch_id === branchId
  );

  return (
    <form onSubmit={handleSubmit(onSubmit)} noValidate>
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Left column */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Detail Booking</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {/* Branch — when the caller has exactly one branch (staff scope),
                show it as a read-only label instead of a dropdown. The
                branch_id is already pre-filled in defaultValues so the form
                still submits correctly. */}
            {branches.length === 1 ? (
              <div className="space-y-1.5">
                <Label>Cabang</Label>
                <div className="rounded-md border bg-muted/40 px-3 py-2 text-sm">
                  {branches[0].name}
                </div>
                <input type="hidden" name="branch_id" value={branches[0].id} />
              </div>
            ) : (
              <div className="space-y-1.5">
                <Label htmlFor="branch_id">
                  Cabang <span className="text-destructive">*</span>
                </Label>
                <Controller
                  control={control}
                  name="branch_id"
                  render={({ field }) => (
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                    >
                      <SelectTrigger id="branch_id">
                        <SelectValue placeholder="Pilih cabang…" />
                      </SelectTrigger>
                      <SelectContent>
                        {branches.map((b) => (
                          <SelectItem key={b.id} value={b.id}>
                            {b.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                />
                {errors.branch_id && (
                  <p className="text-xs text-destructive">
                    {errors.branch_id.message}
                  </p>
                )}
              </div>
            )}

            {/* Service */}
            <div className="space-y-1.5">
              <Label htmlFor="service_id">
                Layanan <span className="text-destructive">*</span>
              </Label>
              <Controller
                control={control}
                name="service_id"
                render={({ field }) => (
                  <Select
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    <SelectTrigger id="service_id">
                      <SelectValue placeholder="Pilih layanan…" />
                    </SelectTrigger>
                    <SelectContent>
                      {services.map((s) => (
                        <SelectItem key={s.id} value={s.id}>
                          {s.name} — {formatPrice(s.price_idr)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
              {errors.service_id && (
                <p className="text-xs text-destructive">
                  {errors.service_id.message}
                </p>
              )}
            </div>

            {/* Add-ons */}
            {addons.length > 0 && (
              <div className="space-y-1.5">
                <Label>Tambahan (opsional)</Label>
                <div className="space-y-1.5 rounded-md border border-input p-2">
                  {addons.map((a) => (
                    <label
                      key={a.id}
                      className="flex cursor-pointer items-center gap-2 text-sm"
                    >
                      <input
                        type="checkbox"
                        value={a.id}
                        checked={addonIds.includes(a.id)}
                        onChange={(e) => {
                          const cur = addonIds;
                          if (e.target.checked) {
                            setValue("addon_ids", [...cur, a.id]);
                          } else {
                            setValue(
                              "addon_ids",
                              cur.filter((id) => id !== a.id)
                            );
                          }
                        }}
                        className="rounded border-input"
                      />
                      {a.name}{" "}
                      <span className="text-muted-foreground">
                        +{formatPrice(a.price_idr)}
                      </span>
                    </label>
                  ))}
                </div>
              </div>
            )}

            {/* Date */}
            <div className="space-y-1.5">
              <Label htmlFor="date">
                Tanggal <span className="text-destructive">*</span>
              </Label>
              <Input
                id="date"
                type="date"
                min={new Date().toISOString().slice(0, 10)}
                {...register("date")}
              />
              {errors.date && (
                <p className="text-xs text-destructive">
                  {errors.date.message}
                </p>
              )}
            </div>

            {/* Slot */}
            <div className="space-y-1.5">
              <Label htmlFor="scheduled_start">
                Slot Waktu <span className="text-destructive">*</span>
              </Label>
              {loadingSlots ? (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 size={14} className="animate-spin" aria-hidden="true" />
                  Memuat slot…
                </div>
              ) : (
                <Controller
                  control={control}
                  name="scheduled_start"
                  render={({ field }) => (
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                      disabled={slots.length === 0}
                    >
                      <SelectTrigger id="scheduled_start">
                        <SelectValue
                          placeholder={
                            slots.length === 0
                              ? "Tidak ada slot tersedia."
                              : "Pilih slot waktu…"
                          }
                        />
                      </SelectTrigger>
                      <SelectContent>
                        {slots.map((slot) => (
                          <SelectItem
                            key={slot.start}
                            value={slot.start}
                          >
                            {formatSlotLabel(slot.start)} —{" "}
                            {formatSlotLabel(slot.end)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                />
              )}
              {errors.scheduled_start && (
                <p className="text-xs text-destructive">
                  {errors.scheduled_start.message}
                </p>
              )}
            </div>

            {/* Therapist */}
            <div className="space-y-1.5">
              <Label htmlFor="therapist_id">
                Terapis{" "}
                <span className="text-muted-foreground">(opsional)</span>
              </Label>
              <Controller
                control={control}
                name="therapist_id"
                render={({ field }) => (
                  <Select
                    value={field.value ?? ""}
                    onValueChange={(v) =>
                      field.onChange(v === "auto" ? undefined : v)
                    }
                  >
                    <SelectTrigger id="therapist_id">
                      <SelectValue placeholder="Pilih otomatis" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="auto">Pilih otomatis</SelectItem>
                      {branchTherapists.map((t) => (
                        <SelectItem key={t.id} value={t.id}>
                          {t.full_name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
            </div>
          </CardContent>
        </Card>

        {/* Right column */}
        <div className="space-y-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Info Pelanggan</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="customer_name">
                  Nama Lengkap <span className="text-destructive">*</span>
                </Label>
                <Input
                  id="customer_name"
                  {...register("customer_name")}
                  placeholder="Nama lengkap pelanggan"
                />
                {errors.customer_name && (
                  <p className="text-xs text-destructive">
                    {errors.customer_name.message}
                  </p>
                )}
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="customer_phone">
                  No. WhatsApp <span className="text-destructive">*</span>
                </Label>
                <Input
                  id="customer_phone"
                  type="tel"
                  {...register("customer_phone")}
                  placeholder="081234567890"
                />
                {errors.customer_phone && (
                  <p className="text-xs text-destructive">
                    {errors.customer_phone.message}
                  </p>
                )}
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="customer_email">
                  Email <span className="text-destructive">*</span>
                </Label>
                <Input
                  id="customer_email"
                  type="email"
                  {...register("customer_email")}
                  placeholder="email@contoh.com"
                />
                {errors.customer_email && (
                  <p className="text-xs text-destructive">
                    {errors.customer_email.message}
                  </p>
                )}
              </div>
            </CardContent>
          </Card>

          {/* Price summary */}
          <Card className="bg-muted/30">
            <CardContent className="pt-4">
              <div className="space-y-1.5 text-sm">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Layanan:</span>
                  <span>
                    {selectedService
                      ? formatPrice(selectedService.price_idr)
                      : "—"}
                  </span>
                </div>
                {selectedAddons.length > 0 && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Tambahan:</span>
                    <span>
                      {formatPrice(
                        selectedAddons.reduce(
                          (sum, a) => sum + a.price_idr,
                          0
                        )
                      )}
                    </span>
                  </div>
                )}
                <div className="my-1 border-t border-border" />
                <div className="flex justify-between font-semibold text-emerald-700">
                  <span>Total:</span>
                  <span>{formatPrice(total)}</span>
                </div>
                <p className="text-xs text-muted-foreground">
                  Metode: Bayar di Tempat
                </p>
              </div>
            </CardContent>
          </Card>

          {/* Server error */}
          {serverError && (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800"
            >
              <AlertCircle
                size={16}
                className="mt-0.5 shrink-0 text-red-600"
                aria-hidden="true"
              />
              <span>{serverError}</span>
            </div>
          )}

          {/* Submit row */}
          <div className="flex gap-2">
            <Button
              type="button"
              variant="ghost"
              onClick={() => router.push("/booking")}
            >
              Batal
            </Button>
            <Button
              type="submit"
              className="flex-1"
              disabled={isSubmitting}
            >
              {isSubmitting ? (
                <Loader2
                  size={16}
                  className="animate-spin mr-1"
                  aria-hidden="true"
                />
              ) : null}
              Buat Booking
            </Button>
          </div>
        </div>
      </div>
    </form>
  );
}
