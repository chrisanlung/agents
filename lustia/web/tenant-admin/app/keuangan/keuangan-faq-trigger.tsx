"use client";

import { useState } from "react";
import { PaymentFaqDialog } from "./payment-faq-dialog";

export function KeuanganFaqTrigger() {
  const [open, setOpen] = useState(false);

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        className="underline-offset-2 hover:underline focus-visible:ring-2 focus-visible:ring-primary rounded"
      >
        Pelajari Skema Pembayaran
      </button>
      <PaymentFaqDialog open={open} onClose={() => setOpen(false)} />
    </>
  );
}
