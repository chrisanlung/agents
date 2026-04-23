import { cn } from "@/lib/utils";

/**
 * AppBackground — shared wrapper for all post-login pages in tenant-admin.
 * Renders the dashboard photo background with a semi-opaque overlay so that
 * white headers and Card components on top remain legible.
 */
export function AppBackground({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "relative min-h-screen bg-cover bg-center bg-fixed bg-no-repeat",
        className
      )}
      style={{ backgroundImage: "url('/bg-lustis-dashboard-tenant.webp')" }}
    >
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 bg-emerald-50/80"
      />
      <div className="relative">{children}</div>
    </div>
  );
}
