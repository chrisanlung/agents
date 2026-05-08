"use client";

import { useActionState, useEffect, useTransition } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { AlertTriangle, Loader2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import {
  updateProfileAction,
  type UpdateProfileState,
  type UserProfileResponse,
} from "./actions";

const schema = z.object({
  full_name: z
    .string()
    .trim()
    .min(1, "Nama wajib diisi")
    .max(200, "Nama terlalu panjang"),
  phone: z
    .string()
    .regex(/^\d{5,30}$/, "Nomor telepon harus 5–30 digit angka")
    .optional()
    .or(z.literal("")),
});

type FormValues = z.infer<typeof schema>;

const initialState: UpdateProfileState = { status: "idle" };

interface ProfileFormProps {
  user: UserProfileResponse;
}

/** Derive at most 2 initials from a full name. */
function getInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].charAt(0).toUpperCase();
  return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase();
}

export function ProfileForm({ user }: ProfileFormProps) {
  const router = useRouter();
  const [state, formAction] = useActionState(updateProfileAction, initialState);
  const [isPending, startTransition] = useTransition();

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    mode: "onBlur",
    defaultValues: {
      full_name: user.full_name,
      phone: user.phone ?? "",
    },
  });

  const { isDirty } = form.formState;

  // Surface server-side field errors.
  useEffect(() => {
    if (state.status === "error" && state.fieldErrors) {
      for (const [field, messages] of Object.entries(state.fieldErrors)) {
        if (messages[0]) {
          form.setError(field as keyof FormValues, { message: messages[0] });
        }
      }
    }
  }, [state, form]);

  // On success: reset form to new values (clears isDirty) and show toast.
  useEffect(() => {
    if (state.status === "success") {
      form.reset({
        full_name: state.user.full_name,
        phone: state.user.phone ?? "",
      });
      toast.success("Profil berhasil diperbarui.");
    }
  }, [state, form]);

  const initials = getInitials(user.full_name);

  function handleSubmit(values: FormValues) {
    const fd = new FormData();
    fd.set("full_name", values.full_name);
    fd.set("phone", values.phone ?? "");
    startTransition(() => formAction(fd));
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(handleSubmit)}
        className="space-y-6"
        noValidate
      >
        {/* Generic server error alert */}
        {state.status === "error" && state.formError && (
          <div
            role="alert"
            aria-live="assertive"
            className="flex items-start gap-3 rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive"
          >
            <AlertTriangle
              size={16}
              className="mt-0.5 shrink-0"
              aria-hidden="true"
            />
            <span>{state.formError}</span>
          </div>
        )}

        {/* Avatar section (v1 — read-only) */}
        <div className="flex items-center gap-4">
          <div
            className="flex h-16 w-16 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary text-xl font-semibold ring-2 ring-border"
            aria-hidden="true"
          >
            {initials}
          </div>
          <div>
            <p className="text-sm font-medium text-foreground">
              {user.full_name}
            </p>
            <p className="mt-1 text-xs text-muted-foreground">
              Perubahan foto profil segera hadir.
            </p>
          </div>
        </div>

        <hr className="border-border" />

        {/* Email — read-only */}
        <div className="space-y-1.5">
          <label className="text-sm font-medium text-foreground">Email</label>
          <div className="flex h-10 items-center rounded-md border bg-muted px-3 text-sm text-muted-foreground">
            {user.email}
          </div>
          <p className="text-xs text-muted-foreground">
            Email tidak dapat diubah setelah akun dibuat.
          </p>
        </div>

        {/* Nama Lengkap */}
        <FormField
          control={form.control}
          name="full_name"
          render={({ field }) => (
            <FormItem>
              <FormLabel>
                Nama Lengkap
                <span className="ml-1 text-destructive" aria-hidden="true">
                  *
                </span>
              </FormLabel>
              <FormControl>
                <Input
                  type="text"
                  placeholder="Nama lengkap Anda"
                  autoComplete="name"
                  maxLength={200}
                  required
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Nomor Telepon */}
        <FormField
          control={form.control}
          name="phone"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Nomor Telepon</FormLabel>
              <FormControl>
                <Input
                  type="tel"
                  placeholder="Contoh: 08123456789"
                  autoComplete="tel"
                  inputMode="numeric"
                  maxLength={30}
                  {...field}
                  onChange={(e) => {
                    // Digit-only filter
                    field.onChange(e.target.value.replace(/\D/g, ""));
                  }}
                />
              </FormControl>
              <p className="text-xs text-muted-foreground">
                Hanya angka (5–30 digit)
              </p>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Action buttons */}
        <div className="flex flex-col-reverse gap-3 pt-2 sm:flex-row sm:justify-end">
          <Button
            type="button"
            variant="outline"
            onClick={() => router.push("/dashboard")}
            disabled={isPending}
          >
            Batal
          </Button>
          <Button
            type="submit"
            disabled={isPending || !isDirty}
            aria-busy={isPending}
          >
            {isPending ? (
              <>
                <Loader2
                  size={16}
                  className="mr-2 animate-spin"
                  aria-hidden="true"
                />
                Menyimpan…
              </>
            ) : (
              "Simpan Perubahan"
            )}
          </Button>
        </div>
      </form>
    </Form>
  );
}
