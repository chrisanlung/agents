"use client";

import * as React from "react";
import { useTransition, useState } from "react";
import { CheckCircle2, XCircle, Loader2, Copy, Check, AlertTriangle } from "lucide-react";
import { toast } from "sonner";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
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
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { approveRegistration, rejectRegistration } from "./actions";
import type { ApproveRegistrationResponse, TenantPackage } from "@/lib/types";

const PACKAGE_DEFAULTS: Record<TenantPackage, number> = {
  starter: 1,
  growth: 5,
  enterprise: 999,
};

interface ApprovalPanelProps {
  registrationId: string;
  defaultPackage: TenantPackage;
  contactEmail: string;
}

export function ApprovalPanel({
  registrationId,
  defaultPackage,
  contactEmail,
}: ApprovalPanelProps) {
  const [isPendingApprove, startApprove] = useTransition();
  const [isPendingReject, startReject] = useTransition();
  const [selectedPackage, setSelectedPackage] = useState<TenantPackage>(defaultPackage);
  const [maxBranches, setMaxBranches] = useState(
    PACKAGE_DEFAULTS[defaultPackage]
  );
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState("");
  const [approvalResult, setApprovalResult] =
    useState<ApproveRegistrationResponse | null>(null);

  function handlePackageChange(val: string) {
    const pkg = val as TenantPackage;
    setSelectedPackage(pkg);
    setMaxBranches(PACKAGE_DEFAULTS[pkg]);
  }

  function handleApprove() {
    startApprove(async () => {
      const fd = new FormData();
      fd.set("id", registrationId);
      fd.set("package", selectedPackage);
      fd.set("max_branches", String(maxBranches));

      const result = await approveRegistration(fd);
      if (!result.ok) {
        toast.error(result.error);
        return;
      }
      setApprovalResult(result.data);
    });
  }

  function handleReject() {
    if (!rejectReason.trim()) return;
    startReject(async () => {
      const fd = new FormData();
      fd.set("id", registrationId);
      fd.set("reason", rejectReason.trim());

      const result = await rejectRegistration(fd);
      if (!result.ok) {
        toast.error(result.error);
        setRejectOpen(false);
      }
      // On success, server action redirects — no extra work here
    });
  }

  // ── Post-approval success view ───────────────────────────────────────────
  if (approvalResult) {
    return (
      <ApprovalSuccessCard
        approvalResult={approvalResult}
        contactEmail={contactEmail}
      />
    );
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Panel Tindakan</CardTitle>
          <CardDescription>
            Sesuaikan paket dan batas cabang sebelum menyetujui.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Package override */}
          <div className="space-y-1.5">
            <label
              htmlFor="package-select"
              className="text-sm font-medium text-foreground"
            >
              Paket
            </label>
            <Select
              value={selectedPackage}
              onValueChange={handlePackageChange}
            >
              <SelectTrigger id="package-select" className="w-full">
                <SelectValue placeholder="Pilih paket" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="starter">Starter (1 cabang)</SelectItem>
                <SelectItem value="growth">Growth (5 cabang)</SelectItem>
                <SelectItem value="enterprise">Enterprise (tidak terbatas)</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {/* Max branches override */}
          <div className="space-y-1.5">
            <label
              htmlFor="max-branches"
              className="text-sm font-medium text-foreground"
            >
              Maks. Cabang
            </label>
            <input
              id="max-branches"
              type="number"
              min={1}
              value={maxBranches === 999 ? "" : maxBranches}
              placeholder={maxBranches === 999 ? "Tidak terbatas (999)" : undefined}
              onChange={(e) => {
                const val = parseInt(e.target.value, 10);
                if (!isNaN(val) && val >= 1) setMaxBranches(val);
              }}
              className={cn(
                "flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm",
                "ring-offset-background placeholder:text-muted-foreground",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
                "disabled:cursor-not-allowed disabled:opacity-50"
              )}
            />
            <p className="text-xs text-muted-foreground">
              Nilai 999 = tidak terbatas (sentinel enterprise).
            </p>
          </div>

          {/* Action buttons */}
          <div className="flex flex-col gap-2 pt-2">
            <Button
              onClick={handleApprove}
              disabled={isPendingApprove || isPendingReject}
              className="w-full"
            >
              {isPendingApprove ? (
                <Loader2 size={16} className="animate-spin" aria-hidden="true" />
              ) : (
                <CheckCircle2 size={16} aria-hidden="true" />
              )}
              Setujui
            </Button>
            <Button
              variant="destructive"
              onClick={() => setRejectOpen(true)}
              disabled={isPendingApprove || isPendingReject}
              className="w-full"
            >
              <XCircle size={16} aria-hidden="true" />
              Tolak
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Reject dialog */}
      <AlertDialog open={rejectOpen} onOpenChange={setRejectOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Tolak Registrasi</AlertDialogTitle>
            <AlertDialogDescription>
              Berikan alasan penolakan. Alasan ini akan dicatat dan dapat dikirimkan ke pemohon.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="py-2">
            <Textarea
              placeholder="Tulis alasan penolakan..."
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
              rows={4}
              maxLength={1000}
              className="w-full"
              aria-label="Alasan penolakan"
            />
            <p className="mt-1 text-right text-xs text-muted-foreground">
              {rejectReason.length}/1000
            </p>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isPendingReject}>Batal</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleReject}
              disabled={!rejectReason.trim() || isPendingReject}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {isPendingReject ? (
                <Loader2 size={14} className="animate-spin" aria-hidden="true" />
              ) : null}
              Tolak Registrasi
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

// ── Success card after approval ────────────────────────────────────────────

function ApprovalSuccessCard({
  approvalResult,
  contactEmail,
}: {
  approvalResult: ApproveRegistrationResponse;
  contactEmail: string;
}) {
  const [copied, setCopied] = useState(false);
  const password = approvalResult.tenant_admin.temporary_password;

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(password);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error("Gagal menyalin ke clipboard.");
    }
  }

  return (
    <Card className="border-emerald-200 bg-emerald-50">
      <CardHeader>
        <div className="flex items-center gap-2">
          <CheckCircle2 size={20} className="text-emerald-600" aria-hidden="true" />
          <CardTitle className="text-base text-emerald-800">
            Registrasi Disetujui
          </CardTitle>
        </div>
        <CardDescription className="text-emerald-700">
          Tenant <strong>{approvalResult.tenant.name}</strong> telah dibuat.
          Admin tenant: <strong>{approvalResult.tenant_admin.email}</strong>.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Temporary password */}
        <div className="rounded-lg border border-amber-200 bg-amber-50 p-4">
          <div className="mb-2 flex items-start gap-2">
            <AlertTriangle
              size={16}
              className="mt-0.5 shrink-0 text-amber-600"
              aria-hidden="true"
            />
            <p className="text-xs font-medium text-amber-800">
              Bagikan password ini kepada admin tenant dengan aman. Password hanya
              ditampilkan sekali di layar ini.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <code className="flex-1 rounded bg-white px-3 py-2 font-mono text-sm tracking-wider text-foreground shadow-sm">
              {password}
            </code>
            <Button
              size="sm"
              variant="outline"
              onClick={handleCopy}
              aria-label="Salin password"
            >
              {copied ? (
                <Check size={14} className="text-emerald-600" />
              ) : (
                <Copy size={14} />
              )}
              {copied ? "Tersalin" : "Salin"}
            </Button>
          </div>
        </div>

        <p className="text-sm text-muted-foreground">
          Email berisi link login + password juga telah dikirim ke{" "}
          <strong>{contactEmail}</strong>.
        </p>

        <dl className="grid grid-cols-2 gap-3 text-sm">
          <div>
            <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Tenant
            </dt>
            <dd>{approvalResult.tenant.name}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Slug
            </dt>
            <dd>
              <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
                {approvalResult.tenant.slug}
              </code>
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Paket
            </dt>
            <dd className="capitalize">{approvalResult.tenant.package}</dd>
          </div>
          <div>
            <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Maks. Cabang
            </dt>
            <dd>
              {approvalResult.tenant.max_branches === 999
                ? "Tidak terbatas"
                : approvalResult.tenant.max_branches}
            </dd>
          </div>
        </dl>
      </CardContent>
    </Card>
  );
}
