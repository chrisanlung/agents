import { Card, CardContent } from "@/components/ui/card";

export default function UsersLoading() {
  return (
    <div className="space-y-6">
      {/* Header skeleton */}
      <div className="flex items-center justify-between">
        <div className="h-7 w-32 animate-pulse rounded bg-muted" />
        <div className="h-9 w-40 animate-pulse rounded bg-muted" />
      </div>

      {/* Filter bar skeleton */}
      <div className="flex gap-3">
        <div className="h-9 w-44 animate-pulse rounded bg-muted" />
        <div className="h-9 w-44 animate-pulse rounded bg-muted" />
        <div className="h-9 w-40 animate-pulse rounded bg-muted" />
      </div>

      {/* Table skeleton */}
      <Card>
        <CardContent className="p-0">
          <div className="space-y-1 p-4">
            {Array.from({ length: 8 }).map((_, i) => (
              <div
                key={i}
                className="h-12 w-full animate-pulse rounded bg-muted"
                style={{ opacity: 1 - i * 0.08 }}
              />
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
