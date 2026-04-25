"use client";

import { useEffect } from "react";
import { useSearchParams, useRouter, usePathname } from "next/navigation";
import { toast } from "sonner";

/**
 * FlashToast reads `?flash=<message>` from the URL on mount, fires a sonner
 * toast, then strips the param from the URL via router.replace so refresh or
 * share-link doesn't re-trigger the toast.
 *
 * Paired with handleApiError() which redirects to /dashboard?flash=... on 403.
 * Query-param approach is used because Next.js forbids cookies().set() inside
 * Server Components.
 */
export function FlashToast() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    const flash = searchParams?.get("flash");
    if (!flash) return;

    toast.info(flash);

    const params = new URLSearchParams(searchParams.toString());
    params.delete("flash");
    const qs = params.toString();
    router.replace(qs ? `${pathname}?${qs}` : pathname);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams]);

  return null;
}
