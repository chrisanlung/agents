import { redirect } from "next/navigation";
import { cookies } from "next/headers";
import type { Metadata } from "next";
import Link from "next/link";
import { Building2, MapPin, ArrowRight } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import type { OnboardingState } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { AppBackground } from "@/components/app-background";
import { SkipOnboardingButton } from "./skip-button";

export const metadata: Metadata = {
  title: "Selamat Datang di Lustia",
};

const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Tenant Portal";

export default async function OnboardingWelcomePage() {
  // If the user already has branches, redirect back to dashboard
  try {
    const state = await apiFetch<OnboardingState>(
      "/tenant/onboarding-state",
      {},
      { auth: true }
    );
    if (state.has_branches) {
      // Branches exist — clear any lingering skip cookie and go to dashboard
      redirect("/dashboard");
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      redirect("/login");
    }
    // If the endpoint doesn't exist yet, continue showing the onboarding page
  }

  return (
    <AppBackground>
      <main className="flex min-h-screen items-center justify-center px-4 py-12">
      <div className="w-full max-w-lg space-y-6">
        {/* Branding */}
        <div className="flex flex-col items-center gap-4 text-center">
          <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-md">
            <Building2 size={32} aria-hidden="true" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-foreground">
              Selamat datang di {appName}!
            </h1>
            <p className="mt-2 text-base text-muted-foreground">
              Akun Anda sudah siap. Langkah selanjutnya adalah menambahkan
              cabang pertama untuk mulai mengoperasikan bisnis Anda.
            </p>
          </div>
        </div>

        {/* CTA card */}
        <Card className="shadow-sm">
          <CardContent className="space-y-5 p-6">
            <div className="flex items-start gap-4">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                <MapPin size={20} className="text-primary" aria-hidden="true" />
              </div>
              <div>
                <h2 className="text-sm font-semibold text-foreground">
                  Tambah Cabang Pertama
                </h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  Masukkan informasi lokasi, kontak, dan jam operasional cabang
                  Anda. Cabang dapat diaktifkan atau dinonaktifkan kapan saja.
                </p>
              </div>
            </div>

            <div className="flex flex-col gap-3">
              <Button asChild className="w-full" size="lg">
                <Link href="/branches/new">
                  Tambah Cabang Pertama
                  <ArrowRight size={16} aria-hidden="true" />
                </Link>
              </Button>
              <div className="flex justify-center">
                <SkipOnboardingButton />
              </div>
            </div>
          </CardContent>
        </Card>

        <p className="text-center text-xs text-muted-foreground">
          Anda dapat kembali ke halaman ini kapan saja dari menu dasbor.
        </p>
      </div>
      </main>
    </AppBackground>
  );
}
