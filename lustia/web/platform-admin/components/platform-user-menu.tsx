"use client";

import Link from "next/link";
import Image from "next/image";
import { useTransition } from "react";
import { ChevronDown, LogOut, Loader2, Settings, User } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { logoutAction } from "@/app/dashboard/actions";

export interface PlatformUserMenuProps {
  user: {
    full_name: string;
    email: string;
    avatar_url?: string;
  };
}

/** Derive at most 2 initials from a full name. */
function getInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].charAt(0).toUpperCase();
  return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase();
}

/**
 * PlatformUserMenu — header avatar dropdown for platform-admin.
 *
 * Desktop (md+): avatar circle + first name + chevron.
 * Mobile (<md): avatar circle + chevron only.
 *
 * Items: Profil Saya → /profil, Pengaturan → /pengaturan, Keluar (server action).
 * No "Ubah Kata Sandi" or workspace switcher — platform-admin scope only.
 *
 * Design spec: docs/DESIGN_FLOWS/platform-admin-dashboard.md §9.
 */
export function PlatformUserMenu({ user }: PlatformUserMenuProps) {
  const [isPending, startTransition] = useTransition();

  const initials = getInitials(user.full_name);
  const firstName = user.full_name.trim().split(/\s+/)[0] ?? user.full_name;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          className="flex items-center gap-2 rounded-full px-2 py-1
                     data-[state=open]:bg-accent
                     focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1"
          aria-label={`Menu akun ${user.full_name}`}
        >
          {/* Avatar circle */}
          {user.avatar_url ? (
            <Image
              src={user.avatar_url}
              alt=""
              aria-hidden="true"
              width={36}
              height={36}
              className="h-9 w-9 shrink-0 rounded-full object-cover ring-1 ring-border"
            />
          ) : (
            <span
              className="flex h-9 w-9 shrink-0 items-center justify-center
                         rounded-full bg-primary/10 text-primary
                         text-xs font-semibold ring-1 ring-border"
              aria-hidden="true"
            >
              {initials}
            </span>
          )}

          {/* First name — desktop only */}
          <span className="hidden md:block text-sm font-medium text-foreground">
            {firstName}
          </span>

          {/* Chevron */}
          <ChevronDown size={14} className="text-muted-foreground" aria-hidden="true" />
        </Button>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="w-56">
        {/* Identity block — non-interactive */}
        <div className="px-2 py-2" role="presentation">
          <p className="truncate text-sm font-semibold text-foreground">
            {user.full_name}
          </p>
          <p className="truncate text-xs text-muted-foreground">{user.email}</p>
        </div>

        <DropdownMenuSeparator />

        <DropdownMenuItem asChild>
          <Link href="/profil" className="cursor-pointer">
            <User size={14} className="mr-2" aria-hidden="true" />
            Profil Saya
          </Link>
        </DropdownMenuItem>

        <DropdownMenuItem asChild>
          <Link href="/pengaturan" className="cursor-pointer">
            <Settings size={14} className="mr-2" aria-hidden="true" />
            Pengaturan
          </Link>
        </DropdownMenuItem>

        <DropdownMenuSeparator />

        <DropdownMenuItem
          disabled={isPending}
          onSelect={(e) => {
            e.preventDefault();
            startTransition(() => logoutAction());
          }}
          className="cursor-pointer text-destructive focus:text-destructive"
        >
          {isPending ? (
            <Loader2
              size={14}
              className="mr-2 motion-safe:animate-spin"
              aria-hidden="true"
            />
          ) : (
            <LogOut size={14} className="mr-2" aria-hidden="true" />
          )}
          Keluar
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
