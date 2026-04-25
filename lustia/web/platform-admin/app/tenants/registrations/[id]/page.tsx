import type { Metadata } from "next";
import type React from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { apiFetch } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import type { TenantRegistration } from "@/lib/types";
import { relativeTime } from "@/lib/relative-time";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { ApprovalPanel } from "./approval-panel";

export const metadata: Metadata = {
  title: "Tinjau Registrasi",
};

interface Props {
  params: Promise<{ id: string }>;
}

const STATUS_LABELS: Record<string, string> = {
  pending: "Menunggu",
  approved: "Disetujui",
  rejected: "Ditolak",
};

const STATUS_VARIANTS: Record<
  string,
  "default" | "success" | "destructive" | "warning" | "secondary" | "muted" | "outline"
> = {
  pending: "warning",
  approved: "success",
  rejected: "destructive",
};

export default async function RegistrationDetailPage({ params }: Props) {
  const { id } = await params;

  let reg: TenantRegistration;
  try {
    reg = await apiFetch<TenantRegistration>(
      `/admin/tenant-registrations/${id}`,
      {},
      { auth: true }
    );
  } catch (err) {
    await handleApiError(err);
    throw err; // unreachable — handleApiError redirects or throws; this line
               // exists solely to help TS understand `reg` is definitely assigned.
  }

  const isPending = reg.status === "pending";

  return (
    <div className="space-y-6">
      {/* Back link */}
      <Link
        href="/tenants/registrations"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke antrian
      </Link>

      <div className="flex items-center gap-3">
        <div>
          <h1 className="text-xl font-semibold text-foreground">
            {reg.company_name}
          </h1>
          <p className="text-sm text-muted-foreground">
            ID: <code className="text-xs">{reg.id}</code>
          </p>
        </div>
        <Badge variant={STATUS_VARIANTS[reg.status] ?? "secondary"}>
          {STATUS_LABELS[reg.status] ?? reg.status}
        </Badge>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Left — registration details (read-only) */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Detail Registrasi</CardTitle>
            <CardDescription>
              Diajukan {relativeTime(reg.created_at)}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <dl className="space-y-4">
              <Field label="Nama Perusahaan" value={reg.company_name} />
              <Field
                label="Slug yang Diminta"
                value={
                  <code className="rounded bg-muted px-1.5 py-0.5 text-xs">
                    {reg.requested_slug}
                  </code>
                }
              />
              <Field label="Paket" value={<span className="capitalize">{reg.package}</span>} />
              <Field label="Nama Kontak" value={reg.contact_name} />
              <Field label="Email Kontak" value={reg.contact_email} />
              {reg.contact_phone && (
                <Field label="Telepon Kontak" value={reg.contact_phone} />
              )}
              <Field
                label="Tanggal Diajukan"
                value={new Date(reg.created_at).toLocaleString("id-ID", {
                  dateStyle: "long",
                  timeStyle: "short",
                })}
              />
              {reg.status === "approved" && reg.approved_at && (
                <Field
                  label="Disetujui Pada"
                  value={new Date(reg.approved_at).toLocaleString("id-ID", {
                    dateStyle: "long",
                    timeStyle: "short",
                  })}
                />
              )}
              {reg.status === "rejected" && (
                <>
                  {reg.rejected_at && (
                    <Field
                      label="Ditolak Pada"
                      value={new Date(reg.rejected_at).toLocaleString("id-ID", {
                        dateStyle: "long",
                        timeStyle: "short",
                      })}
                    />
                  )}
                  {reg.rejection_reason && (
                    <Field
                      label="Alasan Penolakan"
                      value={
                        <p className="whitespace-pre-wrap text-sm text-destructive">
                          {reg.rejection_reason}
                        </p>
                      }
                    />
                  )}
                </>
              )}
            </dl>
          </CardContent>
        </Card>

        {/* Right — action panel (only shown for pending) */}
        {isPending ? (
          <ApprovalPanel
            registrationId={reg.id}
            defaultPackage={reg.package}
            contactEmail={reg.contact_email}
          />
        ) : (
          <Card className="border-dashed">
            <CardContent className="flex items-center justify-center py-16 text-center">
              <p className="text-sm text-muted-foreground">
                Registrasi ini sudah{" "}
                <strong>{STATUS_LABELS[reg.status]?.toLowerCase() ?? reg.status}</strong>.
                Tidak ada tindakan lebih lanjut yang tersedia.
              </p>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}

function Field({
  label,
  value,
}: {
  label: string;
  value: React.ReactNode;
}) {
  return (
    <div>
      <dt className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </dt>
      <dd className="mt-1 text-sm text-foreground">
        {typeof value === "string" ? value : value}
      </dd>
    </div>
  );
}
