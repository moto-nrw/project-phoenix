"use client";

import { useCallback } from "react";
import { useSSE } from "~/lib/hooks/use-sse";
import { dispatchPhoenixNotification } from "~/lib/notification-events";
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
 * rather than each opening its own EventSource to the same endpoint.
 *
 * Guardian-scoped notification events use the same phoenix:notification
 * dispatcher as the staff portal, so the root NotificationBridge renders them.
 *
 * A parent_message_read trigger (the OGS read this guardian's messages, or the
 * guardian read in another tab) only refreshes the unread badge and the open
 * conversation's "Gelesen" receipts — it does NOT refresh the thread LIST, whose
 * order and previews a read never changes. The backend emits it only on a real
 * cursor advance, so the conversation refetch it triggers cannot loop. Renders
 * nothing.
 */
export function ParentRealtimeBridge() {
  const handleSSE = useCallback((event: SSEEvent) => {
    if (typeof window === "undefined") return;
    if (event.type === "notification") {
      dispatchPhoenixNotification(event);
      return;
    }
    if (event.type === "parent_message") {
      window.dispatchEvent(new CustomEvent("parent-messages-unread-refresh"));
      window.dispatchEvent(new CustomEvent("parent-threads-refresh"));
      window.dispatchEvent(
        new CustomEvent("parent-conversation-refresh", {
          detail: { studentId: event.data?.student_id ?? null },
        }),
      );
      return;
    }
    if (event.type === "parent_message_read") {
      window.dispatchEvent(new CustomEvent("parent-messages-unread-refresh"));
      window.dispatchEvent(
        new CustomEvent("parent-conversation-refresh", {
          detail: { studentId: event.data?.student_id ?? null },
        }),
      );
      return;
    }
    if (event.type === "parent_child_updated") {
      // Message-independent care-state invalidation: the child's care view
      // refetches, but this is NOT a message — do NOT touch the unread badge or the
      // thread list. Reuse the same `parent-conversation-refresh` window event the
      // care view already listens on, carrying the affected studentId so only that
      // child's view refetches (others skip on the id mismatch).
      window.dispatchEvent(
        new CustomEvent("parent-conversation-refresh", {
          detail: { studentId: event.data?.student_id ?? null },
        }),
      );
    }
  }, []);
  useSSE("/api/parent/sse/events", { onMessage: handleSSE });
  return null;
}
