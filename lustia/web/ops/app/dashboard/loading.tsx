export default function DashboardLoading() {
  return (
    <div className="min-h-screen bg-amber-50">
      {/* Header skeleton */}
      <header className="border-b border-amber-100/70 bg-gradient-to-r from-amber-100 via-amber-50 to-white px-6 py-3 shadow-sm">
        <div className="mx-auto flex max-w-6xl items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="h-8 w-8 animate-pulse rounded-lg bg-slate-200" />
            <div className="h-5 w-36 animate-pulse rounded bg-slate-200" />
            <div className="ml-4 hidden items-center gap-1 sm:flex">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="h-7 w-20 animate-pulse rounded-md bg-slate-200" />
              ))}
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="h-8 w-24 animate-pulse rounded-md bg-slate-200" />
            <div className="h-9 w-9 animate-pulse rounded-full bg-slate-200" />
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl space-y-6 px-6 py-8">
        {/* Welcome */}
        <div className="space-y-1">
          <div className="h-7 w-56 animate-pulse rounded bg-slate-200" />
          <div className="h-4 w-40 animate-pulse rounded bg-slate-200" />
        </div>

        {/* KPI cards */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div
              key={i}
              className="flex items-center gap-4 rounded-lg border bg-white p-5"
            >
              <div className="h-11 w-11 animate-pulse rounded-full bg-slate-200" />
              <div className="space-y-2">
                <div className="h-3 w-28 animate-pulse rounded bg-slate-200" />
                <div className="h-8 w-10 animate-pulse rounded bg-slate-200" />
                <div className="h-3 w-20 animate-pulse rounded bg-slate-200" />
              </div>
            </div>
          ))}
        </div>

        {/* Aksi cepat */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {Array.from({ length: 2 }).map((_, i) => (
            <div
              key={i}
              className="flex flex-col items-center gap-3 rounded-lg border border-dashed bg-white py-8"
            >
              <div className="h-12 w-12 animate-pulse rounded-full bg-slate-200" />
              <div className="space-y-1.5 text-center">
                <div className="h-4 w-32 animate-pulse rounded bg-slate-200" />
                <div className="h-3 w-48 animate-pulse rounded bg-slate-200" />
              </div>
              <div className="h-8 w-28 animate-pulse rounded-md bg-slate-200" />
            </div>
          ))}
        </div>

        {/* Booking preview */}
        <div className="rounded-lg border bg-white p-6">
          <div className="mb-4 flex items-center justify-between">
            <div className="space-y-1">
              <div className="h-5 w-32 animate-pulse rounded bg-slate-200" />
              <div className="h-3 w-44 animate-pulse rounded bg-slate-200" />
            </div>
            <div className="h-7 w-20 animate-pulse rounded bg-slate-200" />
          </div>
          <div className="space-y-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="flex items-center gap-3 py-1">
                <div className="h-4 w-10 animate-pulse rounded bg-slate-200" />
                <div className="flex-1 space-y-1">
                  <div className="h-4 w-32 animate-pulse rounded bg-slate-200" />
                  <div className="h-3 w-48 animate-pulse rounded bg-slate-200" />
                </div>
                <div className="h-5 w-20 animate-pulse rounded-full bg-slate-200" />
              </div>
            ))}
          </div>
        </div>
      </main>
    </div>
  );
}
