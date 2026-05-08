export default function RootLoading() {
  return (
    <div className="min-h-screen bg-background">
      {/* Header placeholder */}
      <div className="h-14 border-b bg-card animate-pulse" />
      {/* Body skeleton */}
      <main className="mx-auto max-w-5xl px-6 py-8 space-y-4">
        <div className="h-8 w-48 bg-muted rounded animate-pulse" />
        <div className="h-4 w-72 bg-muted rounded animate-pulse" />
        <div className="grid gap-3">
          <div className="h-24 bg-muted/60 rounded-lg animate-pulse" />
          <div className="h-24 bg-muted/60 rounded-lg animate-pulse" />
          <div className="h-24 bg-muted/60 rounded-lg animate-pulse" />
        </div>
      </main>
    </div>
  );
}
