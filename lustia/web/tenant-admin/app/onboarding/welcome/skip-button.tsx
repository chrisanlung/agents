"use client";

import { useTransition } from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";

import { setSkipCookieAction } from "./skip-action";

export function SkipOnboardingButton() {
  const [isPending, startTransition] = useTransition();
  const router = useRouter();

  function handleSkip() {
    startTransition(async () => {
      await setSkipCookieAction();
      router.push("/dashboard");
    });
  }

  return (
    <button
      type="button"
      onClick={handleSkip}
      disabled={isPending}
      className="text-sm text-muted-foreground underline-offset-4 hover:underline disabled:opacity-50"
    >
      {isPending && (
        <Loader2
          size={12}
          className="mr-1 inline animate-spin"
          aria-hidden="true"
        />
      )}
      Nanti saja
    </button>
  );
}
