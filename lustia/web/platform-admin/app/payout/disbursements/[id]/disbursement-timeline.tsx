import { cn } from "@/lib/utils";
import { relativeTime } from "@/lib/relative-time";
import type { DisbursementStatus } from "@/lib/types";

interface TimelineStep {
  status: DisbursementStatus;
  label: string;
  timestamp?: string | null;
  adminName?: string | null;
}

interface DisbursementTimelineProps {
  currentStatus: DisbursementStatus;
  createdAt: string;
  transferredAt: string | null;
  transferredByName: string | null;
}

function isStepCompleted(
  current: DisbursementStatus,
  step: DisbursementStatus
): boolean {
  const order: DisbursementStatus[] = ["pending", "processing", "transferred"];
  const currentIdx = order.indexOf(current);
  const stepIdx = order.indexOf(step);
  return stepIdx < currentIdx;
}

function isStepActive(
  current: DisbursementStatus,
  step: DisbursementStatus
): boolean {
  return current === step;
}

export function DisbursementTimeline({
  currentStatus,
  createdAt,
  transferredAt,
  transferredByName,
}: DisbursementTimelineProps) {
  const steps: TimelineStep[] = [
    {
      status: "pending",
      label: "Menunggu",
      timestamp: createdAt,
      adminName: null,
    },
    {
      status: "processing",
      label: "Sedang Diproses",
      timestamp:
        currentStatus === "processing" ||
        currentStatus === "transferred"
          ? createdAt // fallback — processing timestamp not in DTO currently
          : null,
      adminName: null,
    },
    {
      status: "transferred",
      label: "Sudah Ditransfer",
      timestamp: transferredAt,
      adminName: transferredByName,
    },
  ];

  return (
    <ol className="relative border-l border-border space-y-6 ml-3">
      {steps.map((step) => {
        const completed = isStepCompleted(currentStatus, step.status);
        const active = isStepActive(currentStatus, step.status);
        return (
          <li key={step.status} className="relative pl-6">
            {/* dot */}
            <span
              className={cn(
                "absolute -left-[5px] top-1 h-2.5 w-2.5 rounded-full border-2",
                completed
                  ? "bg-primary border-primary"
                  : active
                  ? "bg-primary/30 border-primary"
                  : "bg-muted border-border"
              )}
              aria-hidden="true"
            />
            <p
              className={cn(
                "text-sm font-medium",
                completed || active
                  ? "text-foreground"
                  : "text-muted-foreground"
              )}
            >
              {step.label}
            </p>
            {(completed || active) && step.timestamp && (
              <p className="text-xs text-muted-foreground">
                {step.adminName ? `${step.adminName} · ` : ""}
                {relativeTime(step.timestamp)}
              </p>
            )}
          </li>
        );
      })}
    </ol>
  );
}
