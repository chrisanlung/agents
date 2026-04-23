"use client";

import { useTransition, useState } from "react";
import { MoreHorizontal, Loader2, Pencil } from "lucide-react";
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
import { toggleBranchStatus, deleteBranch } from "./actions";
import type { Branch } from "@/lib/types";

interface BranchRowActionsProps {
  branch: Branch;
}

export function BranchRowActions({ branch }: BranchRowActionsProps) {
  const [isPending, startTransition] = useTransition();
  const [deleteOpen, setDeleteOpen] = useState(false);

  function handleToggleStatus() {
    startTransition(async () => {
      const result = await toggleBranchStatus(branch.id, branch.status);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success(
        result.newStatus === "active"
          ? `Cabang "${branch.name}" diaktifkan.`
          : `Cabang "${branch.name}" dinonaktifkan.`
      );
    });
  }

  function handleDelete() {
    setDeleteOpen(false);
    startTransition(async () => {
      const result = await deleteBranch(branch.id);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success(`Cabang "${branch.name}" berhasil dihapus.`);
    });
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            disabled={isPending}
            aria-label="Menu tindakan cabang"
          >
            {isPending ? (
              <Loader2 size={14} className="animate-spin" aria-hidden="true" />
            ) : (
              <MoreHorizontal size={14} aria-hidden="true" />
            )}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-44">
          <DropdownMenuItem asChild>
            <Link href={`/branches/${branch.id}`}>
              <Pencil size={14} className="mr-2" aria-hidden="true" />
              Edit
            </Link>
          </DropdownMenuItem>
          <DropdownMenuItem onClick={handleToggleStatus}>
            {branch.status === "active" ? "Nonaktifkan" : "Aktifkan"}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={() => setDeleteOpen(true)}
            className="text-destructive focus:text-destructive"
          >
            Hapus
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Cabang</AlertDialogTitle>
            <AlertDialogDescription>
              Cabang &quot;{branch.name}&quot; akan dihapus secara permanen
              (soft delete). Data historis tetap tersimpan untuk audit.
              Tindakan ini tidak dapat dibatalkan.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Hapus
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
