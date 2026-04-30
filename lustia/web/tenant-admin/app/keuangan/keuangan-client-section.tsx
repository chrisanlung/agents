"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { PaymentFaqDialog } from "./payment-faq-dialog";

/**
 * Client island used inside the KEU-A5 zero-state card.
 * Renders the "Pelajari cara menerima pembayaran" link that opens the FAQ modal.
 */
export function KeuanganClientSection() {
  const [faqOpen, setFaqOpen] = useState(false);

  return (
    <>
      <Button
        variant="link"
        className="mt-4 text-sm"
        onClick={() => setFaqOpen(true)}
      >
        Pelajari cara menerima pembayaran
      </Button>
      <PaymentFaqDialog open={faqOpen} onClose={() => setFaqOpen(false)} />
    </>
  );
}
