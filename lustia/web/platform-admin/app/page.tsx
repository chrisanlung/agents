import { redirect } from "next/navigation";
import { cookies } from "next/headers";

/**
 * Root route: redirect to /dashboard if a session cookie is present,
 * otherwise redirect to /login. The middleware handles this for subsequent
 * navigations; this page handles the initial `/` load.
 */
export default async function RootPage() {
  const cookieStore = await cookies();
  const hasSession = cookieStore.has("access_token");
  redirect(hasSession ? "/dashboard" : "/login");
}
