"use client";

import { useTransition } from "react";
import { LogOut, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { logoutAction } from "./actions";

export function SignOutButton() {
  const [isPending, startTransition] = useTransition();

  return (
    <Button
      variant="outline"
      size="sm"
      disabled={isPending}
      aria-busy={isPending}
      onClick={() => startTransition(() => logoutAction())}
    >
      {isPending ? (
        <Loader2 className="animate-spin" aria-hidden="true" />
      ) : (
        <LogOut size={16} aria-hidden="true" />
      )}
      Keluar
    </Button>
  );
}
