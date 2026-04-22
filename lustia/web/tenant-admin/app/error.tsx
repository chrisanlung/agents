"use client";

import { useEffect } from "react";
import { Button } from "@/components/ui/button";

export default function ErrorPage({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Log to error reporting service in a future phase
    console.error(error);
  }, [error]);

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 px-4">
      <h2 className="text-lg font-semibold">Terjadi kesalahan</h2>
      <p className="max-w-sm text-center text-sm text-muted-foreground">
        Terjadi kesalahan tak terduga. Silakan coba lagi atau hubungi tim dukungan jika masalah berlanjut.
      </p>
      <Button onClick={reset} variant="outline">
        Coba lagi
      </Button>
    </div>
  );
}
