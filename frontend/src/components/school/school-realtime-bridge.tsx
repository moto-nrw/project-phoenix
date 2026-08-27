"use client";

import { useCallback } from "react";
import { useSSE } from "~/lib/hooks/use-sse";
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
 */
export function SchoolRealtimeBridge() {
  const handleSSE = useCallback((event: SSEEvent) => {
    if (typeof window === "undefined") return;
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
  useSSE("/api/school/sse/events", { onMessage: handleSSE });
  return null;
}
