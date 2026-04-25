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
import { toggleRoomStatus, deleteRoom } from "../actions";

interface RoomDetailActionsProps {
  roomId: string;
  roomName: string;
  isActive: boolean;
}

/**
 * Destructive actions for the room edit page: toggle status and delete.
 * Rendered below the form card so the primary action (Simpan) stays prominent.
 */
export function RoomDetailActions({
  roomId,
  roomName,
  isActive,
}: RoomDetailActionsProps) {
  const router = useRouter();
  const [isPendingToggle, startToggle] = useTransition();
  const [isPendingDelete, startDelete] = useTransition();
  const [deleteOpen, setDeleteOpen] = useState(false);

  function handleToggleStatus() {
    startToggle(async () => {
      const result = await toggleRoomStatus(roomId, isActive);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success(
        result.is_active ? "Ruangan diaktifkan." : "Ruangan dinonaktifkan."
      );
      router.refresh();
    });
  }

  function handleDelete() {
    setDeleteOpen(false);
    startDelete(async () => {
      const result = await deleteRoom(roomId);
      if (!result.ok) {
        toast.error(result.error || "Gagal menghapus ruangan. Coba lagi.");
        return;
      }
      toast.success("Ruangan berhasil dihapus.");
      router.push("/master/rooms");
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
            <AlertDialogTitle>Hapus Ruangan?</AlertDialogTitle>
            <AlertDialogDescription>
              Ruangan &quot;{roomName}&quot; akan dihapus secara permanen dan
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
