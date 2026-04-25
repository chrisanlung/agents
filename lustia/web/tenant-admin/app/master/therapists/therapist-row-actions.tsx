"use client";

import { useTransition, useState } from "react";
import { MoreHorizontal, Loader2, Pencil } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
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
import { toggleTherapistStatus, deleteTherapist } from "./actions";
import type { Therapist } from "@/lib/types";

interface TherapistRowActionsProps {
  therapist: Therapist;
}

export function TherapistRowActions({ therapist }: TherapistRowActionsProps) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const [deleteOpen, setDeleteOpen] = useState(false);

  function handleToggleStatus() {
    startTransition(async () => {
      const result = await toggleTherapistStatus(therapist.id, therapist.is_active);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success(
        result.is_active ? "Terapis diaktifkan." : "Terapis dinonaktifkan."
      );
      router.refresh();
    });
  }

  function handleDelete() {
    setDeleteOpen(false);
    startTransition(async () => {
      const result = await deleteTherapist(therapist.id);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success("Terapis berhasil dihapus.");
      router.refresh();
    });
  }

  return (
    <>
      <div className="flex items-center justify-end gap-1">
        {/* Visible Edit button — explicit affordance for non-technical users
            per UX review 2026-04-24. Secondary actions (toggle, delete) stay
            in the dropdown to avoid row-action sprawl. */}
        <Button
          asChild
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          aria-label={`Buka detail ${therapist.full_name}`}
        >
          <Link href={`/master/therapists/${therapist.id}`}>
            <Pencil size={14} aria-hidden="true" />
          </Link>
        </Button>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              disabled={isPending}
              aria-label={`Tindakan lainnya untuk ${therapist.full_name}`}
            >
              {isPending ? (
                <Loader2 size={14} className="animate-spin" aria-hidden="true" />
              ) : (
                <MoreHorizontal size={14} aria-hidden="true" />
              )}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-44">
            <DropdownMenuItem onClick={handleToggleStatus}>
              {therapist.is_active ? "Nonaktifkan" : "Aktifkan"}
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
      </div>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Terapis?</AlertDialogTitle>
            <AlertDialogDescription>
              Data &quot;{therapist.full_name}&quot; akan dihapus secara
              permanen dan tidak dapat dipulihkan. Tindakan ini tidak dapat
              dibatalkan.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Ya, Hapus
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
