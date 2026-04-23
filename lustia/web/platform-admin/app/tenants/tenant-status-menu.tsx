"use client";

import { useTransition, useState } from "react";
import { MoreHorizontal, Loader2 } from "lucide-react";
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
import { changeTenantStatus } from "./actions";
import type { Tenant } from "@/lib/types";

interface TenantStatusMenuProps {
  tenant: Tenant;
}

type PendingAction = {
  status: "suspended" | "deactivated" | "active";
  label: string;
  confirmTitle: string;
  confirmDescription: string;
  isDestructive: boolean;
};

export function TenantStatusMenu({ tenant }: TenantStatusMenuProps) {
  const [isPending, startTransition] = useTransition();
  const [pendingAction, setPendingAction] = useState<PendingAction | null>(null);

  const actions: PendingAction[] = [];

  if (tenant.status === "active") {
    actions.push({
      status: "suspended",
      label: "Suspend",
      confirmTitle: "Suspend Tenant",
      confirmDescription: `Tenant "${tenant.name}" akan disuspend. Semua keanggotaan aktif mereka akan tetap ada, tetapi login akan ditolak. Lanjutkan?`,
      isDestructive: false,
    });
    actions.push({
      status: "deactivated",
      label: "Nonaktifkan",
      confirmTitle: "Nonaktifkan Tenant",
      confirmDescription: `Tenant "${tenant.name}" akan dinonaktifkan secara permanen. Semua keanggotaan aktif akan disuspend. Tindakan ini tidak dapat dibatalkan. Lanjutkan?`,
      isDestructive: true,
    });
  }

  if (tenant.status === "suspended") {
    actions.push({
      status: "active",
      label: "Aktifkan Kembali",
      confirmTitle: "Aktifkan Kembali Tenant",
      confirmDescription: `Tenant "${tenant.name}" akan diaktifkan kembali. Lanjutkan?`,
      isDestructive: false,
    });
    actions.push({
      status: "deactivated",
      label: "Nonaktifkan",
      confirmTitle: "Nonaktifkan Tenant",
      confirmDescription: `Tenant "${tenant.name}" akan dinonaktifkan secara permanen. Tindakan ini tidak dapat dibatalkan. Lanjutkan?`,
      isDestructive: true,
    });
  }

  if (actions.length === 0) return null;

  function handleConfirm() {
    if (!pendingAction) return;
    const action = pendingAction;
    setPendingAction(null);

    startTransition(async () => {
      const fd = new FormData();
      fd.set("id", tenant.id);
      fd.set("status", action.status);

      const result = await changeTenantStatus(fd);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success(
        `Tenant "${tenant.name}" berhasil diperbarui ke status ${action.status}.`
      );
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
            aria-label="Menu tindakan tenant"
          >
            {isPending ? (
              <Loader2 size={14} className="animate-spin" aria-hidden="true" />
            ) : (
              <MoreHorizontal size={14} aria-hidden="true" />
            )}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-44">
          {actions.map((action, i) => (
            <span key={action.status}>
              {i > 0 && action.isDestructive && <DropdownMenuSeparator />}
              <DropdownMenuItem
                onClick={() => setPendingAction(action)}
                className={action.isDestructive ? "text-destructive focus:text-destructive" : ""}
              >
                {action.label}
              </DropdownMenuItem>
            </span>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      {/* Confirmation dialog */}
      <AlertDialog
        open={pendingAction !== null}
        onOpenChange={(open) => !open && setPendingAction(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{pendingAction?.confirmTitle}</AlertDialogTitle>
            <AlertDialogDescription>
              {pendingAction?.confirmDescription}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleConfirm}
              className={
                pendingAction?.isDestructive
                  ? "bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  : undefined
              }
            >
              Konfirmasi
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
