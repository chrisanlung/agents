import { notFound, redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import type { Service } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ServiceForm } from "../service-form";

export const metadata: Metadata = {
  title: "Detail Layanan",
};

interface Props {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ tab?: string }>;
}

export default async function ServiceDetailPage({ params, searchParams }: Props) {
  const { id } = await params;
  const { tab = "detail" } = await searchParams;

  let service: Service;

  try {
    service = await apiFetch<Service>(
      `/tenant/services/${id}`,
      {},
      { auth: true }
    );
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) redirect("/login");
    if (err instanceof ApiError && err.status === 404) notFound();
    throw err;
  }

  const activeTab = ["detail", "terapis"].includes(tab) ? tab : "detail";

  return (
    <div className="space-y-6">
      <Link
        href="/master/services"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke Daftar Layanan
      </Link>

      {/* Page header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-foreground">
            Detail Layanan
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">{service.name}</p>
        </div>
        <Badge variant={service.is_active ? "success" : "muted"}>
          {service.is_active ? "Aktif" : "Nonaktif"}
        </Badge>
      </div>

      {/* Tabs */}
      <Tabs defaultValue={activeTab}>
        <TabsList>
          <TabsTrigger value="detail" asChild>
            <Link href={`/master/services/${id}?tab=detail`} scroll={false}>
              Detail
            </Link>
          </TabsTrigger>
          <TabsTrigger value="terapis" asChild>
            <Link href={`/master/services/${id}?tab=terapis`} scroll={false}>
              Terapis
            </Link>
          </TabsTrigger>
        </TabsList>

        {/* Detail tab */}
        <TabsContent value="detail">
          <Card>
            <CardContent className="p-6">
              <ServiceForm service={service} />
            </CardContent>
          </Card>
        </TabsContent>

        {/* Terapis tab — read-only */}
        <TabsContent value="terapis">
          <Card>
            <CardContent className="p-6">
              <div className="space-y-4">
                <p className="text-xs text-muted-foreground">
                  Penugasan dikelola dari halaman masing-masing terapis.
                </p>
                {!service.therapists || service.therapists.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    Belum ada terapis yang menawarkan layanan ini.
                  </p>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Nama</TableHead>
                        <TableHead>Cabang</TableHead>
                        <TableHead>Status Penugasan</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {service.therapists.map((t) => (
                        <TableRow key={t.therapist_id}>
                          <TableCell>
                            <Link
                              href={`/master/therapists/${t.therapist_id}?tab=layanan`}
                              className="font-medium text-foreground underline-offset-2 hover:underline"
                            >
                              {t.full_name}
                            </Link>
                          </TableCell>
                          <TableCell className="text-sm text-muted-foreground">
                            {t.branch_name}
                          </TableCell>
                          <TableCell>
                            <Badge variant={t.is_active ? "success" : "muted"}>
                              {t.is_active ? "Aktif" : "Nonaktif"}
                            </Badge>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
