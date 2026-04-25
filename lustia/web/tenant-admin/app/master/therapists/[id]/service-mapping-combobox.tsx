"use client";

import { useState, useTransition, useRef } from "react";
import Link from "next/link";
import { Search, ChevronDown, Loader2, X, Plus } from "lucide-react";
import { toast } from "sonner";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
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
import { updateServiceMapping } from "./mapping-actions";
import type { Service, TherapistServiceMapping } from "@/lib/types";
import { cn } from "@/lib/utils";

// ─── Props ───────────────────────────────────────────────────────────────────

interface ServiceMappingComboboxProps {
  therapistId: string;
  allServices: Service[];
  assignedMappings: TherapistServiceMapping[];
}

// ─── Component ───────────────────────────────────────────────────────────────

export function ServiceMappingCombobox({
  therapistId,
  allServices,
  assignedMappings,
}: ServiceMappingComboboxProps) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [isPending, startTransition] = useTransition();
  const [removeTarget, setRemoveTarget] = useState<TherapistServiceMapping | null>(null);
  const [togglePending, setTogglePending] = useState<string | null>(null);
  const searchRef = useRef<HTMLInputElement>(null);

  // Active service ids currently in the full-replace set
  // We derive "currently assigned (active)" from mappings where is_active=true
  const activeServiceIds = assignedMappings
    .filter((m) => m.is_active)
    .map((m) => m.service_id);

  const filtered = allServices.filter((s) =>
    s.name.toLowerCase().includes(search.toLowerCase())
  );

  // ── Toggle a service in/out (PUT full-replace) ────────────────────────────

  function handleToggle(service: Service) {
    const isCurrentlyActive = activeServiceIds.includes(service.id);

    if (isCurrentlyActive) {
      // Removing — confirm via AlertDialog
      const mapping = assignedMappings.find((m) => m.service_id === service.id)!;
      setRemoveTarget(mapping);
      return;
    }

    // Adding — immediate
    const newIds = [...activeServiceIds, service.id];
    doUpdate(newIds, "Layanan berhasil ditugaskan.");
  }

  function confirmRemove() {
    if (!removeTarget) return;
    const newIds = activeServiceIds.filter((id) => id !== removeTarget.service_id);
    setRemoveTarget(null);
    doUpdate(newIds, "Layanan berhasil dilepas.");
  }

  function doUpdate(newIds: string[], successMsg: string) {
    startTransition(async () => {
      const result = await updateServiceMapping(therapistId, newIds);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success(successMsg);
      router.refresh();
    });
  }

  // ── Toggle is_active on existing mapping (without removing) ──────────────

  function handleToggleActive(mapping: TherapistServiceMapping) {
    setTogglePending(mapping.service_id);
    const otherActive = activeServiceIds.filter((id) => id !== mapping.service_id);
    const newIds = mapping.is_active
      ? otherActive // deactivate: remove from active set
      : [...otherActive, mapping.service_id]; // reactivate: add back

    startTransition(async () => {
      const result = await updateServiceMapping(therapistId, newIds);
      setTogglePending(null);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      toast.success(mapping.is_active ? "Layanan dinonaktifkan." : "Layanan diaktifkan.");
      router.refresh();
    });
  }

  // ── Remove mapping entirely ───────────────────────────────────────────────

  function handleRemove(mapping: TherapistServiceMapping) {
    setRemoveTarget(mapping);
  }

  return (
    <>
      <div className="space-y-4">
        {/* Section header */}
        <h3 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
          Layanan yang Ditawarkan
        </h3>

        {/* Combobox trigger */}
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <Button
              variant="ghost"
              className="gap-1.5 text-sm"
              disabled={isPending}
            >
              {isPending ? (
                <Loader2 size={14} className="animate-spin" aria-hidden="true" />
              ) : (
                <ChevronDown size={14} aria-hidden="true" />
              )}
              Tambah Layanan
            </Button>
          </PopoverTrigger>
          <PopoverContent
            className="w-80 p-0"
            align="start"
            onOpenAutoFocus={() => {
              // Focus search on open
              setTimeout(() => searchRef.current?.focus(), 50);
            }}
          >
            {/* Search */}
            <div className="flex items-center border-b px-3 py-2">
              <Search size={14} className="mr-2 shrink-0 text-muted-foreground" aria-hidden="true" />
              <input
                ref={searchRef}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Cari dan tambah layanan..."
                className="w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                aria-label="Cari layanan"
              />
              {search && (
                <button
                  type="button"
                  onClick={() => setSearch("")}
                  className="ml-2 text-muted-foreground hover:text-foreground"
                  aria-label="Hapus pencarian"
                >
                  <X size={12} />
                </button>
              )}
            </div>

            {/* Service list */}
            <div className="max-h-64 overflow-y-auto py-1">
              {allServices.length === 0 ? (
                <div className="space-y-2 px-3 py-4 text-center">
                  <p className="text-sm text-muted-foreground">
                    Belum ada layanan di tenant ini.
                  </p>
                  <Link
                    href="/master/services/new"
                    className="inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline"
                  >
                    <Plus size={12} aria-hidden="true" />
                    Buat layanan pertama
                  </Link>
                </div>
              ) : filtered.length === 0 ? (
                <p className="px-3 py-4 text-center text-sm text-muted-foreground">
                  Tidak ada layanan yang cocok dengan pencarian.
                </p>
              ) : (
                filtered.map((s) => {
                  const isActive = activeServiceIds.includes(s.id);
                  return (
                    <button
                      key={s.id}
                      type="button"
                      className={cn(
                        "flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-muted/50",
                        isActive && "font-medium text-foreground"
                      )}
                      onClick={() => handleToggle(s)}
                      disabled={isPending}
                    >
                      <input
                        type="checkbox"
                        readOnly
                        checked={isActive}
                        className="h-4 w-4 rounded border-input accent-primary"
                        aria-hidden="true"
                        tabIndex={-1}
                      />
                      <span className="flex-1 truncate">{s.name}</span>
                      {s.category && (
                        <Badge variant="outline" className="shrink-0 text-xs">
                          {s.category}
                        </Badge>
                      )}
                    </button>
                  );
                })
              )}
            </div>
          </PopoverContent>
        </Popover>

        {/* Assigned services list */}
        {assignedMappings.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            Terapis ini belum memiliki layanan. Gunakan pencarian di atas untuk
            menambahkan.
          </p>
        ) : (
          <div className="divide-y rounded-lg border">
            {assignedMappings.map((mapping) => (
              <div
                key={mapping.service_id}
                className={cn(
                  "flex items-center gap-3 px-4 py-3 transition-colors",
                  !mapping.is_active && "opacity-60"
                )}
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-foreground">
                    {mapping.name}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {[
                      mapping.category,
                      `${mapping.duration_minutes} menit`,
                      new Intl.NumberFormat("id-ID", {
                        style: "currency",
                        currency: "IDR",
                        minimumFractionDigits: 0,
                      }).format(mapping.price_idr),
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </p>
                </div>

                {/* is_active toggle */}
                <label className="flex cursor-pointer items-center gap-1.5 text-xs text-muted-foreground">
                  <input
                    type="checkbox"
                    checked={mapping.is_active}
                    disabled={isPending || togglePending === mapping.service_id}
                    onChange={() => handleToggleActive(mapping)}
                    className="h-4 w-4 cursor-pointer rounded border-input accent-primary"
                    aria-label={`Layanan ${mapping.name} aktif`}
                  />
                  <span
                    className={cn(
                      mapping.is_active ? "text-foreground" : "text-muted-foreground"
                    )}
                  >
                    Aktif
                  </span>
                </label>

                {/* Remove button */}
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 shrink-0 text-xs text-destructive hover:text-destructive"
                  disabled={isPending}
                  onClick={() => handleRemove(mapping)}
                >
                  Lepas
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Confirm remove dialog */}
      <AlertDialog
        open={!!removeTarget}
        onOpenChange={(v) => !v && setRemoveTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Lepas Layanan?</AlertDialogTitle>
            <AlertDialogDescription>
              Layanan &quot;{removeTarget?.name}&quot; akan dilepas dari terapis
              ini.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Batal</AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmRemove}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              Ya, Lepas
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
