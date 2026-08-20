"use client";

import { useEffect } from "react";
import { useToast } from "~/contexts/ToastContext";
import { installRateLimitFetchGuard } from "~/lib/rate-limit-backoff";

export function RateLimitBridge() {
  const toast = useToast();

  useEffect(() => installRateLimitFetchGuard(), []);
  useEffect(() => {
    const showNotice = (event: Event) => {
      const seconds =
        event instanceof CustomEvent &&
        typeof event.detail?.seconds === "number"
          ? Math.max(1, Math.ceil(event.detail.seconds))
          : 60;
      toast.warning(
        `Zu viele Anfragen auf einmal. Bitte warten Sie ${seconds} Sekunden und versuchen Sie es dann erneut.`,
        { id: "rate-limit", duration: Math.min(10_000, seconds * 1000) },
      );
    };
    window.addEventListener("phoenix:rate-limited", showNotice);
    return () => window.removeEventListener("phoenix:rate-limited", showNotice);
  }, [toast]);

  return null;
}
