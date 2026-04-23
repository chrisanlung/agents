import { notFound, redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

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
import { ServiceMappingCombobox } from "./service-mapping-combobox";

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

  const activeTab = ["profil", "layanan", "ketersediaan"].includes(tab)
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

      {/* Page header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-foreground">
            Detail Terapis
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            {therapist.full_name}
          </p>
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
          <TabsTrigger value="ketersediaan" asChild>
            <Link
              href={`/master/therapists/${id}?tab=ketersediaan`}
              scroll={false}
            >
              Ketersediaan
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

        {/* Ketersediaan tab */}
        <TabsContent value="ketersediaan">
          <Card>
            <CardContent className="p-6">
              <AvailabilityEditor
                therapistId={id}
                initialWindows={availability.windows}
              />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
