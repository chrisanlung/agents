"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { BatchDetailDialog } from "./batch-detail-dialog";
import type { SettlementBatch } from "@/lib/types";

export function BatchDetailButton({ batch }: { batch: SettlementBatch }) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setOpen(true)}
        aria-label={`Detail batch ${batch.settled_at}`}
      >
        Detail
      </Button>
      <BatchDetailDialog
        batch={batch}
        open={open}
        onClose={() => setOpen(false)}
      />
    </>
  );
}
