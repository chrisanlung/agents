// server-only: uses redirect() from next/navigation inside Server Components.
import "server-only";

import { redirect, notFound } from "next/navigation";

import { ApiError } from "./api";

/**
 * Handle auth/authorization errors from apiFetch calls inside Server Components.
 *
 *   - 401 → redirect /login
 *   - 403 → redirect /dashboard?flash=<encoded message> so the client can pop
 *           a toast. We use a query param instead of a cookie because
 *           cookies().set() is disallowed inside Server Components.
 *   - 404 → notFound()
 *   - else → rethrow
 */
export async function handleApiError(err: unknown): Promise<never> {
  if (err instanceof ApiError) {
    if (err.status === 401) {
      redirect("/login");
    }
    if (err.status === 403) {
      const msg =
        err.message ||
        "Anda tidak memiliki izin untuk mengakses halaman tersebut.";
      redirect(`/dashboard?flash=${encodeURIComponent(msg)}`);
    }
    if (err.status === 404) {
      notFound();
    }
  }
  throw err;
}
