"use client";

import { useEffect } from "react";
import { useLatest } from "~/lib/hooks/use-latest";

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
 * the staff fan-out is sanitized (both thread_id and student_id stripped, see
 * realtime/events.go staffSafeParentMessage), so a null threadId/studentId must
 * trigger a refetch, not be skipped. thread_id survives on the guardian's OWN
 * events, so a parent conversation view still skips unrelated threads; on the
 * staff side it is null, so a staff thread page refetches on any parent message
 * (the debounce collapses the burst) rather than letting an unauthorized staffer
 * correlate events to a specific conversation.
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
 *
 * Missed-event recovery (`refetchOnFocus`): the SSE bridge stops reconnecting
 * after maxReconnectAttempts and stays "failed". If that happens while the tab is
 * backgrounded, a message arriving in that window dispatches NO window event, so a
 * consumer whose ONLY refresh path is this hook (the staff thread page, the parent
 * conversation view) never learns about it — returning to the tab shows a lit
 * unread badge while the open conversation stays stale until a manual reload. The
 * unread badges already heal via useUnreadCount's `refetchOnFocus`; passing
 * `refetchOnFocus: true` here refetches the active conversation on the SAME window
 * `focus` event so the open chat recovers in lockstep with its badge.
 */
interface MessagesActivityDetail {
  readonly threadId?: string | null;
  readonly studentId?: string | null;
  /** Account that caused the event, when the emitter knows it. */
  readonly source?: string | null;
}

export function useMessagesActivity({
  onMatch,
  threadId,
  studentId,
  debounceMs,
  eventName = "messages-activity",
  marksRead = true,
  refetchOnFocus = false,
  ignoreOwnSource,
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
  /**
   * Also refetch on window `focus`, recovering messages missed entirely while the
   * tab slept and the SSE bridge was disconnected (no window event fired). Set on
   * conversation views whose only refresh path is this hook; mirrors the unread
   * badge's refetchOnFocus so the open chat heals on the same focus the badge does.
   */
  refetchOnFocus?: boolean;
  /**
   * Skip events this account itself caused. Set it ONLY on read-advancing
   * consumers (marksRead: true); a refetch-only consumer wants its own events
   * too, or its view goes stale after its own write.
   */
  ignoreOwnSource?: string | null;
}): void {
  const onMatchRef = useLatest(onMatch);

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
      // A consumer that ADVANCES A READ CURSOR must ignore what this very
      // account just did: the sender's other tab would otherwise refetch the
      // thread and mark everything the counterpart wrote in the meantime as
      // read, without anybody having looked. Refetch-only consumers (the
      // inbox) leave this unset and stay fresh — suppressing the whole event
      // for the sender would leave their own previews, ordering and unread
      // pills stale until some other refresh path runs.
      if (
        ignoreOwnSource &&
        detail?.source &&
        detail.source === ignoreOwnSource
      ) {
        return;
      }
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

    // Returning to the tab → recover any event missed entirely while it slept
    // (SSE dropped / exhausted reconnects, so no `eventName` event ever arrived to
    // set pendingWhileHidden). runMatch re-checks visibility, and `focus` only
    // fires while the tab is foregrounded, so a read-advancing onMatch here always
    // happens with the user looking. The rare overlap with onVisibility (an event
    // DID arrive while hidden) is collapsed by debounce, or harmless for the
    // latest-wins conversation refetches.
    const onFocus = () => fire();

    window.addEventListener(eventName, handler);
    document.addEventListener("visibilitychange", onVisibility);
    if (refetchOnFocus) window.addEventListener("focus", onFocus);
    return () => {
      window.removeEventListener(eventName, handler);
      document.removeEventListener("visibilitychange", onVisibility);
      if (refetchOnFocus) window.removeEventListener("focus", onFocus);
      if (timer) clearTimeout(timer);
    };
  }, [
    threadId,
    studentId,
    debounceMs,
    eventName,
    marksRead,
    refetchOnFocus,
    ignoreOwnSource,
    onMatchRef,
  ]);
}
