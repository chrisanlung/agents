export default function DashboardLoading() {
  return (
    <div className="min-h-screen bg-slate-50">
      <header className="border-b bg-white px-6 py-4 shadow-sm">
        <div className="mx-auto flex max-w-5xl items-center justify-between">
          <div className="h-6 w-48 animate-pulse rounded bg-slate-200" />
          <div className="h-8 w-20 animate-pulse rounded bg-slate-200" />
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-8">
        <div className="rounded-lg border bg-white p-6">
          <div className="space-y-4">
            <div className="h-5 w-32 animate-pulse rounded bg-slate-200" />
            <div className="grid grid-cols-2 gap-4">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="space-y-1">
                  <div className="h-3 w-20 animate-pulse rounded bg-slate-200" />
                  <div className="h-4 w-36 animate-pulse rounded bg-slate-200" />
                </div>
              ))}
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
