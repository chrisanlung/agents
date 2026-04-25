import type { Metadata } from "next";
import { Suspense } from "react";
import { Inter } from "next/font/google";
import { Toaster } from "@/components/ui/sonner";
import { FlashToast } from "@/components/flash-toast";
import { ActivityTracker } from "@/components/activity-tracker";
import "@/app/globals.css";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
});

const appName = process.env.NEXT_PUBLIC_APP_NAME ?? "Lustia Platform Console";

export const metadata: Metadata = {
  title: {
    default: appName,
    template: `%s | ${appName}`,
  },
  description: "Konsol administrasi platform untuk Lustia",
  robots: {
    // Admin portal — do not index
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
