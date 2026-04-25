"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { toggleAddonStatus, deleteAddon } from "../actions";

interface AddonDetailActionsProps {
  addonId: string;
  addonName: string;
  isActive: boolean;
}

/**
 * Destructive actions for the add-on edit page: toggle status and delete.
 * Rendered below the form card so the primary action (Simpan) stays prominent.
 */
export function AddonDetailActions({
  addonId,
  addonName,
  isActive,
}: AddonDetailActionsProps) {
  const router = useRouter();
  const [isPendingToggle, startToggle] = useTransition();
  const [isPendingDelete, startDelete] = useTransition();
  const [deleteOpen, setDeleteOpen] = useState(false);

  function handleToggleStatus() {
    startToggle(async () => {
      const result = await toggleAddonStatus(addonId, isActive);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success(
        result.is_active ? "Add-on diaktifkan." : "Add-on dinonaktifkan."
      );
      router.refresh();
    });
  }

  function handleDelete() {
    setDeleteOpen(false);
    startDelete(async () => {
      const result = await deleteAddon(addonId);
      if (!result.ok) {
        toast.error(result.error || "Gagal menghapus add-on. Coba lagi.");
        return;
      }
      toast.success("Add-on berhasil dihapus.");
      router.push("/master/addons");
    });
  }

  return (
    <>
      <div className="flex justify-end gap-2">
        <Button
          type="button"
          variant="outline"
          onClick={handleToggleStatus}
          disabled={isPendingToggle || isPendingDelete}
          className={
            isActive
              ? "border-amber-300 text-amber-700 hover:bg-amber-50"
              : "border-emerald-300 text-emerald-700 hover:bg-emerald-50"
          }
        >
          {isPendingToggle && (
            <Loader2 size={14} className="animate-spin" aria-hidden="true" />
          )}
          {isActive ? "Nonaktifkan" : "Aktifkan"}
        </Button>

        <Button
          type="button"
          variant="outline"
          onClick={() => setDeleteOpen(true)}
          disabled={isPendingToggle || isPendingDelete}
          className="border-destructive/40 text-destructive hover:bg-destructive/5"
        >
          {isPendingDelete && (
            <Loader2 size={14} className="animate-spin" aria-hidden="true" />
          )}
          Hapus
        </Button>
      </div>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Add-on?</AlertDialogTitle>
            <AlertDialogDescription>
              Add-on &quot;{addonName}&quot; akan dihapus secara permanen dan
              tidak dapat dipulihkan.
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
