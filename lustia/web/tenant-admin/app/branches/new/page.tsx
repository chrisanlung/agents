import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { BranchForm } from "../branch-form";

export const metadata: Metadata = {
  title: "Tambah Cabang Baru",
};

export default function NewBranchPage() {
  return (
    <div className="space-y-6">
      <Link
        href="/branches"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke daftar cabang
      </Link>

      <div>
        <h1 className="text-xl font-semibold text-foreground">
          Tambah Cabang Baru
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Isi informasi cabang baru. Cabang dapat diaktifkan atau dinonaktifkan
          kapan saja setelah dibuat.
        </p>
      </div>

      <Card>
        <CardContent className="p-6">
          <BranchForm />
        </CardContent>
      </Card>
    </div>
  );
}
