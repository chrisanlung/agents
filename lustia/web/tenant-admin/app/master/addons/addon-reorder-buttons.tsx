"use client";

import { useState, useTransition } from "react";
import {
  ChevronUp,
  ChevronDown,
  Eye,
  EyeOff,
  Loader2,
  MoreHorizontal,
  Pencil,
  Trash2,
} from "lucide-react";
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
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { reorderAddons, toggleAddonStatus, deleteAddon } from "./actions";
import type { Addon } from "@/lib/types";

interface AddonReorderListProps {
  /** The ordered list of add-ons for the current page. */
  initialAddons: Addon[];
}

function formatPrice(priceIdr: number): string {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
  }).format(priceIdr);
}

/**
 * Client component that owns the reorder state for the current page.
 * Renders the complete add-on table (header + rows) with Up/Down controls
 * and row actions. The parent Server Component passes the fetched addons;
 * all reorder interaction is handled here with optimistic state.
 */
export function AddonReorderList({ initialAddons }: AddonReorderListProps) {
  const [addons, setAddons] = useState<Addon[]>(initialAddons);
  const [isReordering, setIsReordering] = useState(false);

  async function handleMove(index: number, direction: "up" | "down") {
    const targetIndex = direction === "up" ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= addons.length) return;

    // Snapshot for rollback
    const prev = addons;

    // Optimistic swap
    const next = [...addons];
    [next[index], next[targetIndex]] = [next[targetIndex], next[index]];
    setAddons(next);
    setIsReordering(true);

    try {
      const items = next.map((a, i) => ({ id: a.id, sort_order: i }));
      const result = await reorderAddons(items);
      if (!result.ok) {
        setAddons(prev);
        toast.error("Gagal mengubah urutan. Coba lagi.");
      }
    } catch {
      setAddons(prev);
      toast.error("Gagal mengubah urutan. Coba lagi.");
    } finally {
      setIsReordering(false);
    }
  }

  return (
    <Table className="min-w-[480px]">
      <TableHeader className="bg-muted/30">
        <TableRow>
          <TableHead>Nama</TableHead>
          <TableHead className="w-[130px]">Harga</TableHead>
          <TableHead className="w-[90px]">Status</TableHead>
          <TableHead className="w-[96px] text-center">Urutan</TableHead>
          <TableHead className="w-[80px]">Aksi</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody className="text-sm">
        {addons.map((addon, index) => (
          <AddonTableRow
            key={addon.id}
            addon={addon}
            index={index}
            total={addons.length}
            isReordering={isReordering}
            onMove={handleMove}
          />
        ))}
      </TableBody>
    </Table>
  );
}

// ─── Individual row ───────────────────────────────────────────────────────────

interface AddonTableRowProps {
  addon: Addon;
  index: number;
  total: number;
  isReordering: boolean;
  onMove: (index: number, direction: "up" | "down") => void;
}

function AddonTableRow({
  addon,
  index,
  total,
  isReordering,
  onMove,
}: AddonTableRowProps) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const [deleteOpen, setDeleteOpen] = useState(false);

  const isFirst = index === 0;
  const isLast = index === total - 1;

  function handleToggleStatus() {
    startTransition(async () => {
      const result = await toggleAddonStatus(addon.id, addon.is_active);
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
    startTransition(async () => {
      const result = await deleteAddon(addon.id);
      if (!result.ok) {
        toast.error(result.error || "Gagal menghapus add-on. Coba lagi.");
        return;
      }
      toast.success("Add-on berhasil dihapus.");
      router.refresh();
    });
  }

  return (
    <>
      <TableRow className="h-14">
        {/* Nama */}
        <TableCell className="min-w-[200px]">
          <span
            className={
              addon.is_active
                ? "font-medium"
                : "font-medium line-through text-muted-foreground"
            }
          >
            {addon.name}
          </span>
        </TableCell>

        {/* Harga */}
        <TableCell className="w-[130px]">
          <Badge variant="outline" className="tabular-nums font-normal">
            {formatPrice(addon.price_idr)}
          </Badge>
        </TableCell>

        {/* Status */}
        <TableCell className="w-[90px]">
          <Badge variant={addon.is_active ? "success" : "muted"}>
            {addon.is_active ? "Aktif" : "Nonaktif"}
          </Badge>
        </TableCell>

        {/* Urutan */}
        <TableCell className="w-[96px]">
          <div className="flex items-center justify-center gap-0.5">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onMove(index, "up")}
              disabled={isFirst || isReordering}
              aria-label={`Pindah ke atas: ${addon.name}`}
            >
              <ChevronUp size={14} aria-hidden="true" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onMove(index, "down")}
              disabled={isLast || isReordering}
              aria-label={`Pindah ke bawah: ${addon.name}`}
            >
              <ChevronDown size={14} aria-hidden="true" />
            </Button>
          </div>
        </TableCell>

        {/* Aksi */}
        <TableCell className="w-[80px]">
          <div className="flex items-center justify-end gap-1">
            <Button
              asChild
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              aria-label={`Edit add-on: ${addon.name}`}
            >
              <Link href={`/master/addons/${addon.id}`}>
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
                  aria-label={`Tindakan lainnya untuk add-on: ${addon.name}`}
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
              <DropdownMenuContent align="end" className="w-44">
                <DropdownMenuItem onClick={handleToggleStatus}>
                  {addon.is_active ? (
                    <>
                      <EyeOff size={14} aria-hidden="true" />
                      Nonaktifkan
                    </>
                  ) : (
                    <>
                      <Eye size={14} aria-hidden="true" />
                      Aktifkan
                    </>
                  )}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={() => setDeleteOpen(true)}
                  className="text-destructive focus:text-destructive"
                >
                  <Trash2 size={14} aria-hidden="true" />
                  Hapus
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </TableCell>
      </TableRow>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Hapus Add-on?</AlertDialogTitle>
            <AlertDialogDescription>
              Add-on &quot;{addon.name}&quot; akan dihapus secara permanen dan
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
