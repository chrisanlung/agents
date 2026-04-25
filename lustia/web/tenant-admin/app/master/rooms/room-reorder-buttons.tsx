"use client";

import { useState } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { reorderRooms } from "./actions";
import type { Room } from "@/lib/types";

interface RoomReorderButtonsProps {
  /** All rooms on the current page (branch-filtered, RM-5). */
  rooms: Room[];
  /** Index of this room in the list. */
  index: number;
  /** Total number of rooms on the current page. */
  total: number;
  /** Whether a reorder is in flight (shared across all rows). */
  isReordering: boolean;
  /** Callback to move a room; parent owns the optimistic state. */
  onMove: (index: number, direction: "up" | "down") => void;
}

export function RoomReorderButtons({
  rooms,
  index,
  total,
  isReordering,
  onMove,
}: RoomReorderButtonsProps) {
  const room = rooms[index];
  const isFirst = index === 0;
  const isLast = index === total - 1;

  return (
    <div className="flex items-center justify-center gap-0.5">
      <Button
        variant="ghost"
        size="icon"
        className="h-7 w-7"
        onClick={() => onMove(index, "up")}
        disabled={isFirst || isReordering}
        aria-label={`Pindah ke atas: ${room.name}`}
      >
        <ChevronUp size={14} aria-hidden="true" />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        className="h-7 w-7"
        onClick={() => onMove(index, "down")}
        disabled={isLast || isReordering}
        aria-label={`Pindah ke bawah: ${room.name}`}
      >
        <ChevronDown size={14} aria-hidden="true" />
      </Button>
    </div>
  );
}

// ─── Hook: manages optimistic reorder state for a page ───────────────────────

/**
 * useRoomReorder — owns the optimistic list state for the current page.
 * Calls reorderRooms on every swap; rolls back on failure.
 */
export function useRoomReorder(initialRooms: Room[], branchId: string) {
  const [rooms, setRooms] = useState<Room[]>(initialRooms);
  const [isReordering, setIsReordering] = useState(false);

  async function handleMove(index: number, direction: "up" | "down") {
    const targetIndex = direction === "up" ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= rooms.length) return;

    // Snapshot for rollback (lesson from addon code-review)
    const prev = rooms;

    // Optimistic swap
    const next = [...rooms];
    [next[index], next[targetIndex]] = [next[targetIndex], next[index]];
    setRooms(next);
    setIsReordering(true);

    try {
      const items = next.map((r, i) => ({ id: r.id, sort_order: i }));
      const result = await reorderRooms(branchId, items);
      if (!result.ok) {
        setRooms(prev);
        toast.error("Gagal mengubah urutan. Coba lagi.");
      }
    } catch {
      setRooms(prev);
      toast.error("Gagal mengubah urutan. Coba lagi.");
    } finally {
      setIsReordering(false);
    }
  }

  return { rooms, isReordering, handleMove };
}
