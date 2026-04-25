"use client";

/**
 * BookingBarChart — recharts BarChart for bookings-per-branch.
 *
 * Accessibility: an sr-only companion <table> duplicates the data for
 * screen readers and keyboard-only users (BK-C4 spec).
 *
 * prefers-reduced-motion: animationDuration set to 0 when the OS reduces motion.
 */

import { useMemo } from "react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import type { BookingReportBranch } from "@/lib/types";

interface BookingBarChartProps {
  data: BookingReportBranch[];
  from: string;
  to: string;
}

function formatPrice(idr: number) {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(idr);
}

interface TooltipPayloadItem {
  payload?: BookingReportBranch;
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: TooltipPayloadItem[];
  label?: string;
}

function CustomTooltip({ active, payload, label }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;
  const d = payload[0]?.payload;
  if (!d) return null;
  return (
    <div className="rounded-md border border-border bg-popover p-3 text-sm shadow-md">
      <p className="mb-1 font-semibold text-foreground">{label}</p>
      <p className="text-muted-foreground">
        Booking:{" "}
        <span className="font-medium text-foreground">{d.total_bookings}</span>
      </p>
      <p className="text-muted-foreground">
        Pendapatan:{" "}
        <span className="font-medium text-emerald-600">
          {formatPrice(d.total_revenue_idr)}
        </span>
      </p>
    </div>
  );
}

export function BookingBarChart({ data, from, to }: BookingBarChartProps) {
  const reducedMotion = useMemo(() => {
    if (typeof window === "undefined") return false;
    return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  }, []);

  return (
    <div>
      {/* Visual chart */}
      <ResponsiveContainer width="100%" height={240}>
        <BarChart data={data} margin={{ top: 8, right: 8, bottom: 8, left: 8 }}>
          <XAxis
            dataKey="branch_name"
            tick={{ fontSize: 12 }}
            tickLine={false}
            axisLine={false}
          />
          <YAxis
            tick={{ fontSize: 12 }}
            tickLine={false}
            axisLine={false}
            allowDecimals={false}
          />
          <Tooltip content={<CustomTooltip />} />
          <Bar
            dataKey="total_bookings"
            fill="#10B981"
            radius={[4, 4, 0, 0]}
            animationDuration={reducedMotion ? 0 : 400}
          />
        </BarChart>
      </ResponsiveContainer>

      {/* sr-only companion table (BK-C4 accessibility requirement) */}
      <table className="sr-only">
        <caption>
          Booking per cabang, periode {from}–{to}
        </caption>
        <thead>
          <tr>
            <th scope="col">Cabang</th>
            <th scope="col">Jumlah Booking</th>
            <th scope="col">Total Pendapatan</th>
            <th scope="col">No-show</th>
          </tr>
        </thead>
        <tbody>
          {data.map((row) => (
            <tr key={row.branch_id}>
              <td>{row.branch_name}</td>
              <td>{row.total_bookings}</td>
              <td>{formatPrice(row.total_revenue_idr)}</td>
              <td>{row.no_show_count}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
