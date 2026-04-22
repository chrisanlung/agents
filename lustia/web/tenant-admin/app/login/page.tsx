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
    <main className="flex min-h-screen items-center justify-center bg-emerald-50 px-4 py-12">
      {/* Reads ?err= query param and fires a toast (wrapped in Suspense per Next.js requirement). */}
      <Suspense fallback={null}>
        <LoginErrorToast />
      </Suspense>

      <div className="w-full max-w-md space-y-6">
        <div className="flex flex-col items-center gap-3 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm">
            <Building2 size={24} aria-hidden="true" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight text-foreground">
              {appName}
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              Akses Administrator Tenant
            </p>
          </div>
        </div>

        <Card className="border-border shadow-sm">
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

        <p className="text-center text-xs text-muted-foreground">
          Akses terbatas untuk admin tenant yang berwenang.
        </p>
      </div>
    </main>
  );
}
