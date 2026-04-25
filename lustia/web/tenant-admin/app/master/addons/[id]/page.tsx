import { notFound, redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import type { Addon } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { AddonForm } from "../addon-form";
import { AddonDetailActions } from "./addon-detail-actions";

export const metadata: Metadata = {
  title: "Detail Add-on",
};

interface Props {
  params: Promise<{ id: string }>;
}

export default async function AddonDetailPage({ params }: Props) {
  const { id } = await params;

  let addon: Addon;

  try {
    addon = await apiFetch<Addon>(
      `/tenant/addons/${id}`,
      {},
      { auth: true }
    );
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) redirect("/login");
    if (err instanceof ApiError && err.status === 404) notFound();
    throw err;
  }

  return (
    <div className="space-y-6">
      <Link
        href="/master/addons"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke Daftar Add-on
      </Link>

      {/* Page header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-foreground">Detail Add-on</h1>
          <p className="mt-1 text-sm text-muted-foreground">{addon.name}</p>
        </div>
        <Badge variant={addon.is_active ? "success" : "muted"}>
          {addon.is_active ? "Aktif" : "Nonaktif"}
        </Badge>
      </div>

      {/* Form card */}
      <Card>
        <CardContent className="p-6">
          <AddonForm addon={addon} />
        </CardContent>
      </Card>

      {/* Destructive actions */}
      <AddonDetailActions
        addonId={addon.id}
        addonName={addon.name}
        isActive={addon.is_active}
      />
    </div>
  );
}
