"use client";

import { useRef, useState, useTransition, KeyboardEvent } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { DoorOpen, Loader2, X } from "lucide-react";
import { toast } from "sonner";
import { useRouter } from "next/navigation";
import Link from "next/link";

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
import { Badge } from "@/components/ui/badge";
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
import { createRoom, updateRoom } from "./actions";
import { uploadRoomPhoto, removeRoomPhoto } from "./[id]/photo-actions";
import type { Room, Branch, RoomType } from "@/lib/types";

// ─── Room type display labels ─────────────────────────────────────────────────

export const ROOM_TYPE_LABELS: Record<RoomType, string> = {
  single: "Single",
  couple: "Couple",
  group: "Grup",
  vip: "VIP",
};

// ─── Zod schema ───────────────────────────────────────────────────────────────

const roomFormSchema = z.object({
  branch_id: z.string().min(1, "Cabang wajib dipilih."),
  name: z
    .string()
    .min(1, "Nama ruangan wajib diisi.")
    .max(120, "Nama maksimal 120 karakter."),
  description: z
    .string()
    .max(500, "Deskripsi maksimal 500 karakter.")
    .optional(),
  room_type: z.enum(["single", "couple", "group", "vip"] as const, {
    required_error: "Tipe ruangan wajib dipilih.",
  }),
  capacity: z.coerce
    .number({ invalid_type_error: "Kapasitas harus berupa angka." })
    .int("Kapasitas harus bilangan bulat.")
    .min(1, "Kapasitas minimal 1.")
    .max(20, "Kapasitas maksimal 20."),
  amenities: z
    .array(z.string().max(80, "Fasilitas maksimal 80 karakter."))
    .max(20, "Fasilitas maksimal 20 item."),
  is_active: z.boolean(),
});

type RoomFormValues = z.infer<typeof roomFormSchema>;

// ─── Props ────────────────────────────────────────────────────────────────────

interface RoomFormProps {
  room?: Room;
  branches?: Branch[];
}

// ─── Photo picker (edit-only) ─────────────────────────────────────────────────

interface RoomPhotoPickerProps {
  roomId: string;
  roomName: string;
  initialPhotoUrl: string | null;
  onPhotoChange: (newUrl: string | null) => void;
}

function RoomPhotoPicker({
  roomId,
  roomName,
  initialPhotoUrl,
  onPhotoChange,
}: RoomPhotoPickerProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [photoUrl, setPhotoUrl] = useState<string | null>(initialPhotoUrl);
  const [isUploading, startUploadTransition] = useTransition();
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [photoJustUploaded, setPhotoJustUploaded] = useState(false);

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;

    setUploadError(null);

    const allowedTypes = ["image/jpeg", "image/png", "image/webp"];
    if (!allowedTypes.includes(file.type)) {
      setUploadError("Format tidak didukung (JPEG, PNG, WebP).");
      return;
    }
    if (file.size > 5 * 1024 * 1024) {
      setUploadError("Ukuran melebihi 5 MB.");
      return;
    }

    const fd = new FormData();
    fd.set("photo", file);

    startUploadTransition(async () => {
      const result = await uploadRoomPhoto(roomId, fd);
      if (!result.ok) {
        setUploadError(result.error);
        return;
      }
      const newUrl = result.data.photo_url;
      setPhotoUrl(newUrl);
      onPhotoChange(newUrl);
      setPhotoJustUploaded(true);
    });

    e.target.value = "";
  }

  function handleDeleteConfirm() {
    startUploadTransition(async () => {
      const result = await removeRoomPhoto(roomId);
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
      <div className="flex flex-col items-center gap-4 sm:flex-row sm:items-start">
        {/* Preview — rounded-md (rooms are spaces, not people — RM-8) */}
        {photoUrl ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={photoUrl}
            alt={`Foto ruangan ${roomName}`}
            className="h-32 w-32 shrink-0 rounded-md object-cover"
          />
        ) : (
          <div
            aria-label="Belum ada foto"
            className="flex h-32 w-32 shrink-0 items-center justify-center rounded-md bg-muted/40"
          >
            <DoorOpen size={32} className="text-muted-foreground/50" aria-hidden="true" />
          </div>
        )}

        {/* Action buttons */}
        <div className="flex flex-col items-center gap-2 sm:items-start">
          <label htmlFor="room-photo-upload" className="sr-only">
            Unggah foto ruangan
          </label>
          <input
            id="room-photo-upload"
            ref={fileInputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            className="hidden"
            aria-describedby={
              uploadError
                ? "room-photo-upload-error"
                : "room-photo-upload-hint"
            }
            disabled={isUploading}
            onChange={handleFileChange}
          />

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
                    Foto ruangan ini akan dihapus.
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

          <p id="room-photo-upload-hint" className="text-xs text-muted-foreground">
            JPEG, PNG, WebP. Maks. 5 MB.
          </p>
        </div>
      </div>

      {uploadError && (
        <p id="room-photo-upload-error" role="alert" className="text-sm text-destructive">
          {uploadError}
        </p>
      )}

      {photoJustUploaded && (
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

// ─── Amenity tag input ────────────────────────────────────────────────────────

interface AmenityTagInputProps {
  value: string[];
  onChange: (tags: string[]) => void;
  error?: string;
}

function AmenityTagInput({ value, onChange, error }: AmenityTagInputProps) {
  const [inputValue, setInputValue] = useState("");

  function addTag(raw: string) {
    const tag = raw.trim().toLowerCase();
    if (!tag) return;
    if (tag.length > 80) return; // hard cap per schema
    if (value.includes(tag)) return; // dedup
    if (value.length >= 20) return; // max 20
    onChange([...value, tag]);
    setInputValue("");
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      addTag(inputValue);
    } else if (e.key === "Backspace" && inputValue === "" && value.length > 0) {
      onChange(value.slice(0, -1));
    }
  }

  function handleBlur() {
    if (inputValue.trim()) {
      addTag(inputValue);
    }
  }

  function removeTag(tag: string) {
    onChange(value.filter((t) => t !== tag));
  }

  return (
    <div className="space-y-2">
      <div
        className={`flex min-h-[40px] flex-wrap gap-1.5 rounded-md border bg-background px-3 py-2 text-sm focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 ${
          error ? "border-destructive" : "border-input"
        }`}
      >
        {value.map((tag) => (
          <Badge key={tag} variant="secondary" className="gap-1 pr-1 text-xs">
            {tag}
            <button
              type="button"
              aria-label={`Hapus fasilitas: ${tag}`}
              onClick={() => removeTag(tag)}
              className="rounded-sm opacity-60 hover:opacity-100"
            >
              <X size={10} aria-hidden="true" />
            </button>
          </Badge>
        ))}
        <input
          id="amenities-input"
          type="text"
          aria-label="Tambah fasilitas"
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={handleKeyDown}
          onBlur={handleBlur}
          placeholder={value.length === 0 ? "shower, tv, aromaterapi…" : ""}
          className="min-w-[120px] flex-1 bg-transparent outline-none placeholder:text-muted-foreground"
          disabled={value.length >= 20}
        />
      </div>
      <p className="text-xs text-muted-foreground">
        Ketik lalu tekan Enter atau koma untuk menambahkan. Maks. 20 item.
      </p>
    </div>
  );
}

// ─── Main form component ──────────────────────────────────────────────────────

export function RoomForm({ room, branches = [] }: RoomFormProps) {
  const isEdit = !!room;
  const [isPending, startTransition] = useTransition();
  const router = useRouter();

  const [currentPhotoUrl, setCurrentPhotoUrl] = useState<string | null>(
    room?.photo_url ?? null
  );

  const form = useForm<RoomFormValues>({
    resolver: zodResolver(roomFormSchema),
    defaultValues: {
      branch_id: room?.branch_id ?? "",
      name: room?.name ?? "",
      description: room?.description ?? "",
      room_type: room?.room_type ?? ("single" as RoomType),
      capacity: room?.capacity ?? 1,
      amenities: room?.amenities ?? [],
      is_active: room?.is_active ?? true,
    },
  });

  const watchedName = form.watch("name");

  function onSubmit(values: RoomFormValues) {
    startTransition(async () => {
      const fd = new FormData();
      fd.set("branch_id", values.branch_id);
      fd.set("name", values.name);
      fd.set("description", values.description ?? "");
      fd.set("room_type", values.room_type);
      fd.set("capacity", String(values.capacity));
      fd.set("amenities", JSON.stringify(values.amenities));
      fd.set("is_active", String(values.is_active));

      const result = isEdit
        ? await updateRoom(room!.id, fd)
        : await createRoom(fd);

      if (!result.ok) {
        Object.entries(result.errors).forEach(([field, message]) => {
          form.setError(field as keyof RoomFormValues, { message });
        });
        if (result.error) toast.error(result.error);
        return;
      }

      if (isEdit) {
        toast.success("Perubahan berhasil disimpan.");
        router.refresh();
      } else {
        toast.success("Ruangan berhasil ditambahkan.");
        // redirect happens server-side via redirect() in createRoom
      }
    });
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">

        {/* ── Photo Picker (edit only) ──────────────────────────────────── */}
        {isEdit && (
          <RoomPhotoPicker
            roomId={room!.id}
            roomName={watchedName}
            initialPhotoUrl={currentPhotoUrl}
            onPhotoChange={setCurrentPhotoUrl}
          />
        )}

        {/* ── Cabang ────────────────────────────────────────────────────── */}
        <FormField
          control={form.control}
          name="branch_id"
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                Cabang <span className="text-destructive">*</span>
              </FormLabel>
              {isEdit ? (
                <>
                  <div className="rounded-md border border-input bg-muted/30 px-3 py-2 text-sm text-foreground">
                    {branches.find((b) => b.id === field.value)?.name ?? field.value}
                  </div>
                  <FormDescription>
                    Cabang ruangan tidak dapat diubah setelah ruangan dibuat.
                  </FormDescription>
                </>
              ) : (
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
              )}
              <FormMessage />
            </FormItem>
          )}
        />

        {/* ── Nama Ruangan ──────────────────────────────────────────────── */}
        <FormField
          control={form.control}
          name="name"
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                Nama Ruangan <span className="text-destructive">*</span>
              </FormLabel>
              <FormControl>
                <Input placeholder="VIP 1" autoFocus={!isEdit} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* ── Tipe dan Kapasitas ──────────────────────────────────────── */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField
            control={form.control}
            name="room_type"
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  Tipe Ruangan <span className="text-destructive">*</span>
                </FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue placeholder="Pilih tipe" />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value="single">Single</SelectItem>
                    <SelectItem value="couple">Couple</SelectItem>
                    <SelectItem value="group">Grup</SelectItem>
                    <SelectItem value="vip">VIP</SelectItem>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name="capacity"
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  Kapasitas <span className="text-destructive">*</span>
                </FormLabel>
                <FormControl>
                  <Input
                    type="number"
                    inputMode="numeric"
                    min={1}
                    max={20}
                    step={1}
                    placeholder="1"
                    {...field}
                  />
                </FormControl>
                <FormDescription>Maks. orang serentak (1–20).</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        {/* ── Fasilitas (amenities) ──────────────────────────────────── */}
        <FormField
          control={form.control}
          name="amenities"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Fasilitas</FormLabel>
              <FormControl>
                <AmenityTagInput
                  value={field.value}
                  onChange={field.onChange}
                  error={form.formState.errors.amenities?.message}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* ── Deskripsi ──────────────────────────────────────────────── */}
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

        {/* ── Status (is_active) ─────────────────────────────────────── */}
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

        {/* ── Button row ─────────────────────────────────────────────── */}
        <div className="flex items-center justify-between gap-3 pt-2">
          <Button type="button" variant="ghost" asChild disabled={isPending}>
            <Link href="/master/rooms">Batal</Link>
          </Button>
          <Button type="submit" disabled={isPending}>
            {isPending && (
              <Loader2 size={14} className="animate-spin" aria-hidden="true" />
            )}
            {isEdit ? "Simpan Perubahan" : "Simpan & Buat Ruangan"}
          </Button>
        </div>
      </form>
    </Form>
  );
}
