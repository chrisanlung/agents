import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { Card, CardContent } from "@/components/ui/card";
import { ServiceForm } from "../service-form";

export const metadata: Metadata = {
  title: "Tambah Layanan",
};

export default function NewServicePage() {
  return (
    <div className="space-y-6">
      <Link
        href="/master/services"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke Daftar Layanan
      </Link>

      <div>
        <h1 className="text-xl font-semibold text-foreground">Tambah Layanan</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Isi detail layanan baru
        </p>
      </div>

      <Card>
        <CardContent className="p-6">
          <ServiceForm />
        </CardContent>
      </Card>
    </div>
  );
}
