"use client";

/**
 * CheckinScanner — QR camera scanner + manual code entry.
 *
 * Uses @zxing/library (BrowserMultiFormatReader) for camera-based QR scanning.
 *
 * NOTE: Camera API (MediaDevices) requires a secure context (HTTPS or localhost).
 * In non-secure contexts, the camera card renders a warning and falls back
 * to manual entry only.
 */

import { useEffect, useRef, useState, useCallback } from "react";
import { CameraOff, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

interface CheckinScannerProps {
  onScan: (code: string) => void;
  isLoading?: boolean;
}

type CameraState = "initializing" | "active" | "denied" | "unsupported" | "error";

/** Auto-format: insert hyphen after 4 chars, uppercase everything. */
function formatCode(raw: string): string {
  const cleaned = raw.toUpperCase().replace(/[^A-Z0-9]/g, "").slice(0, 8);
  if (cleaned.length > 4) {
    return `${cleaned.slice(0, 4)}-${cleaned.slice(4)}`;
  }
  return cleaned;
}

export function CheckinScanner({ onScan, isLoading = false }: CheckinScannerProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const readerRef = useRef<import("@zxing/library").BrowserMultiFormatReader | null>(null);
  const [cameraState, setCameraState] = useState<CameraState>("initializing");
  const [manualCode, setManualCode] = useState("");
  const [isSecureContext, setIsSecureContext] = useState(true);

  useEffect(() => {
    // Camera requires secure context
    if (typeof window !== "undefined" && !window.isSecureContext) {
      setIsSecureContext(false);
      setCameraState("unsupported");
      return;
    }

    let stopped = false;

    async function startCamera() {
      try {
        const { BrowserMultiFormatReader } = await import("@zxing/library");
        const reader = new BrowserMultiFormatReader();
        readerRef.current = reader;

        if (!videoRef.current) return;

        await reader.decodeFromVideoDevice(
          null,
          videoRef.current,
          (result, err) => {
            if (stopped) return;
            if (result) {
              const text = result.getText();
              // Only act on codes that match 8-char booking code pattern
              if (/^[A-Z2-7]{4}-[A-Z2-7]{4}$/.test(text) || /^[A-Z0-9]{8}$/.test(text)) {
                const formatted = text.includes("-")
                  ? text
                  : `${text.slice(0, 4)}-${text.slice(4)}`;
                onScan(formatted);
              }
            }
            if (err && err.name !== "NotFoundException") {
              // NotFoundException is normal (no QR in frame), ignore
            }
          }
        );

        if (!stopped) {
          setCameraState("active");
        }
      } catch (err: unknown) {
        if (stopped) return;
        if (
          err instanceof Error &&
          (err.name === "NotAllowedError" || err.name === "PermissionDeniedError")
        ) {
          setCameraState("denied");
        } else {
          setCameraState("error");
        }
      }
    }

    startCamera();

    return () => {
      stopped = true;
      if (readerRef.current) {
        readerRef.current.reset();
        readerRef.current = null;
      }
    };
  }, [onScan]);

  const retryCamera = useCallback(() => {
    if (readerRef.current) {
      readerRef.current.reset();
      readerRef.current = null;
    }
    setCameraState("initializing");
    // Re-mounting handled by key change — parent can handle if needed.
    // Here we just attempt permission again via MediaDevices.
    navigator.mediaDevices
      ?.getUserMedia({ video: true })
      .then(() => window.location.reload())
      .catch(() => setCameraState("denied"));
  }, []);

  function handleManualSubmit(e: React.FormEvent) {
    e.preventDefault();
    const code = manualCode.replace(/-/g, "").toUpperCase();
    if (code.length !== 8) return;
    const formatted = `${code.slice(0, 4)}-${code.slice(4)}`;
    onScan(formatted);
  }

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      {/* LEFT — Camera */}
      <Card className="overflow-hidden">
        {cameraState === "unsupported" || !isSecureContext ? (
          <CardContent className="flex h-[420px] flex-col items-center justify-center gap-4 text-center">
            <CameraOff
              size={48}
              className="text-muted-foreground"
              aria-hidden="true"
            />
            <div className="space-y-1">
              <p className="font-medium text-foreground">
                Kamera tidak tersedia
              </p>
              <p className="max-w-xs text-sm text-muted-foreground">
                Pemindai QR memerlukan koneksi HTTPS. Gunakan input kode manual
                di sebelah kanan.
              </p>
            </div>
          </CardContent>
        ) : cameraState === "denied" || cameraState === "error" ? (
          <CardContent className="flex h-[420px] flex-col items-center justify-center gap-4 text-center">
            <CameraOff
              size={48}
              className="text-muted-foreground"
              aria-hidden="true"
            />
            <div className="space-y-1">
              <p className="font-medium text-foreground">Akses kamera ditolak.</p>
              <p className="max-w-xs text-sm text-muted-foreground">
                Izinkan akses kamera di pengaturan browser untuk menggunakan
                pemindai QR.
              </p>
            </div>
            <Button variant="outline" onClick={retryCamera}>
              Coba Izinkan Kamera
            </Button>
          </CardContent>
        ) : (
          <div className="relative h-[420px] w-full bg-black">
            {cameraState === "initializing" && (
              <div className="absolute inset-0 flex items-center justify-center">
                <Loader2
                  size={32}
                  className="animate-spin text-white"
                  aria-hidden="true"
                />
              </div>
            )}
            {/* Camera feed */}
            <video
              ref={videoRef}
              className="h-full w-full object-cover"
              autoPlay
              muted
              playsInline
              aria-label="Kamera untuk pemindai QR"
            />
            {/* Scanning overlay */}
            <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
              <div
                className={cn(
                  "h-48 w-48 rounded-md border-2 border-white/80",
                  cameraState === "active" && "animate-pulse"
                )}
                aria-hidden="true"
              />
            </div>
            <p className="absolute bottom-4 left-0 right-0 text-center text-sm text-white drop-shadow">
              Arahkan kamera ke QR code booking
            </p>
          </div>
        )}
      </Card>

      {/* RIGHT — Manual entry */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Atau masukkan kode manual</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleManualSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="manual-code">
                Kode Booking{" "}
                <span className="text-muted-foreground">(8 karakter)</span>
              </Label>
              <Input
                id="manual-code"
                value={manualCode}
                onChange={(e) => setManualCode(formatCode(e.target.value))}
                placeholder="B7K3-M2QF"
                maxLength={9}
                className="text-center font-mono text-lg uppercase tracking-widest"
                autoComplete="off"
                autoCorrect="off"
                spellCheck={false}
                aria-describedby="code-hint"
              />
              <p
                id="code-hint"
                className="text-xs text-muted-foreground"
              >
                Format: XXXX-XXXX (contoh: B7K3-M2QF)
              </p>
            </div>
            <Button
              type="submit"
              className="w-full"
              disabled={
                isLoading ||
                manualCode.replace(/-/g, "").length !== 8
              }
            >
              {isLoading ? (
                <Loader2
                  size={16}
                  className="animate-spin"
                  aria-hidden="true"
                />
              ) : (
                "Cari Booking"
              )}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
