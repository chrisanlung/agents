import Link from "next/link";
import { Suspense } from "react";
import type { Metadata } from "next";
import { Building2 } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { LoginForm } from "./login-form";
import { LoginErrorToast } from "./login-error-toast";

export const metadata: Metadata = {
  title: "Masuk",
};

const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Tenant Portal";

export default function LoginPage() {
  return (
    <main
      className="relative flex min-h-screen items-center justify-center overflow-hidden bg-cover bg-center bg-no-repeat px-4 py-12"
      style={{ backgroundImage: "url('/bg-lustia-login-tenant.webp')" }}
    >
      {/* Overlay — keeps the card legible over the photo background */}
      <div
        aria-hidden="true"
        className="absolute inset-0 bg-gradient-to-br from-emerald-950/55 via-emerald-900/35 to-teal-800/20"
      />

      {/* Reads ?err= query param and fires a toast (wrapped in Suspense per Next.js requirement). */}
      <Suspense fallback={null}>
        <LoginErrorToast />
      </Suspense>

      <div className="relative z-10 w-full max-w-md space-y-6">
        <Link
          href="/"
          className="flex flex-col items-center gap-3 text-center outline-none transition hover:opacity-90 focus-visible:ring-2 focus-visible:ring-white/50 focus-visible:ring-offset-2 focus-visible:ring-offset-transparent"
          aria-label="Kembali ke beranda"
        >
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-white/10 text-white shadow-sm ring-1 ring-white/25 backdrop-blur-sm">
            <Building2 size={24} aria-hidden="true" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight text-white">
              {appName}
            </h1>
            <p className="mt-1 text-sm text-emerald-50/80">
              Akses Administrator Tenant
            </p>
          </div>
        </Link>

        <Card className="border-border shadow-lg">
          <CardHeader className="pb-4">
            <CardTitle className="text-lg">Masuk ke akun Anda</CardTitle>
            <CardDescription>
              Masukkan kredensial Anda untuk melanjutkan.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <LoginForm />
          </CardContent>
        </Card>

        <p className="text-center text-sm text-emerald-50/90">
          Belum memiliki akun perusahaan?{" "}
          <Link
            href="/register"
            className="font-medium text-white underline-offset-4 hover:underline"
          >
            Daftarkan perusahaan Anda
          </Link>
        </p>

        <p className="text-center text-xs text-emerald-50/70">
          Akses terbatas untuk admin tenant yang berwenang.
        </p>
      </div>
    </main>
  );
}
