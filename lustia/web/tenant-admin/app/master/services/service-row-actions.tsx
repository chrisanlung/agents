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
import { toggleServiceStatus, deleteService } from "./actions";
import type { Service } from "@/lib/types";

interface ServiceRowActionsProps {
  service: Service;
}

export function ServiceRowActions({ service }: ServiceRowActionsProps) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const [deleteOpen, setDeleteOpen] = useState(false);

  function handleToggleStatus() {
    startTransition(async () => {
      const result = await toggleServiceStatus(service.id, service.is_active);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success(
        result.is_active ? "Layanan diaktifkan." : "Layanan dinonaktifkan."
      );
      router.refresh();
    });
  }

  function handleDelete() {
    setDeleteOpen(false);
    startTransition(async () => {
      const result = await deleteService(service.id);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success("Layanan berhasil dihapus.");
      router.refresh();
    });
  }

  return (
    <>
      <div className="flex items-center justify-end gap-1">
        <Button
          asChild
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          aria-label={`Buka detail ${service.name}`}
        >
          <Link href={`/master/services/${service.id}`}>
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
              aria-label={`Tindakan lainnya untuk ${service.name}`}
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
              {service.is_active ? "Nonaktifkan" : "Aktifkan"}
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
            <AlertDialogTitle>Hapus Layanan?</AlertDialogTitle>
            <AlertDialogDescription>
              Layanan &quot;{service.name}&quot; akan dihapus. Terapis yang
              memiliki layanan ini akan kehilangan penugasannya.
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
