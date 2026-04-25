import type { Metadata } from "next";

import { CheckinFlow } from "./checkin-flow";

export const metadata: Metadata = {
  title: "Check-in Pelanggan",
};

export default function CheckinPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-foreground sm:text-2xl">
          Check-in Pelanggan
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Pindai QR code atau masukkan kode booking secara manual.
        </p>
      </div>

      <CheckinFlow />
    </div>
  );
}
