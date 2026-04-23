"use client";

import { useState } from "react";
import Link from "next/link";
import { CheckCircle2, Mail } from "lucide-react";

import { Button } from "@/components/ui/button";
import { RegisterForm } from "./register-form";

export function RegisterPanel() {
  const [successEmail, setSuccessEmail] = useState<string | null>(null);

  if (successEmail) {
    return (
      <div className="space-y-5 text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-emerald-100 text-emerald-700">
          <CheckCircle2 size={28} aria-hidden="true" />
        </div>
        <div>
          <h2 className="text-lg font-semibold text-foreground">Permintaan terkirim</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Tim kami akan meninjau pendaftaran Anda dalam 1&ndash;2 hari kerja.
          </p>
        </div>
        <div className="flex items-start gap-3 rounded-lg border bg-muted/40 p-4 text-left">
          <Mail size={18} className="mt-0.5 shrink-0 text-muted-foreground" aria-hidden="true" />
          <div className="text-sm">
            <p className="font-medium text-foreground">Apa selanjutnya?</p>
            <p className="mt-1 text-muted-foreground">
              Setelah disetujui, kami akan mengirim email ke{" "}
              <span className="font-medium text-foreground">{successEmail}</span> berisi
              kata sandi sementara untuk masuk ke portal admin tenant.
            </p>
          </div>
        </div>
        <Button asChild variant="outline" className="w-full">
          <Link href="/login">Kembali ke halaman masuk</Link>
        </Button>
      </div>
    );
  }

  return <RegisterForm onSuccess={setSuccessEmail} />;
}
