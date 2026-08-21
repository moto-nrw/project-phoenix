"use client";

import { useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { useToast } from "~/contexts/ToastContext";
import type { PhoenixNotificationDetail } from "~/lib/notification-events";
import { installRateLimitFetchGuard } from "~/lib/rate-limit-backoff";
import { tenantAwarePath } from "~/lib/tenant-path";

export type { PhoenixNotificationDetail } from "~/lib/notification-events";

/**
 * NotificationBridge — the in-app rendering half of the notification
 * abstraction (#1624).
 *
 * useGlobalSSE receives backend "notification" events and re-dispatches them
 * as "phoenix:notification" window events (it runs outside this component
 * tree). This bridge listens for them and renders a toast. It carries no
 * state and renders nothing itself; mount it once inside ToastProvider.
 *
 * The payload is display-safe by backend contract (no sensitive student
 * data); sensitive details are only shown after navigating into the
 * authenticated app via the deep link.
 */
function isSafeAppRelativePath(path: string): boolean {
  if (!path.startsWith("/") || path.startsWith("//")) return false;

  try {
    return (
      new URL(path, window.location.origin).origin === window.location.origin
    );
  } catch {
    return false;
  }
}

export function tenantNotificationPath(
  path: string,
  tenantSlug: string | undefined,
  hostname: string,
): string {
  const routingMode =
    tenantSlug && hostname.startsWith(`${tenantSlug}.`) ? "subdomain" : "path";
  return tenantAwarePath(path, tenantSlug, routingMode);
}

export function NotificationBridge() {
  const toast = useToast();
  const router = useRouter();
  const { tenant: tenantSlug } = useParams<{ tenant?: string }>();

  useEffect(() => installRateLimitFetchGuard(), []);

  useEffect(() => {
    const onNotification = (event: Event) => {
      const detail = (event as CustomEvent<PhoenixNotificationDetail>).detail;
      if (!detail?.title) return;

      const message = detail.body
        ? `${detail.title}: ${detail.body}`
        : detail.title;
      const deepLink = detail.deepLink;
      const action =
        deepLink && isSafeAppRelativePath(deepLink)
          ? {
              label: "Öffnen",
              onClick: () =>
                router.push(
                  tenantNotificationPath(
                    deepLink,
                    tenantSlug,
                    window.location.hostname,
                  ),
                ),
            }
          : undefined;

      // High priority renders as warning (amber) so it stands out; everything
      // else as info. Errors stay reserved for actual failures.
      if (detail.priority === "high") {
        toast.warning(message, { duration: 8000, action });
      } else {
        toast.info(message, { duration: 6000, action });
      }
    };

    window.addEventListener("phoenix:notification", onNotification);
    return () =>
      window.removeEventListener("phoenix:notification", onNotification);
  }, [router, tenantSlug, toast]);

  useEffect(() => {
    const onRateLimited = (event: Event) => {
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

    window.addEventListener("phoenix:rate-limited", onRateLimited);
    return () =>
      window.removeEventListener("phoenix:rate-limited", onRateLimited);
  }, [toast]);

  return null;
}
