import { Loader2 } from "lucide-react";

export default function Loading() {
  return (
    <div className="flex min-h-[400px] items-center justify-center">
      <Loader2 size={28} className="animate-spin text-primary" aria-label="Memuat..." />
    </div>
  );
}
