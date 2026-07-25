"use client";

import { useEffect } from "react";
import { useToast } from "~/contexts/ToastContext";

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
export interface PhoenixNotificationDetail {
  title: string | null;
  body: string | null;
  deepLink: string | null;
  priority: string | null;
  notificationType: string | null;
}

export function NotificationBridge() {
  const toast = useToast();

  useEffect(() => {
    const onNotification = (event: Event) => {
      const detail = (event as CustomEvent<PhoenixNotificationDetail>).detail;
      if (!detail?.title) return;

      const message = detail.body
        ? `${detail.title}: ${detail.body}`
        : detail.title;

      // High priority renders as warning (amber) so it stands out; everything
      // else as info. Errors stay reserved for actual failures.
      if (detail.priority === "high") {
        toast.warning(message, { duration: 8000 });
      } else {
        toast.info(message, { duration: 6000 });
      }
    };

    window.addEventListener("phoenix:notification", onNotification);
    return () =>
      window.removeEventListener("phoenix:notification", onNotification);
  }, [toast]);

  return null;
}
