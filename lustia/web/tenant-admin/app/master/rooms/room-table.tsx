"use client";

import { DoorOpen } from "lucide-react";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { RoomRowActions } from "./room-row-actions";
import { RoomReorderButtons, useRoomReorder } from "./room-reorder-buttons";
import { ROOM_TYPE_LABELS } from "./room-form";
import type { Room, Branch } from "@/lib/types";

interface RoomTableProps {
  initialRooms: Room[];
  branches: Branch[];
  /** Show branch column when no single branch filter is active */
  showBranchColumn: boolean;
  /** When a single branch is filtered and this is a single-page result */
  showReorderColumn: boolean;
  /** Active branch_id filter — required when showReorderColumn is true */
  branchId?: string;
}

/**
 * Client component owning the optimistic reorder state.
 * When `showReorderColumn` is false, reorder controls are absent from the DOM
 * entirely (RM-12: not just hidden, so not keyboard-reachable).
 */
export function RoomTable({
  initialRooms,
  branches,
  showBranchColumn,
  showReorderColumn,
  branchId,
}: RoomTableProps) {
  const { rooms, isReordering, handleMove } = useRoomReorder(
    initialRooms,
    branchId ?? ""
  );

  return (
    <Table className="min-w-[600px]">
      <TableHeader className="bg-muted/30">
        <TableRow>
          <TableHead className="w-[48px]">Foto</TableHead>
          <TableHead>Nama</TableHead>
          {showBranchColumn && <TableHead>Cabang</TableHead>}
          <TableHead className="w-[100px]">Tipe</TableHead>
          <TableHead className="w-[90px] text-center">Kapasitas</TableHead>
          <TableHead className="w-[90px]">Status</TableHead>
          {showReorderColumn && (
            <TableHead className="w-[96px] text-center">Urutan</TableHead>
          )}
          <TableHead className="w-[80px]">Aksi</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody className="text-sm">
        {rooms.map((room, index) => {
          const branchName = branches.find((b) => b.id === room.branch_id)?.name;
          return (
            <TableRow key={room.id} className="h-14">
              {/* Foto */}
              <TableCell className="w-[48px]">
                {room.photo_url ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={room.photo_url}
                    alt={`Foto ruangan ${room.name}`}
                    className={`h-10 w-10 shrink-0 rounded-md object-cover ${
                      !room.is_active ? "opacity-60" : ""
                    }`}
                  />
                ) : (
                  <div
                    aria-label="Belum ada foto"
                    className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-muted/40 ${
                      !room.is_active ? "opacity-60" : ""
                    }`}
                  >
                    <DoorOpen
                      size={16}
                      className="text-muted-foreground/50"
                      aria-hidden="true"
                    />
                  </div>
                )}
              </TableCell>

              {/* Nama */}
              <TableCell className="min-w-[160px]">
                <span
                  className={
                    room.is_active
                      ? "font-medium"
                      : "font-medium line-through text-muted-foreground"
                  }
                >
                  {room.name}
                </span>
              </TableCell>

              {/* Cabang */}
              {showBranchColumn && (
                <TableCell>
                  {branchName ?? (
                    <span className="text-muted-foreground">—</span>
                  )}
                </TableCell>
              )}

              {/* Tipe */}
              <TableCell className="w-[100px]">
                <Badge variant="outline">
                  {ROOM_TYPE_LABELS[room.room_type] ?? room.room_type}
                </Badge>
              </TableCell>

              {/* Kapasitas */}
              <TableCell className="w-[90px] text-center tabular-nums">
                {room.capacity}
              </TableCell>

              {/* Status */}
              <TableCell className="w-[90px]">
                <Badge variant={room.is_active ? "success" : "muted"}>
                  {room.is_active ? "Aktif" : "Nonaktif"}
                </Badge>
              </TableCell>

              {/* Urutan — only when single branch filter is active (RM-5) */}
              {showReorderColumn && (
                <TableCell className="w-[96px]">
                  <RoomReorderButtons
                    rooms={rooms}
                    index={index}
                    total={rooms.length}
                    isReordering={isReordering}
                    onMove={handleMove}
                  />
                </TableCell>
              )}

              {/* Aksi */}
              <TableCell className="w-[80px]">
                <RoomRowActions room={room} />
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}
