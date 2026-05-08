export default function RootLoading() {
  return (
    <div className="min-h-screen bg-amber-50">
      {/* Header placeholder */}
      <div className="h-14 border-b border-amber-100/70 bg-gradient-to-r from-amber-100 via-amber-50 to-white animate-pulse" />
      {/* Body skeleton */}
      <main className="mx-auto max-w-5xl px-6 py-8 space-y-4">
        <div className="h-8 w-48 bg-slate-200 rounded animate-pulse" />
        <div className="h-4 w-72 bg-slate-200 rounded animate-pulse" />
        <div className="grid gap-3">
          <div className="h-24 bg-white/70 rounded-lg border border-amber-100 animate-pulse" />
          <div className="h-24 bg-white/70 rounded-lg border border-amber-100 animate-pulse" />
          <div className="h-24 bg-white/70 rounded-lg border border-amber-100 animate-pulse" />
        </div>
      </main>
    </div>
  );
}
