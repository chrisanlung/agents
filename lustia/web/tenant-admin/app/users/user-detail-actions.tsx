"use client";

import { useTransition, useState } from "react";
import { LockOpen, Loader2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from "@/components/ui/alert-dialog";
import { unlockUser } from "./actions";

interface UserDetailActionsProps {
  userId: string;
  userName: string;
  isLocked: boolean;
}

export function UserDetailActions({
  userId,
  userName,
  isLocked,
}: UserDetailActionsProps) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const [unlockOpen, setUnlockOpen] = useState(false);

  function handleUnlock() {
    setUnlockOpen(false);
    startTransition(async () => {
      const result = await unlockUser(userId);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success(`Akun "${userName}" berhasil dibuka kuncinya.`);
      router.refresh();
    });
  }

  if (!isLocked) return null;

  return (
    <>
      <div
        role="alert"
        className="flex items-center justify-between gap-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3"
      >
        <p className="text-sm text-amber-800">
          Akun ini terkunci karena terlalu banyak percobaan login yang gagal.
        </p>
        <Button
          variant="outline"
          size="sm"
          disabled={isPending}
          onClick={() => setUnlockOpen(true)}
          className="shrink-0 border-amber-300 text-amber-800 hover:bg-amber-100"
        >
          {isPending ? (
            <Loader2 size={14} className="mr-2 animate-spin" aria-hidden="true" />
          ) : (
            <LockOpen size={14} className="mr-2" aria-hidden="true" />
          )}
          Buka Kunci Akun
        </Button>
      </div>

      <AlertDialog open={unlockOpen} onOpenChange={setUnlockOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Buka Kunci Akun</AlertDialogTitle>
            <AlertDialogDescription>
              Akun &quot;{userName}&quot; terkunci. Membuka kunci akan
              mengizinkan pengguna login kembali.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction onClick={handleUnlock}>
              Buka Kunci
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
