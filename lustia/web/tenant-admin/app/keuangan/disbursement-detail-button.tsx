"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { DisbursementDetailDialog } from "./disbursement-detail-dialog";
import type { TenantDisbursement } from "@/lib/types";

export function DisbursementDetailButton({
  disbursement,
}: {
  disbursement: TenantDisbursement;
}) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setOpen(true)}
        aria-label={`Lihat detail pencairan periode ${disbursement.period_start}`}
      >
        Lihat
      </Button>
      <DisbursementDetailDialog
        disbursement={disbursement}
        open={open}
        onClose={() => setOpen(false)}
      />
    </>
  );
}
