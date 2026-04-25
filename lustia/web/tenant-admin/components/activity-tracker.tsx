"use client";

import { useEffect } from "react";

const COOKIE_NAME = "lustia_last_activity";
// Throttle cookie writes — once per 60s is enough to slide the idle window
// without writing cookies on every keystroke / mouse move.
const WRITE_INTERVAL_MS = 60_000;
// Keep the cookie alive for the lifetime of a refresh token (14 days) so that
// the middleware has a value to read on the very first request after a browser
// restart. Activity rewrites this on each interaction.
const COOKIE_MAX_AGE_SECONDS = 14 * 24 * 60 * 60;

/**
 * ActivityTracker — writes `lustia_last_activity=<epoch ms>` as a cookie when
 * the user is active, throttled to once per minute. Middleware reads this
 * cookie before performing silent refresh: if the gap between now and
 * last-activity exceeds the idle threshold, refresh is skipped so the session
 * expires naturally and the next protected request redirects to /login.
 *
 * Activity sources listened to (passive, capture phase):
 *   - mousedown / keydown  — real interaction
 *   - touchstart           — tablet / mobile
 *   - visibilitychange     — user returning to the tab
 *
 * Scroll and mousemove are intentionally NOT tracked — they fire constantly
 * even when the user is afk reading, and would defeat idle detection.
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

    // Seed immediately so mid-session refreshes don't incorrectly see "no
    // activity ever".
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
