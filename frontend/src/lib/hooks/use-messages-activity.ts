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
}: {
  onMatch: () => void;
  threadId?: string;
  studentId?: string;
  debounceMs?: number;
  eventName?: string;
}): void {
  const onMatchRef = useRef(onMatch);
  onMatchRef.current = onMatch;

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    const handler = (event: Event) => {
      const detail = (event as CustomEvent<MessagesActivityDetail>).detail;
      if (threadId && detail?.threadId && detail.threadId !== threadId) return;
      if (studentId && detail?.studentId && detail.studentId !== studentId) {
        return;
      }
      if (debounceMs && debounceMs > 0) {
        if (timer) clearTimeout(timer);
        timer = setTimeout(() => onMatchRef.current(), debounceMs);
      } else {
        onMatchRef.current();
      }
    };
    window.addEventListener(eventName, handler);
    return () => {
      window.removeEventListener(eventName, handler);
      if (timer) clearTimeout(timer);
    };
  }, [threadId, studentId, debounceMs, eventName]);
}
