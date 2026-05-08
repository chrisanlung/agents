"use client";

import { useActionState, useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  AlertTriangle,
  CheckCircle2,
  Eye,
  EyeOff,
  Loader2,
  LogIn,
  LogOut,
  ShieldCheck,
} from "lucide-react";

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
import { cn } from "@/lib/utils";
import {
  changePasswordAction,
  forcedLogoutAction,
  type ChangePasswordState,
} from "./actions";
import { PasswordStrength } from "./password-strength";

const schema = z
  .object({
    old_password: z.string().min(1, "Kata sandi saat ini wajib diisi"),
    new_password: z
      .string()
      .min(8, "Kata sandi baru minimal 8 karakter")
      .max(128, "Kata sandi baru maksimal 128 karakter"),
    confirm_password: z.string().min(1, "Konfirmasi kata sandi wajib diisi"),
  })
  .refine((v) => v.new_password === v.confirm_password, {
    path: ["confirm_password"],
    message: "Konfirmasi kata sandi tidak cocok",
  });

type FormValues = z.infer<typeof schema>;

const initialState: ChangePasswordState = { status: "idle" };

interface ChangePasswordFormProps {
  forced: boolean;
}

export function ChangePasswordForm({ forced }: ChangePasswordFormProps) {
  const router = useRouter();
  const [state, formAction, isPending] = useActionState(
    changePasswordAction,
    initialState
  );
  const [isLoggingOut, startLogout] = useTransition();

  const [showOld, setShowOld] = useState(false);
  const [showNew, setShowNew] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    mode: "onBlur",
    defaultValues: {
      old_password: "",
      new_password: "",
      confirm_password: "",
    },
  });

  const newPassword = form.watch("new_password");

  // Surface server-side field errors on form fields (e.g. wrong old password).
  useEffect(() => {
    if (state.status === "error" && state.fieldErrors) {
      for (const [field, messages] of Object.entries(state.fieldErrors)) {
        if (messages[0]) {
          form.setError(field as keyof FormValues, { message: messages[0] });
        }
      }
    }
  }, [state, form]);

  // After success: backend invalidated every session for this user. Auto-
  // redirect to login after a short delay so the user can see the confirmation.
  useEffect(() => {
    if (state.status === "success") {
      const t = setTimeout(() => router.replace("/login"), 2500);
      return () => clearTimeout(t);
    }
  }, [state, router]);

  if (state.status === "success") {
    return (
      <div className="space-y-5 text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-emerald-100 text-emerald-700">
          <CheckCircle2 size={28} aria-hidden="true" />
        </div>
        <div>
          <h2 className="text-lg font-semibold text-foreground">
            Kata sandi berhasil diubah
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Demi keamanan, semua sesi Anda telah dikeluarkan. Silakan masuk
            kembali menggunakan kata sandi baru.
          </p>
        </div>
        <Button className="w-full" onClick={() => router.replace("/login")}>
          <LogIn size={16} aria-hidden="true" />
          Masuk Kembali
        </Button>
      </div>
    );
  }

  function handleForcedLogout() {
    startLogout(async () => {
      await forcedLogoutAction();
      router.replace("/login");
    });
  }

  return (
    <Form {...form}>
      <form action={formAction} className="space-y-5" noValidate>
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

        <FormField
          control={form.control}
          name="old_password"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Kata Sandi Saat Ini</FormLabel>
              <div className="relative">
                <FormControl>
                  <Input
                    type={showOld ? "text" : "password"}
                    autoComplete="current-password"
                    autoFocus
                    placeholder="Masukkan kata sandi saat ini"
                    className="pr-10"
                    {...field}
                  />
                </FormControl>
                <EyeToggle
                  show={showOld}
                  onToggle={() => setShowOld((v) => !v)}
                  label="kata sandi saat ini"
                />
              </div>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="new_password"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Kata Sandi Baru</FormLabel>
              <div className="relative">
                <FormControl>
                  <Input
                    type={showNew ? "text" : "password"}
                    autoComplete="new-password"
                    placeholder="Minimal 8 karakter"
                    className="pr-10"
                    {...field}
                  />
                </FormControl>
                <EyeToggle
                  show={showNew}
                  onToggle={() => setShowNew((v) => !v)}
                  label="kata sandi baru"
                />
              </div>
              <PasswordStrength value={newPassword ?? ""} />
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="confirm_password"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Konfirmasi Kata Sandi Baru</FormLabel>
              <div className="relative">
                <FormControl>
                  <Input
                    type={showConfirm ? "text" : "password"}
                    autoComplete="new-password"
                    placeholder="Ulangi kata sandi baru"
                    className="pr-10"
                    {...field}
                  />
                </FormControl>
                <EyeToggle
                  show={showConfirm}
                  onToggle={() => setShowConfirm((v) => !v)}
                  label="konfirmasi kata sandi"
                />
              </div>
              <FormMessage />
            </FormItem>
          )}
        />

        <div className="flex flex-col gap-3 pt-2 sm:flex-row">
          <Button
            type="submit"
            className="flex-1"
            disabled={isPending}
            aria-busy={isPending}
          >
            {isPending ? (
              <>
                <Loader2 className="animate-spin" aria-hidden="true" />
                Menyimpan…
              </>
            ) : forced ? (
              <>
                <ShieldCheck size={16} aria-hidden="true" />
                Aktifkan Akun
              </>
            ) : (
              "Simpan Kata Sandi Baru"
            )}
          </Button>
          {!forced && (
            <Button
              type="button"
              variant="outline"
              onClick={() => router.back()}
              disabled={isPending}
            >
              Batal
            </Button>
          )}
        </div>

        {forced && (
          <div className="border-t pt-4 text-center">
            <button
              type="button"
              onClick={handleForcedLogout}
              disabled={isPending || isLoggingOut}
              className="inline-flex items-center gap-1.5 text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline disabled:opacity-50"
            >
              {isLoggingOut ? (
                <Loader2 size={14} className="animate-spin" aria-hidden="true" />
              ) : (
                <LogOut size={14} aria-hidden="true" />
              )}
              Atau, keluar dari akun ini
            </button>
          </div>
        )}
      </form>
    </Form>
  );
}

function EyeToggle({
  show,
  onToggle,
  label,
}: {
  show: boolean;
  onToggle: () => void;
  label: string;
}) {
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={show}
      aria-label={`${show ? "Sembunyikan" : "Tampilkan"} ${label}`}
      className={cn(
        "absolute right-2 top-1/2 -translate-y-1/2",
        "flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground",
        "hover:bg-muted hover:text-foreground"
      )}
    >
      {show ? (
        <EyeOff size={16} aria-hidden="true" />
      ) : (
        <Eye size={16} aria-hidden="true" />
      )}
    </button>
  );
}
