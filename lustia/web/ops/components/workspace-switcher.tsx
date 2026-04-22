"use client";

import { useTransition } from "react";
import { ChevronDown, Loader2, Building2, Check } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { selectTenantAction } from "@/app/select-tenant/actions";

export interface MembershipSummary {
  tenant_id: string;
  tenant_name: string;
  tenant_slug: string;
  roles: string[];
  branches: string[];
  status: string;
}

interface WorkspaceSwitcherProps {
  currentTenantName: string;
  currentTenantSlug: string;
  memberships: MembershipSummary[];
}

/**
 * WorkspaceSwitcher — displayed in the dashboard header for ops.
 *
 * When the user has only one membership it renders as a static label.
 * When multiple memberships exist it renders a popover dropdown.
 *
 * Switching calls selectTenantAction (Server Action) which replaces the
 * session cookies with a new tenant-scoped token and redirects to /dashboard.
 *
 * ADR 0007 §2.4 — workspace switcher.
 */
export function WorkspaceSwitcher({
  currentTenantName,
  currentTenantSlug,
  memberships,
}: WorkspaceSwitcherProps) {
  const [isPending, startTransition] = useTransition();

  // Only one membership — no switcher needed.
  if (memberships.length <= 1) {
    return (
      <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Building2 size={14} aria-hidden="true" />
        <span className="font-medium text-foreground">{currentTenantName}</span>
      </div>
    );
  }

  const otherMemberships = memberships.filter(
    (m) => m.tenant_slug !== currentTenantSlug
  );

  function handleSwitch(tenantId: string) {
    startTransition(async () => {
      await selectTenantAction(tenantId);
    });
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className="flex items-center gap-1.5"
          disabled={isPending}
          aria-busy={isPending}
          aria-label={`Workspace saat ini: ${currentTenantName}. Klik untuk ganti.`}
        >
          {isPending ? (
            <Loader2 size={14} className="animate-spin" aria-hidden="true" />
          ) : (
            <Building2 size={14} aria-hidden="true" />
          )}
          <span className="max-w-[120px] truncate">{currentTenantName}</span>
          <ChevronDown size={14} aria-hidden="true" />
        </Button>
      </PopoverTrigger>

      <PopoverContent align="end" className="w-64 p-2">
        <p className="mb-1.5 px-2 text-xs font-medium text-muted-foreground">
          Ganti workspace
        </p>

        {/* Current workspace (non-interactive) */}
        <div className="flex items-center gap-2 rounded-sm px-2 py-1.5">
          <Check size={14} className="text-primary" aria-hidden="true" />
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{currentTenantName}</p>
            <p className="truncate font-mono text-xs text-muted-foreground">
              {currentTenantSlug}
            </p>
          </div>
        </div>

        {otherMemberships.length > 0 && (
          <div className="my-1 border-t" role="separator" />
        )}

        {/* Other workspaces */}
        {otherMemberships.map((membership) => (
          <button
            key={membership.tenant_id}
            type="button"
            onClick={() => handleSwitch(membership.tenant_id)}
            disabled={isPending}
            className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left transition-colors hover:bg-accent hover:text-accent-foreground disabled:pointer-events-none disabled:opacity-50"
          >
            <Building2
              size={14}
              className="shrink-0 text-muted-foreground"
              aria-hidden="true"
            />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm">{membership.tenant_name}</p>
              <p className="truncate font-mono text-xs text-muted-foreground">
                {membership.tenant_slug}
              </p>
            </div>
            <span className="shrink-0 text-xs text-muted-foreground">
              Ganti
            </span>
          </button>
        ))}
      </PopoverContent>
    </Popover>
  );
}
