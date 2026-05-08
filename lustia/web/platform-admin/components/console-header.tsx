import { ShieldCheck } from "lucide-react";

import { NavLinks } from "@/components/nav-links";
import { PlatformUserMenu } from "@/components/platform-user-menu";

const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Platform Console";

interface ConsoleHeaderProps {
  user: {
    full_name: string;
    email: string;
    avatar_url?: string;
  };
  pendingRegistrationCount: number;
  failedDisbursementCount?: number;
}

/**
 * ConsoleHeader — shared authenticated header for platform-admin pages.
 *
 * Server Component. Renders the brand logo, NavLinks (with pending badge),
 * and PlatformUserMenu (client leaf).
 *
 * Do NOT add this to app/layout.tsx — that wraps /login too.
 * Instead, include it in each authenticated page/layout that needs it.
 */
export function ConsoleHeader({ user, pendingRegistrationCount, failedDisbursementCount = 0 }: ConsoleHeaderProps) {
  return (
    <header className="border-b bg-white px-6 py-4 shadow-sm">
      <div className="mx-auto flex max-w-5xl items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <ShieldCheck size={16} aria-hidden="true" />
          </div>
          <span className="font-semibold text-foreground">{appName}</span>
        </div>
        <div className="flex items-center gap-4">
          <NavLinks pendingCount={pendingRegistrationCount} failedDisbursementCount={failedDisbursementCount} />
          <PlatformUserMenu user={user} />
        </div>
      </div>
    </header>
  );
}
