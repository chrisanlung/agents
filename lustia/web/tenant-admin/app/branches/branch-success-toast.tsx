"use client";

import { useEffect } from "react";
import { useSearchParams, useRouter, usePathname } from "next/navigation";
import { toast } from "sonner";

/**
 * Reads ?created=1 or ?updated=1 query params set by the Server Action after
 * redirect, fires a toast, then cleans the URL.
 * Must be wrapped in <Suspense> by its parent — useSearchParams() requires it.
 */
export function BranchSuccessToast() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    const created = searchParams.get("created");
    const updated = searchParams.get("updated");

    if (created === "1") {
      toast.success("Cabang berhasil ditambahkan.");
      const params = new URLSearchParams(searchParams.toString());
      params.delete("created");
      const qs = params.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname);
    } else if (updated === "1") {
      toast.success("Cabang berhasil diperbarui.");
      const params = new URLSearchParams(searchParams.toString());
      params.delete("updated");
      const qs = params.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname);
    }
  }, [searchParams, router, pathname]);

  return null;
}
