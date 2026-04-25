import { notFound, redirect } from "next/navigation";
import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { apiFetch, ApiError } from "@/lib/api";
import { handleApiError } from "@/lib/auth-guard";
import type { Room, BranchListResponse, Branch } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { RoomForm } from "../room-form";
import { RoomDetailActions } from "./room-detail-actions";

export const metadata: Metadata = {
  title: "Detail Ruangan",
};

interface Props {
  params: Promise<{ id: string }>;
}

export default async function RoomDetailPage({ params }: Props) {
  const { id } = await params;

  let room: Room;
  let branches: Branch[] = [];

  try {
    const [roomRes, branchRes] = await Promise.allSettled([
      apiFetch<Room>(`/tenant/rooms/${id}`, {}, { auth: true }),
      apiFetch<BranchListResponse>("/tenant/branches?limit=200", {}, { auth: true }),
    ]);

    if (roomRes.status === "rejected") {
      const err = roomRes.reason;
      if (err instanceof ApiError && err.status === 401) redirect("/login");
      if (err instanceof ApiError && err.status === 404) notFound();
      throw err;
    }

    room = roomRes.value;

    if (branchRes.status === "fulfilled") {
      branches = branchRes.value.data;
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) redirect("/login");
    if (err instanceof ApiError && err.status === 404) notFound();
    await handleApiError(err);
    throw err;
  }

  return (
    <div className="space-y-6">
      <Link
        href="/master/rooms"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" />
        Kembali ke Daftar Ruangan
      </Link>

      {/* Page header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-foreground">
            Detail Ruangan
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">{room.name}</p>
        </div>
        <Badge variant={room.is_active ? "success" : "muted"}>
          {room.is_active ? "Aktif" : "Nonaktif"}
        </Badge>
      </div>

      {/* Form card */}
      <Card>
        <CardContent className="p-6">
          <RoomForm room={room} branches={branches} />
        </CardContent>
      </Card>

      {/* Destructive actions */}
      <RoomDetailActions
        roomId={room.id}
        roomName={room.name}
        isActive={room.is_active}
      />
    </div>
  );
}
