import Link from "next/link";
import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 px-4">
      <h2 className="text-lg font-semibold">Halaman tidak ditemukan</h2>
      <p className="max-w-sm text-center text-sm text-muted-foreground">
        Halaman yang Anda cari tidak tersedia.
      </p>
      <Button asChild variant="outline">
        <Link href="/dashboard">Ke dasbor</Link>
      </Button>
    </div>
  );
}
