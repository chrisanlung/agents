import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { Card, CardContent } from "@/components/ui/card";
import { AddonForm } from "../addon-form";

export const metadata: Metadata = {
  title: "Tambah Add-on",
};

export default function NewAddonPage() {
  return (
    <div className="space-y-6">
      <Link
        href="/master/addons"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke Daftar Add-on
      </Link>

      <div>
        <h1 className="text-xl font-semibold text-foreground">Tambah Add-on</h1>
        <p className="mt-1 text-sm text-muted-foreground">Isi detail add-on baru</p>
      </div>

      <Card>
        <CardContent className="p-6">
          <AddonForm />
        </CardContent>
      </Card>
    </div>
  );
}
