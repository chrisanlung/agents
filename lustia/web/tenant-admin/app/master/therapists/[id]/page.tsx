import { notFound, redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { AlertCircle, ArrowLeft, UserRound } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import type {
  Therapist,
  ServiceListResponse,
  Service,
  AvailabilityResponse,
} from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { TherapistForm } from "../therapist-form";
import { AvailabilityEditor } from "../availability-editor";
import { PrepMinutesCard } from "../prep-minutes-card";
import { ServiceMappingCombobox } from "./service-mapping-combobox";

// TEP-7 — detect placeholder / migration-default data.
function isPlaceholderProfile(t: Pick<Therapist, "height_cm" | "weight_kg" | "build" | "photo_url">): boolean {
  return (
    t.height_cm === 160 &&
    t.weight_kg === 60 &&
    t.build === "sedang" &&
    t.photo_url === null
  );
}

export const metadata: Metadata = {
  title: "Detail Terapis",
};

interface Props {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ tab?: string }>;
}

export default async function TherapistDetailPage({ params, searchParams }: Props) {
  const { id } = await params;
  const { tab = "profil" } = await searchParams;

  let therapist: Therapist;
  let allServices: Service[] = [];
  let availability: AvailabilityResponse = { therapist_id: id, windows: [] };

  try {
    const [therapistRes, servicesRes, availabilityRes] = await Promise.allSettled([
      apiFetch<Therapist>(`/tenant/therapists/${id}`, {}, { auth: true }),
      apiFetch<ServiceListResponse>("/tenant/services?limit=200", {}, { auth: true }),
      apiFetch<AvailabilityResponse>(
        `/tenant/therapists/${id}/availability`,
        {},
        { auth: true }
      ),
    ]);

    if (therapistRes.status === "rejected") {
      const err = therapistRes.reason;
      if (err instanceof ApiError && err.status === 401) redirect("/login");
      if (err instanceof ApiError && err.status === 404) notFound();
      throw err;
    }
    therapist = therapistRes.value;

    if (servicesRes.status === "fulfilled") {
      allServices = servicesRes.value.data;
    }

    if (availabilityRes.status === "fulfilled") {
      availability = availabilityRes.value;
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) redirect("/login");
    throw err;
  }

  const activeTab = ["profil", "layanan", "jadwal"].includes(tab)
    ? tab
    : "profil";

  return (
    <div className="space-y-6">
      <Link
        href="/master/therapists"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke Daftar Terapis
      </Link>

      {/* TEP-7 — amber banner when therapist has only migration-default data */}
      {isPlaceholderProfile(therapist) && (
        <div
          role="status"
          className="flex items-start gap-2 rounded-lg border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-800"
        >
          <AlertCircle size={16} className="mt-0.5 shrink-0 text-amber-600" aria-hidden="true" />
          <span>
            Profil belum dilengkapi — lengkapi data postur dan foto sebelum meluncurkan ke pelanggan.
          </span>
        </div>
      )}

      {/* Page header — TEP-6: 48×48 avatar beside the title */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3">
          {/* 48×48 identity anchor (read-only, upload controls are in the form) */}
          {therapist.photo_url ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={therapist.photo_url}
              alt={`Foto ${therapist.full_name}`}
              className="h-12 w-12 shrink-0 rounded-full object-cover"
            />
          ) : (
            <div
              aria-label={therapist.full_name}
              className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary"
            >
              {therapist.full_name
                .split(" ")
                .slice(0, 2)
                .map((w: string) => w[0]?.toUpperCase() ?? "")
                .join("") || <UserRound size={20} aria-hidden="true" />}
            </div>
          )}
          <div>
            <h1 className="text-xl font-semibold text-foreground">
              Detail Terapis
            </h1>
            <p className="mt-1 text-sm text-muted-foreground">
              {therapist.full_name}
            </p>
          </div>
        </div>
        <Badge variant={therapist.is_active ? "success" : "muted"}>
          {therapist.is_active ? "Aktif" : "Nonaktif"}
        </Badge>
      </div>

      {/* Tabs — URL-driven via ?tab= */}
      <Tabs defaultValue={activeTab}>
        <TabsList>
          <TabsTrigger value="profil" asChild>
            <Link
              href={`/master/therapists/${id}?tab=profil`}
              scroll={false}
            >
              Profil
            </Link>
          </TabsTrigger>
          <TabsTrigger value="layanan" asChild>
            <Link
              href={`/master/therapists/${id}?tab=layanan`}
              scroll={false}
            >
              Layanan
            </Link>
          </TabsTrigger>
          <TabsTrigger value="jadwal" asChild>
            <Link
              href={`/master/therapists/${id}?tab=jadwal`}
              scroll={false}
            >
              Jadwal
            </Link>
          </TabsTrigger>
        </TabsList>

        {/* Profil tab */}
        <TabsContent value="profil">
          <Card>
            <CardContent className="p-6">
              <TherapistForm therapist={therapist} />
            </CardContent>
          </Card>
        </TabsContent>

        {/* Layanan tab */}
        <TabsContent value="layanan">
          <Card>
            <CardContent className="p-6">
              <ServiceMappingCombobox
                therapistId={id}
                allServices={allServices.filter((s) => s.is_active)}
                assignedMappings={therapist.services ?? []}
              />
            </CardContent>
          </Card>
        </TabsContent>

        {/* Jadwal tab */}
        <TabsContent value="jadwal">
          <Card>
            <CardContent className="p-6">
              {/* PrepMinutesCard is above the weekly grid — per design spec §2 */}
              <div className="space-y-4">
                <PrepMinutesCard
                  therapistId={id}
                  initialPrepMinutes={therapist.prep_minutes}
                />
                <AvailabilityEditor
                  therapistId={id}
                  initialWindows={availability.windows}
                />
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
