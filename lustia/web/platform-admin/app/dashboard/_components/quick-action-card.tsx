import Link from "next/link";
import { ChevronRight, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface QuickActionCardProps {
  title: string;
  description: string;
  icon: LucideIcon;
  href: string;
}

/**
 * Aksi Cepat shortcut card — a full-card Link with icon circle, title,
 * description and a ChevronRight affordance.  Three of these are rendered
 * side-by-side (or stacked on mobile) in the Aksi Cepat section.
 */
export function QuickActionCard({
  title,
  description,
  icon: Icon,
  href,
}: QuickActionCardProps) {
  return (
    <Link
      href={href}
      className={cn(
        "group flex items-center gap-4 rounded-lg border bg-card p-5",
        "transition-shadow duration-150 hover:shadow-md",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      )}
    >
      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary/10">
        <Icon size={20} className="text-primary" aria-hidden="true" />
      </div>
      <div className="min-w-0 flex-1">
        <p className="text-sm font-semibold text-foreground">{title}</p>
        <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
          {description}
        </p>
      </div>
      <ChevronRight
        size={16}
        className={cn(
          "shrink-0 text-muted-foreground/60",
          "transition-colors duration-150 group-hover:text-foreground"
        )}
        aria-hidden="true"
      />
    </Link>
  );
}
