"use client";

import { useEffect, useRef } from "react";

/**
 * Subscribe to a messaging window event (the app-wide `messages-activity`
 * dispatched by the staff global SSE connection in use-global-sse, or — via the
 * `eventName` override — the parents portal's `parent-conversation-refresh` from
 * ParentRealtimeBridge), so messaging surfaces refetch WITHOUT opening their own
 * EventSource. Replaces the hand-rolled addEventListener + CustomEvent cast +
 * id-mismatch skip + cleanup previously copied into the inbox, the thread page,
 * the student-detail card, and the parent conversation view.
 *
 * Matching: the callback fires unless the event explicitly names a DIFFERENT id
 * than the one this consumer cares about. A missing/null id ALWAYS matches —
 * the staff fan-out is sanitized (student_id stripped, see
 * realtime/events.go staffSafeParentMessage), so a null studentId must trigger a
 * refetch, not be skipped. thread_id is retained in that fan-out, so a thread
 * page still skips unrelated threads.
 *
 * debounceMs collapses bursts (the morning rush wakes every tenant staffer per
 * message) into one refetch — used by the heavy inbox query.
 *
 * Backgrounded-tab guard (marksRead consumers only): opening/refetching a thread
 * advances the reader's read cursor server-side (GET marks the thread read), so
 * firing onMatch while the tab is hidden would silently mark an unseen message
 * read — the staffer never sees it and the unread badge never lights up. For those
 * consumers (the staff thread page, the parent conversation view) a matching event
 * arriving while `document.hidden` is remembered and flushed once the tab is
 * visible again, so a real read only happens when the user is actually looking.
 *
 * Refetch-only consumers (the inbox, the parent multi-child list, the child-detail
 * card) never advance a read cursor, so deferring them just leaves the rows/unread
 * pills stale in a background tab for no benefit. They pass `marksRead: false` to
 * fire immediately. The flag defaults to `true` so a new read-advancing consumer
 * keeps the protective guard unless it explicitly opts out.
 */
export interface MessagesActivityDetail {
  readonly threadId?: string | null;
  readonly studentId?: string | null;
}

export function useMessagesActivity({
  onMatch,
  threadId,
  studentId,
  debounceMs,
  eventName = "messages-activity",
  marksRead = true,
}: {
  onMatch: () => void;
  threadId?: string;
  studentId?: string;
  debounceMs?: number;
  eventName?: string;
  /**
   * Whether onMatch advances a server-side read cursor (true for thread/chat
   * views). When true the hidden-tab guard defers the refetch until the tab is
   * visible; when false the refetch fires immediately even in a background tab.
   */
  marksRead?: boolean;
}): void {
  const onMatchRef = useRef(onMatch);
  onMatchRef.current = onMatch;

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    // A matching event arrived while the tab was hidden — defer the refetch until
    // the tab is visible again so a backgrounded thread is never marked read unseen.
    let pendingWhileHidden = false;

    // Run onMatch, but re-check visibility FIRST: the tab can be backgrounded
    // during the debounce window (event arrives while visible → timer scheduled →
    // user switches tabs before it fires). Firing now would mark the thread read
    // unseen, so defer to the visibilitychange flush instead — same guard the
    // arrival handler applies, repeated here because the timer outlives it.
    const runMatch = () => {
      if (marksRead && typeof document !== "undefined" && document.hidden) {
        pendingWhileHidden = true;
        return;
      }
      onMatchRef.current();
    };

    const fire = () => {
      if (debounceMs && debounceMs > 0) {
        if (timer) clearTimeout(timer);
        timer = setTimeout(runMatch, debounceMs);
      } else {
        runMatch();
      }
    };

    const handler = (event: Event) => {
      const detail = (event as CustomEvent<MessagesActivityDetail>).detail;
      if (threadId && detail?.threadId && detail.threadId !== threadId) return;
      if (studentId && detail?.studentId && detail.studentId !== studentId) {
        return;
      }
      if (marksRead && typeof document !== "undefined" && document.hidden) {
        pendingWhileHidden = true;
        return;
      }
      fire();
    };

    const onVisibility = () => {
      if (
        typeof document !== "undefined" &&
        !document.hidden &&
        pendingWhileHidden
      ) {
        pendingWhileHidden = false;
        fire();
      }
    };

    window.addEventListener(eventName, handler);
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      window.removeEventListener(eventName, handler);
      document.removeEventListener("visibilitychange", onVisibility);
      if (timer) clearTimeout(timer);
    };
  }, [threadId, studentId, debounceMs, eventName, marksRead]);
}
