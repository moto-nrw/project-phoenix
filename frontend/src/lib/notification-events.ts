import type { SSEEvent } from "~/lib/sse-types";

export interface PhoenixNotificationDetail {
  title: string | null;
  body: string | null;
  deepLink: string | null;
  priority: string | null;
  notificationType: string | null;
  data: Record<string, string> | null;
}

export function dispatchPhoenixNotification(event: SSEEvent) {
  if (typeof window === "undefined") return;

  window.dispatchEvent(
    new CustomEvent<PhoenixNotificationDetail>("phoenix:notification", {
      detail: {
        title: event.data.title ?? null,
        body: event.data.body ?? null,
        deepLink: event.data.deep_link ?? null,
        priority: event.data.priority ?? null,
        notificationType: event.data.notification_type ?? null,
        data: event.data.notification_data ?? null,
      },
    }),
  );
}
