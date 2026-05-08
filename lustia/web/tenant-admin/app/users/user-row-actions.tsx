"use client";

import { useTransition, useState } from "react";
import { MoreHorizontal, Loader2, Pencil, LockOpen } from "lucide-react";
import Link from "next/link";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
import { toggleUserStatus, unlockUser } from "./actions";
import type { User } from "@/lib/types";

interface UserRowActionsProps {
  user: User;
}

export function UserRowActions({ user }: UserRowActionsProps) {
  const [isPending, startTransition] = useTransition();
  const [deactivateOpen, setDeactivateOpen] = useState(false);
  const [unlockOpen, setUnlockOpen] = useState(false);

  const now = new Date();
  const isLocked =
    user.locked_until != null && new Date(user.locked_until) > now;

  function handleToggleStatus() {
    setDeactivateOpen(false);
    startTransition(async () => {
      const result = await toggleUserStatus(user.id, !user.is_active);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success(
        user.is_active
          ? `Pengguna "${user.full_name}" dinonaktifkan.`
          : `Pengguna "${user.full_name}" diaktifkan.`
      );
    });
  }

  function handleUnlock() {
    setUnlockOpen(false);
    startTransition(async () => {
      const result = await unlockUser(user.id);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success(`Akun "${user.full_name}" berhasil dibuka kuncinya.`);
    });
  }

  return (
    <>
      <div className="flex items-center justify-end gap-1">
        <Button variant="ghost" size="icon" asChild aria-label="Edit pengguna">
          <Link href={`/users/${user.id}`}>
            <Pencil size={14} aria-hidden="true" />
          </Link>
        </Button>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              disabled={isPending}
              aria-label="Menu tindakan pengguna"
            >
              {isPending ? (
                <Loader2
                  size={14}
                  className="animate-spin"
                  aria-hidden="true"
                />
              ) : (
                <MoreHorizontal size={14} aria-hidden="true" />
              )}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48">
            {isLocked && (
              <>
                <DropdownMenuItem onClick={() => setUnlockOpen(true)}>
                  <LockOpen size={14} className="mr-2" aria-hidden="true" />
                  Buka Kunci Akun
                </DropdownMenuItem>
                <DropdownMenuSeparator />
              </>
            )}
            <DropdownMenuItem
              onClick={() =>
                user.is_active ? setDeactivateOpen(true) : handleToggleStatus()
              }
              className={
                user.is_active
                  ? "text-destructive focus:text-destructive"
                  : undefined
              }
            >
              {user.is_active ? "Nonaktifkan" : "Aktifkan"}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {/* Deactivate confirm */}
      <AlertDialog open={deactivateOpen} onOpenChange={setDeactivateOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Nonaktifkan Pengguna</AlertDialogTitle>
            <AlertDialogDescription>
              Pengguna &quot;{user.full_name}&quot; tidak akan dapat login
              setelah dinonaktifkan. Anda dapat mengaktifkannya kembali kapan
              saja.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleToggleStatus}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Nonaktifkan
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Unlock confirm */}
      <AlertDialog open={unlockOpen} onOpenChange={setUnlockOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Buka Kunci Akun</AlertDialogTitle>
            <AlertDialogDescription>
              Akun &quot;{user.full_name}&quot; terkunci karena terlalu banyak
              percobaan login yang gagal. Membuka kunci akan mengizinkan
              pengguna untuk login kembali.
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
