"use client";

import { useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { toast } from "sonner";

/**
 * Reads the `err` search param and fires a toast when the page is first rendered.
 * Used by select-tenant redirecting back to /login?err=role_mismatch.
 */
export function LoginErrorToast() {
  const searchParams = useSearchParams();
  const err = searchParams.get("err");

  useEffect(() => {
    if (err === "role_mismatch") {
      toast.error("Anda tidak memiliki akses ke portal ini untuk tenant tersebut.");
    }
  }, [err]);

  return null;
}
