"use client";

import { useCallback } from "react";
import { useSSE } from "~/lib/hooks/use-sse";
import type { SSEEvent } from "~/lib/sse-types";

/**
 * Portal-wide SSE bridge for the parents app, mounted once in the authenticated
 * shell. The per-conversation chat has its own SSE handler, but a guardian
 * sitting on the dashboard or the multi-child messages list otherwise never
 * learned about new staff activity until they reopened a chat or refocused the
 * window — the unread badge and the list both only refetched on those triggers.
 *
 * On a parent_message trigger this dispatches the window events those surfaces
 * already listen on: `parent-messages-unread-refresh` (the sidebar unread badge),
 * `parent-threads-refresh` (the messages list), and `parent-conversation-refresh`
 * carrying the affected studentId (an open OgsConversation). This is the SINGLE
 * SSE connection for the parents portal — surfaces react to these window events
 * rather than each opening its own EventSource to the same endpoint. Renders
 * nothing.
 */
export function ParentRealtimeBridge() {
  const handleSSE = useCallback((event: SSEEvent) => {
    if (event.type !== "parent_message") return;
    if (typeof window === "undefined") return;
    window.dispatchEvent(new CustomEvent("parent-messages-unread-refresh"));
    window.dispatchEvent(new CustomEvent("parent-threads-refresh"));
    window.dispatchEvent(
      new CustomEvent("parent-conversation-refresh", {
        detail: { studentId: event.data?.student_id ?? null },
      }),
    );
  }, []);
  useSSE("/api/parent/sse/events", { onMessage: handleSSE });
  return null;
}
