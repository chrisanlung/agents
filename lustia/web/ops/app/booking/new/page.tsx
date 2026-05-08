import type { Metadata } from "next";
import Link from "next/link";
import { ChevronLeft } from "lucide-react";

import { apiFetch } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import type {
  BranchListResponse,
  Branch,
  ServiceListResponse,
  Service,
  AddonListResponse,
  Addon,
  TherapistListResponse,
  Therapist,
} from "@/lib/types";
import { Button } from "@/components/ui/button";
import { ConciergeBookingForm } from "./concierge-booking-form";

export const metadata: Metadata = {
  title: "Buat Booking",
};

export default async function NewBookingPage() {
  let branches: Branch[] = [];
  let services: Service[] = [];
  let addons: Addon[] = [];
  let therapists: Therapist[] = [];

  try {
    const [branchRes, serviceRes, addonRes, therapistRes] =
      await Promise.allSettled([
        apiFetch<BranchListResponse>(
          "/tenant/branches?limit=200&status=active&scope=mine",
          {},
          { auth: true }
        ),
        apiFetch<ServiceListResponse>(
          "/tenant/services?limit=200&is_active=true",
          {},
          { auth: true }
        ),
        apiFetch<AddonListResponse>(
          "/tenant/addons?limit=200&is_active=true",
          {},
          { auth: true }
        ),
        apiFetch<TherapistListResponse>(
          "/tenant/therapists?limit=200&is_active=true",
          {},
          { auth: true }
        ),
      ]);

    if (branchRes.status === "fulfilled") branches = branchRes.value.data;
    if (serviceRes.status === "fulfilled") services = serviceRes.value.data;
    if (addonRes.status === "fulfilled") addons = addonRes.value.data;
    if (therapistRes.status === "fulfilled") therapists = therapistRes.value.data;
  } catch (err) {
    await handleApiError(err);
  }

  return (
    <div className="space-y-6">
      <div>
        <Button variant="ghost" size="sm" asChild className="-ml-1">
          <Link href="/booking">
            <ChevronLeft size={16} aria-hidden="true" />
            Kembali ke Daftar Booking
          </Link>
        </Button>
        <h1 className="mt-2 text-xl font-semibold text-foreground sm:text-2xl">
          Buat Booking
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Booking atas nama pelanggan (bayar di tempat)
        </p>
      </div>

      <ConciergeBookingForm
        branches={branches}
        services={services}
        addons={addons}
        therapists={therapists}
      />
    </div>
  );
}
