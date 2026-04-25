"use client";

import { useRef, useState, useTransition } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, UserRound } from "lucide-react";
import { toast } from "sonner";
import { useRouter } from "next/navigation";

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
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { createTherapist, updateTherapist } from "./actions";
import { uploadTherapistPhoto, deleteTherapistPhoto } from "./[id]/photo-actions";
import type { Therapist, Branch } from "@/lib/types";

// ─── Build display labels ─────────────────────────────────────────────────────

const BUILD_LABELS: Record<string, string> = {
  langsing: "Langsing",
  sedang: "Sedang",
  atletis: "Atletis",
  tegap: "Tegap",
};

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
  // ADR 0011 §2.3.2 — posture fields (required)
  height_cm: z.coerce
    .number({ invalid_type_error: "Tinggi wajib diisi." })
    .int("Masukkan angka bulat.")
    .min(100, "Tinggi minimal 100 cm.")
    .max(250, "Tinggi maksimal 250 cm."),
  weight_kg: z.coerce
    .number({ invalid_type_error: "Berat wajib diisi." })
    .int("Masukkan angka bulat.")
    .min(30, "Berat minimal 30 kg.")
    .max(250, "Berat maksimal 250 kg."),
  build: z.enum(["langsing", "sedang", "atletis", "tegap"], {
    required_error: "Postur wajib dipilih.",
    invalid_type_error: "Pilih salah satu postur.",
  }),
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

// ─── PhotoPicker (client-only sub-component) ─────────────────────────────────

interface PhotoPickerProps {
  therapistId: string | null; // null = /new page, upload disabled
  therapistName: string;
  initialPhotoUrl: string | null;
  onPhotoChange: (newUrl: string | null) => void;
}

function PhotoPicker({
  therapistId,
  therapistName,
  initialPhotoUrl,
  onPhotoChange,
}: PhotoPickerProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [photoUrl, setPhotoUrl] = useState<string | null>(initialPhotoUrl);
  const [isUploading, startUploadTransition] = useTransition();
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [photoJustUploaded, setPhotoJustUploaded] = useState(false);

  const isNew = therapistId === null;

  const initials = therapistName
    .split(" ")
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    // Reset error on each attempt
    setUploadError(null);

    // Client-side pre-validation (TEP-2)
    const allowedTypes = ["image/jpeg", "image/png", "image/webp"];
    if (!allowedTypes.includes(file.type)) {
      setUploadError("Format tidak didukung (JPEG, PNG, WebP).");
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      setUploadError("Ukuran melebihi 5 MB.");
      return;
    }

    if (!therapistId) return; // should not happen — input is disabled on /new

    const fd = new FormData();
    fd.set("photo", file);

    startUploadTransition(async () => {
      const result = await uploadTherapistPhoto(therapistId, fd);
      if (!result.ok) {
        setUploadError("Gagal mengunggah. Coba lagi.");
        return;
      }
      const newUrl = result.data.photo_url;
      setPhotoUrl(newUrl);
      onPhotoChange(newUrl);
      setPhotoJustUploaded(true);
    });

    // Reset input so the same file can be re-selected
    e.target.value = "";
  }

  function handleDeleteConfirm() {
    if (!therapistId) return;
    startUploadTransition(async () => {
      const result = await deleteTherapistPhoto(therapistId);
      if (!result.ok) {
        toast.error("Gagal menghapus foto. Coba lagi.");
        return;
      }
      setPhotoUrl(null);
      onPhotoChange(null);
      setPhotoJustUploaded(false);
    });
  }

  return (
    <div className="space-y-2">
      <div className="flex flex-col items-center gap-4 sm:flex-row sm:items-center">
        {/* Preview circle */}
        {photoUrl ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={photoUrl}
            alt={`Foto ${therapistName}`}
            className="h-32 w-32 shrink-0 rounded-full object-cover"
          />
        ) : (
          <div
            aria-label={therapistName || undefined}
            className="flex h-32 w-32 shrink-0 items-center justify-center rounded-full bg-primary/10 text-2xl font-semibold text-primary"
          >
            {initials || <UserRound size={32} aria-hidden="true" />}
          </div>
        )}

        {/* Action buttons */}
        <div className="flex flex-col items-center gap-2 sm:items-start">
          {/* Hidden file input — accessibility: associated label below via htmlFor */}
          <label htmlFor="photo-file-input" className="sr-only">
            Unggah foto terapis
          </label>
          <input
            id="photo-file-input"
            ref={fileInputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            className="hidden"
            aria-describedby={uploadError ? "photo-error" : undefined}
            disabled={isNew || isUploading}
            onChange={handleFileChange}
          />

          {isNew ? (
            /* /new page — upload disabled with tooltip-like hint */
            <div className="group relative">
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled
                title="Simpan terapis terlebih dahulu untuk mengunggah foto."
                aria-disabled="true"
              >
                Unggah Foto
              </Button>
              <p className="mt-1 text-xs text-muted-foreground">
                Simpan terapis terlebih dahulu untuk mengunggah foto.
              </p>
            </div>
          ) : (
            <>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={isUploading}
                onClick={() => fileInputRef.current?.click()}
              >
                {isUploading ? (
                  <>
                    <Loader2 size={14} className="animate-spin" aria-hidden="true" />
                    Foto sedang diunggah…
                  </>
                ) : photoUrl ? (
                  "Ganti Foto"
                ) : (
                  "Unggah Foto"
                )}
              </Button>

              {/* Hapus Foto — only when photo exists */}
              {photoUrl && (
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      disabled={isUploading}
                      className="text-destructive hover:text-destructive"
                    >
                      Hapus Foto
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>Hapus Foto?</AlertDialogTitle>
                      <AlertDialogDescription>
                        Foto profil terapis ini akan dihapus.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>Batal</AlertDialogCancel>
                      <AlertDialogAction
                        className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                        onClick={handleDeleteConfirm}
                      >
                        Ya, Hapus
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              )}
            </>
          )}

          <p className="text-xs text-muted-foreground">JPEG, PNG, WebP. Maks. 5 MB.</p>
        </div>
      </div>

      {/* Upload error — inline, role="alert" */}
      {uploadError && (
        <p id="photo-error" role="alert" className="text-sm text-destructive">
          {uploadError}
        </p>
      )}

      {/* Partial-save reminder (TEP-2 edge case) */}
      {photoJustUploaded && !isNew && (
        <p
          role="status"
          className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700"
        >
          Foto tersimpan. Simpan formulir untuk menerapkan perubahan lainnya.
        </p>
      )}
    </div>
  );
}

// ─── Main form component ──────────────────────────────────────────────────────

export function TherapistForm({
  therapist,
  branches = [],
  showBranchSelector = false,
}: TherapistFormProps) {
  const isEdit = !!therapist;
  const [isPending, startTransition] = useTransition();
  const router = useRouter();

  // Tracks current photo_url after upload (independent of RHF form state —
  // photo upload is a separate endpoint, not part of the profile PATCH).
  const [currentPhotoUrl, setCurrentPhotoUrl] = useState<string | null>(
    therapist?.photo_url ?? null
  );

  // Watch full_name to feed into PhotoPicker initials without re-rendering everything
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
      // ADR 0011: /new page uses blank — user must actively choose (micro-decision 3)
      height_cm: therapist?.height_cm ?? (undefined as unknown as number),
      weight_kg: therapist?.weight_kg ?? (undefined as unknown as number),
      build: therapist?.build ?? (undefined as unknown as "langsing"),
    },
  });

  const watchedName = form.watch("full_name");

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

        {/* ── Photo Picker (TEP-2) ──────────────────────────────────────── */}
        <PhotoPicker
          therapistId={therapist?.id ?? null}
          therapistName={watchedName}
          initialPhotoUrl={currentPhotoUrl}
          onPhotoChange={setCurrentPhotoUrl}
        />

        {/* ── Branch selector — tenant_admin on /new only ──────────────── */}
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

        {/* ── Nama Lengkap ─────────────────────────────────────────────── */}
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
          {/* Jenis Kelamin */}
          <FormField
            control={form.control}
            name="gender"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Jenis Kelamin</FormLabel>
                <Select
                  value={field.value ? field.value : undefined}
                  onValueChange={(v) => field.onChange(v === "_unset" ? "" : v)}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue placeholder="Pilih jenis kelamin" />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value="_unset">Tidak disebutkan</SelectItem>
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
                  <Input type="date" placeholder="dd/mm/yyyy" {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        {/* ── Bio ──────────────────────────────────────────────────────── */}
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

        {/* ── Informasi Postur (TEP-1, TEP-3, TEP-4) ───────────────────── */}
        <p className="mt-6 mb-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
          Informasi Postur
        </p>

        {/* Read-only posture summary — edit mode only (TEP-6) */}
        {isEdit && (
          <div className="rounded-md border bg-muted/30 px-3 py-2 text-sm text-foreground">
            Tinggi: {therapist.height_cm} cm &middot; Berat: {therapist.weight_kg} kg &middot; Postur:{" "}
            {BUILD_LABELS[therapist.build] ?? therapist.build}
          </div>
        )}

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          {/* Tinggi (cm) */}
          <FormField
            control={form.control}
            name="height_cm"
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  Tinggi (cm) <span className="text-destructive">*</span>
                </FormLabel>
                <div className="flex items-center">
                  <FormControl>
                    <Input
                      type="number"
                      inputMode="numeric"
                      min={100}
                      max={250}
                      step={1}
                      placeholder="167"
                      className="rounded-r-none"
                      {...field}
                    />
                  </FormControl>
                  <span
                    aria-hidden="true"
                    className="flex h-10 items-center rounded-r-md border border-l-0 border-input bg-muted/50 px-3 text-sm text-muted-foreground select-none"
                  >
                    cm
                  </span>
                </div>
                <FormMessage />
              </FormItem>
            )}
          />

          {/* Berat (kg) */}
          <FormField
            control={form.control}
            name="weight_kg"
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  Berat (kg) <span className="text-destructive">*</span>
                </FormLabel>
                <div className="flex items-center">
                  <FormControl>
                    <Input
                      type="number"
                      inputMode="numeric"
                      min={30}
                      max={250}
                      step={1}
                      placeholder="58"
                      className="rounded-r-none"
                      {...field}
                    />
                  </FormControl>
                  <span
                    aria-hidden="true"
                    className="flex h-10 items-center rounded-r-md border border-l-0 border-input bg-muted/50 px-3 text-sm text-muted-foreground select-none"
                  >
                    kg
                  </span>
                </div>
                <FormMessage />
              </FormItem>
            )}
          />

          {/* Postur */}
          <FormField
            control={form.control}
            name="build"
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  Postur <span className="text-destructive">*</span>
                </FormLabel>
                <Select value={field.value ?? ""} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue placeholder="Pilih postur" />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value="langsing">Langsing</SelectItem>
                    <SelectItem value="sedang">Sedang</SelectItem>
                    <SelectItem value="atletis">Atletis</SelectItem>
                    <SelectItem value="tegap">Tegap</SelectItem>
                  </SelectContent>
                </Select>
                <FormDescription>
                  Kategori yang akan ditampilkan ke pelanggan.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        {/* ── Read-only linked account ──────────────────────────────────── */}
        {isEdit && (
          <div className="rounded-md border bg-muted/30 px-3 py-2">
            <p className="text-xs text-muted-foreground">Akun terhubung</p>
            <p className="mt-0.5 text-sm text-foreground">
              {therapist?.user_id ? therapist.email ?? therapist.user_id : "Belum terhubung"}
            </p>
          </div>
        )}

        {/* ── Submit ───────────────────────────────────────────────────── */}
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
