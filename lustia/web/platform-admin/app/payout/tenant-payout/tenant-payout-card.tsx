"use client";

import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { TenantPayoutSummary } from "@/lib/types";
import { CreateDisbursementDialog } from "./create-disbursement-dialog";

interface TenantPayoutCardProps {
  tenant: TenantPayoutSummary;
  periodStart: string;
  periodEnd: string;
}

export function TenantPayoutCard({
  tenant,
  periodStart,
  periodEnd,
}: TenantPayoutCardProps) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const isEmpty = tenant.settled_amount_idr === 0;

  return (
    <>
      <Card className={cn(isEmpty && "bg-muted/30")}>
        <CardContent className="pt-4">
          <div className="flex items-start justify-between">
            <div>
              <p className="font-semibold text-foreground">
                {tenant.tenant_name}
              </p>
              <p className="text-xs text-muted-foreground mt-0.5">
                {tenant.branch_count} cabang
              </p>
            </div>
            <Badge variant="outline" className="text-xs">
              {tenant.tenant_slug}
            </Badge>
          </div>

          <div className="mt-4">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              Siap Dicairkan
            </p>
            <p
              className={cn(
                "mt-1 text-2xl font-bold tabular-nums",
                isEmpty ? "text-muted-foreground" : "text-primary"
              )}
              aria-label={formatRupiah(tenant.settled_amount_idr)}
            >
              {formatRupiah(tenant.settled_amount_idr)}
            </p>
            <p className="mt-0.5 text-xs text-muted-foreground">
              {tenant.transaction_count} transaksi
            </p>
          </div>

          <div className="mt-4">
            <Button
              size="sm"
              disabled={isEmpty}
              onClick={() => setDialogOpen(true)}
              className="w-full"
              aria-label={`Buat pencairan untuk ${tenant.tenant_name}`}
            >
              Buat Pencairan
            </Button>
          </div>
        </CardContent>
      </Card>

      <CreateDisbursementDialog
        tenant={tenant}
        periodStart={periodStart}
        periodEnd={periodEnd}
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
      />
    </>
  );
}
