"use client";

import Link from "next/link";
import { useTransition } from "react";
import { LogOut, Settings, KeyRound, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { logoutAction } from "@/app/dashboard/actions";

interface UserMenuProps {
  fullName: string;
  email: string;
}

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].charAt(0).toUpperCase();
  return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase();
}

export function UserMenu({ fullName, email }: UserMenuProps) {
  const [isPending, startTransition] = useTransition();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="h-9 w-9 rounded-full bg-white/80 p-0 text-sm font-semibold text-primary shadow-sm ring-1 ring-pink-200 hover:bg-white"
          aria-label={`Menu akun ${fullName}`}
        >
          {initials(fullName)}
        </Button>
      </DropdownMenuTrigger>

      <DropdownMenuContent align="end" className="w-60">
        <div className="px-2 py-2">
          <p className="truncate text-sm font-semibold text-foreground">
            {fullName}
          </p>
          <p className="truncate text-xs text-muted-foreground">{email}</p>
        </div>

        <DropdownMenuSeparator />

        <DropdownMenuItem asChild>
          <Link href="/settings" className="cursor-pointer">
            <Settings size={14} className="mr-2" aria-hidden="true" />
            Pengaturan
          </Link>
        </DropdownMenuItem>

        <DropdownMenuItem asChild>
          <Link href="/pengaturan/ubah-kata-sandi" className="cursor-pointer">
            <KeyRound size={14} className="mr-2" aria-hidden="true" />
            Ubah Kata Sandi
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
            <Loader2 size={14} className="mr-2 animate-spin" aria-hidden="true" />
          ) : (
            <LogOut size={14} className="mr-2" aria-hidden="true" />
          )}
          Keluar
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
