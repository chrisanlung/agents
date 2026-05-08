import type { Metadata } from "next";
import { Suspense } from "react";
import { Inter } from "next/font/google";
import NextTopLoader from "nextjs-toploader";
import { Toaster } from "@/components/ui/sonner";
import { FlashToast } from "@/components/flash-toast";
import { ActivityTracker } from "@/components/activity-tracker";
import "@/app/globals.css";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
});

const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Tenant Portal";

export const metadata: Metadata = {
  title: {
    default: appName,
    template: `%s | ${appName}`,
  },
  description: "Portal administrasi tenant untuk Lustia",
  robots: {
    index: false,
    follow: false,
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="id" suppressHydrationWarning>
      <body className={`${inter.variable} font-sans antialiased`}>
        {/* Thin progress bar shown on every route transition */}
        <NextTopLoader color="#0d9488" height={3} showSpinner={false} shadow={false} />
        {children}
        <Toaster position="top-right" />
        <Suspense fallback={null}>
          <FlashToast />
        </Suspense>
        <ActivityTracker />
      </body>
    </html>
  );
}
