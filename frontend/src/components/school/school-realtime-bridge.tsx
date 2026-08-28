"use client";

import { useCallback } from "react";
import { useSession } from "next-auth/react";
import { useSSE } from "~/lib/hooks/use-sse";
import { dispatchPhoenixNotification } from "~/lib/notification-events";
import { schoolTeamChatDeepLink } from "~/lib/school-team-chat-links";
import { schoolPath } from "~/lib/school-url";
import type { SSEEvent } from "~/lib/sse-types";

/**
 * Die eine SSE-Verbindung des Schul-Portals (#2208), einmal in der
 * angemeldeten Hülle eingehängt. Das Backend schickt hier nur, was an das
 * Konto der Lehrkraft adressiert ist; heute ist das der Team-Chat.
 *
 * Auf `staff_message` löst sie dieselben Fensterereignisse aus wie die
 * OGS-Verbindung (use-global-sse): den Ungelesen-Zähler der Navigation und die
 * Aktivität, auf die Posteingang und offene Unterhaltung lauschen. Die
 * Chat-Oberflächen sind in beiden Portalen dieselben Komponenten und
 * unterscheiden nicht, welche Verbindung sie geweckt hat.
 *
 * Ein `notification`-Ereignis (der persönliche Hinweis "Neue Nachricht aus
 * dem Team") geht an den app-weiten NotificationBridge-Toast; sein Deep-Link
 * ist für das OGS-Portal geschrieben und wird vorher auf den Posteingang des
 * Schul-Hosts umgebogen.
 *
 * Die Verbindung hängt an der angemeldeten Sitzung (Schule und Konto): der
 * Server ordnet den Stream dem Konto des mitgeschickten Zugangs zu, und die
 * Hülle bleibt beim Schulwechsel eingehängt. Ohne diesen Schlüssel liefe der
 * alte EventSource mit dem alten Zugang weiter, bis er von selbst abbricht.
 * Die neue Sitzung bekäme dann Hinweise und Zähler-Auffrischungen der vorigen
 * Schule.
 */
export function SchoolRealtimeBridge() {
  const { data: session, status } = useSession();
  const handleSSE = useCallback((event: SSEEvent) => {
    if (typeof window === "undefined") return;
    if (event.type === "notification") {
      // The backend names the school destination itself (school_deep_link);
      // the mapper only covers producers that do not yet.
      const schoolLink =
        event.data?.school_deep_link ??
        (typeof event.data?.deep_link === "string"
          ? schoolTeamChatDeepLink(event.data.deep_link)
          : undefined);
      dispatchPhoenixNotification(
        typeof schoolLink === "string"
          ? {
              ...event,
              data: { ...event.data, deep_link: schoolPath(schoolLink) },
            }
          : event,
      );
      return;
    }
    if (event.type !== "staff_message") return;
    window.dispatchEvent(new CustomEvent("team-messages-unread-refresh"));
    window.dispatchEvent(
      new CustomEvent("team-messages-activity", {
        detail: {
          threadId: event.data?.thread_id ?? null,
          source: event.data?.source ?? null,
        },
      }),
    );
  }, []);
  useSSE("/api/school/sse/events", {
    onMessage: handleSSE,
    enabled: status === "authenticated",
    reconnectKey: `${session?.user?.tenantId ?? ""}:${session?.user?.id ?? ""}`,
  });
  return null;
}
