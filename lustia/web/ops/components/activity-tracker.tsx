"use client";

import { useEffect } from "react";

const COOKIE_NAME = "lustia_last_activity";
const WRITE_INTERVAL_MS = 60_000;
const COOKIE_MAX_AGE_SECONDS = 14 * 24 * 60 * 60;

/**
 * Writes `lustia_last_activity=<epoch ms>` cookie on user interaction.
 * Middleware reads this to gate silent refresh — if idle > threshold,
 * refresh is skipped so the session expires naturally.
 */
export function ActivityTracker() {
  useEffect(() => {
    let lastWrite = 0;

    const writeCookie = () => {
      const now = Date.now();
      if (now - lastWrite < WRITE_INTERVAL_MS) return;
      lastWrite = now;
      document.cookie = `${COOKIE_NAME}=${now}; Path=/; Max-Age=${COOKIE_MAX_AGE_SECONDS}; SameSite=Lax`;
    };

    writeCookie();

    const onActivity = () => writeCookie();
    const onVisibility = () => {
      if (document.visibilityState === "visible") writeCookie();
    };

    window.addEventListener("mousedown", onActivity, { passive: true });
    window.addEventListener("keydown", onActivity, { passive: true });
    window.addEventListener("touchstart", onActivity, { passive: true });
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      window.removeEventListener("mousedown", onActivity);
      window.removeEventListener("keydown", onActivity);
      window.removeEventListener("touchstart", onActivity);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, []);

  return null;
}
