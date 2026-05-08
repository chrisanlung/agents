"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Copy, Check, AlertTriangle } from "lucide-react";
import { toast } from "sonner";

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

interface InitialPasswordModalProps {
  email: string;
  username: string | null;
  initialPassword: string;
  onClose: () => void;
}

export function InitialPasswordModal({
  email,
  username,
  initialPassword,
  onClose,
}: InitialPasswordModalProps) {
  const router = useRouter();
  const [copied, setCopied] = useState(false);

  async function handleCopyCredentials() {
    try {
      const lines = [`Email: ${email}`];
      if (username) lines.push(`Username: ${username}`);
      lines.push(`Password: ${initialPassword}`);
      await navigator.clipboard.writeText(lines.join("\n"));
      setCopied(true);
      toast.success("Kredensial disalin ke clipboard.");
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Gagal menyalin. Salin secara manual.");
    }
  }

  function handleClose() {
    onClose();
    router.push("/users");
    router.refresh();
  }

  return (
    <Dialog open onOpenChange={(open) => { if (!open) handleClose(); }}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Pengguna berhasil dibuat</DialogTitle>
          <DialogDescription>
            Simpan atau kirimkan kredensial berikut ke pengguna sebelum menutup
            dialog ini.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Warning banner */}
          <div
            role="alert"
            className="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800"
          >
            <AlertTriangle
              size={16}
              className="mt-0.5 shrink-0 text-amber-600"
              aria-hidden="true"
            />
            <span>
              Password ini hanya muncul satu kali. Pastikan Anda
              menyimpan/mengirimkannya ke pengguna sebelum menutup.
            </span>
          </div>

          {/* Credentials display */}
          <div className="rounded-lg border bg-muted/50 p-4 space-y-2">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Email
              </span>
              <span className="text-sm font-medium select-all">{email}</span>
            </div>
            {username && (
              <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  Username
                </span>
                <code className="rounded bg-background px-2 py-1 text-sm font-mono font-semibold select-all border">
                  {username}
                </code>
              </div>
            )}
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Password
              </span>
              <code className="rounded bg-background px-2 py-1 text-sm font-mono font-semibold select-all border">
                {initialPassword}
              </code>
            </div>
          </div>
          {username && (
            <p className="text-xs text-muted-foreground">
              Staff dapat login menggunakan email atau username:{" "}
              <code className="font-mono font-semibold">{username}</code>
            </p>
          )}
        </div>

        <DialogFooter className="flex-col gap-2 sm:flex-row">
          <Button
            variant="outline"
            onClick={handleCopyCredentials}
            className="w-full sm:w-auto"
          >
            {copied ? (
              <Check size={14} className="mr-2" aria-hidden="true" />
            ) : (
              <Copy size={14} className="mr-2" aria-hidden="true" />
            )}
            {copied ? "Disalin!" : "Salin Kredensial"}
          </Button>
          <Button onClick={handleClose} className="w-full sm:w-auto">
            Tutup &amp; ke Daftar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
