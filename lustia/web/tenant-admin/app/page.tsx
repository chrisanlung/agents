import Link from "next/link";
import { redirect } from "next/navigation";
import { cookies } from "next/headers";
import { Building2, CheckCircle2, Sparkles, ShieldCheck, LogIn, UserPlus } from "lucide-react";

import { Button } from "@/components/ui/button";

const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Tenant Portal";

export default async function RootPage() {
  const cookieStore = await cookies();
  const hasSession = cookieStore.has("access_token");
  if (hasSession) {
    redirect("/dashboard");
  }

  return (
    <main
      className="relative min-h-screen overflow-hidden bg-cover bg-center bg-no-repeat"
      style={{ backgroundImage: "url('/bg-lustia-tenant.webp')" }}
    >
      {/* Overlay — tone down the background so text stays readable */}
      <div
        aria-hidden="true"
        className="absolute inset-0 bg-gradient-to-br from-emerald-950/60 via-emerald-900/40 to-teal-800/25"
      />

      <div className="relative z-10 flex min-h-screen flex-col">
        {/* Top bar */}
        <header className="flex items-center justify-between px-6 py-5 sm:px-10">
          <div className="flex items-center gap-3 text-white">
            <div className="flex h-9 w-9 items-center justify-center rounded-xl bg-white/10 backdrop-blur-sm ring-1 ring-white/20">
              <Building2 size={18} aria-hidden="true" />
            </div>
            <span className="font-semibold tracking-tight">{appName}</span>
          </div>

          <nav className="flex items-center gap-2 sm:gap-3">
            <Button
              asChild
              variant="ghost"
              className="text-white hover:bg-white/10 hover:text-white"
            >
              <Link href="/login">
                <LogIn className="mr-1.5" size={16} aria-hidden="true" />
                Masuk
              </Link>
            </Button>
            <Button
              asChild
              className="bg-white text-emerald-900 shadow-sm hover:bg-emerald-50"
            >
              <Link href="/register">
                <UserPlus className="mr-1.5" size={16} aria-hidden="true" />
                Daftar
              </Link>
            </Button>
          </nav>
        </header>

        {/* Hero */}
        <section className="flex flex-1 items-center px-6 py-12 sm:px-10">
          <div className="mx-auto grid w-full max-w-6xl gap-12 lg:grid-cols-2 lg:items-center">
            <div className="space-y-6 text-white">
              <div className="inline-flex items-center gap-2 rounded-full border border-white/20 bg-white/10 px-3 py-1 text-xs font-medium backdrop-blur-sm">
                <Sparkles size={14} aria-hidden="true" />
                Platform manajemen spa &amp; terapi modern
              </div>

              <h1 className="text-4xl font-semibold leading-tight tracking-tight sm:text-5xl">
                Kelola seluruh cabang spa Anda{" "}
                <span className="text-emerald-200">dalam satu tempat</span>
              </h1>

              <p className="text-lg text-emerald-50/90 sm:text-xl">
                Booking, jadwal terapis, kasir, dan laporan operasional &mdash; semua
                terintegrasi, terisolasi per tenant, dan siap berkembang bersama bisnis
                Anda.
              </p>

              <ul className="space-y-3 text-emerald-50/90">
                <BulletItem>Multi-cabang dengan pembatasan akses per peran</BulletItem>
                <BulletItem>Data tenant terisolasi penuh (RLS tingkat database)</BulletItem>
                <BulletItem>Siap pakai dalam hitungan menit setelah disetujui</BulletItem>
              </ul>

              <div className="flex flex-col gap-3 pt-2 sm:flex-row">
                <Button
                  asChild
                  size="lg"
                  className="bg-white text-emerald-900 shadow-md hover:bg-emerald-50"
                >
                  <Link href="/register">
                    <UserPlus className="mr-2" size={18} aria-hidden="true" />
                    Daftarkan Perusahaan Anda
                  </Link>
                </Button>
                <Button
                  asChild
                  size="lg"
                  variant="outline"
                  className="border-white/30 bg-transparent text-white hover:bg-white/10 hover:text-white"
                >
                  <Link href="/login">Sudah punya akun?</Link>
                </Button>
              </div>
            </div>

            {/* Right column — value cards */}
            <div className="grid gap-4 sm:grid-cols-2">
              <FeatureCard
                icon={<Building2 size={20} aria-hidden="true" />}
                title="Multi-cabang"
                body="Pantau operasional setiap outlet dalam satu dashboard terpusat."
              />
              <FeatureCard
                icon={<ShieldCheck size={20} aria-hidden="true" />}
                title="Aman &amp; terisolasi"
                body="Setiap tenant punya ruang data sendiri, dilindungi Row-Level Security."
              />
              <FeatureCard
                icon={<Sparkles size={20} aria-hidden="true" />}
                title="Siap berkembang"
                body="Mulai dari paket starter, upgrade saat bisnis Anda bertumbuh."
              />
              <FeatureCard
                icon={<CheckCircle2 size={20} aria-hidden="true" />}
                title="Onboarding cepat"
                body="Setup cabang pertama Anda langsung setelah pendaftaran disetujui."
              />
            </div>
          </div>
        </section>

        <footer className="px-6 pb-6 text-center text-xs text-emerald-100/70 sm:px-10">
          &copy; {new Date().getFullYear()} Lustia. Dirancang untuk bisnis spa &amp;
          terapi modern.
        </footer>
      </div>
    </main>
  );
}

function BulletItem({ children }: { children: React.ReactNode }) {
  return (
    <li className="flex items-start gap-3">
      <CheckCircle2
        size={20}
        className="mt-0.5 shrink-0 text-emerald-200"
        aria-hidden="true"
      />
      <span>{children}</span>
    </li>
  );
}

function FeatureCard({
  icon,
  title,
  body,
}: {
  icon: React.ReactNode;
  title: string;
  body: string;
}) {
  return (
    <div className="rounded-xl border border-white/15 bg-white/10 p-5 text-white backdrop-blur-sm transition hover:bg-white/15">
      <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-white/10 ring-1 ring-white/20">
        {icon}
      </div>
      <h3 className="text-base font-semibold">{title}</h3>
      <p className="mt-1 text-sm text-emerald-50/80">{body}</p>
    </div>
  );
}
