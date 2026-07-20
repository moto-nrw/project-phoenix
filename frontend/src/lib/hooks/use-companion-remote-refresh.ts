"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { subscribeStudentCompanionsChanged } from "~/lib/student-companion-api";

/**
 * Keeps an EDITABLE Laufgemeinschaft view honest about remote writes (#1694).
 *
 * The read-only card can simply refetch on every announcement, but a form holds
 * a draft plus the loaded snapshot its dirty check compares against. Both go
 * stale when another tab, another browser or another staff member changes the
 * links of the same child — and because the submitted list REPLACES the stored
 * one, saving a draft that was built on the old snapshot would silently delete
 * whatever they added.
 *
 * Two cases, deliberately handled differently:
 *
 * - Nothing edited yet: refetch. The user loses nothing and the form continues
 *   from the current state.
 * - Draft in progress: do NOT throw the user's work away behind their back.
 *   The view is flagged stale so the caller can block the save and offer an
 *   explicit reload — a wrong list saved silently is far worse than a form the
 *   user has to fill in again.
 *
 * A write made by the caller itself announces on the same bus. That echo is not
 * a conflict — marking the view stale for the user's own change would leave
 * them staring at a warning about themselves — so
 * {@link CompanionRemoteRefresh.withOwnWrite} downgrades it to a plain refetch:
 * it is the first signal that is certainly post-commit, which the caller's own
 * reload right after the save is not.
 */
/**
 * How long after a own save its announcement is still treated as the echo of
 * that save rather than as a remote edit.
 *
 * Generous on purpose: an announcement misread as our own echo in this window
 * still refetches the stored links, it merely skips the stale warning — and a
 * save built on a snapshot that a remote write has replaced is refused by the
 * backend's fingerprint check anyway (409 `companions_changed` → markStale).
 * Being too STRICT is what costs the user their work: the form would block the
 * save they just completed.
 *
 * The generosity only holds because the window is armed solely after a save
 * that can actually be echoed (see withOwnWrite's `mayAnnounce`): armed after
 * every save, a pure name edit would spend it on suppressing a genuinely
 * remote change instead.
 */
const OWN_WRITE_ECHO_GRACE_MS = 10_000;

interface CompanionRemoteRefreshOptions {
  /** False while the view is not showing the links (a closed modal). */
  readonly active: boolean;
  /** True while the draft differs from the loaded snapshot. */
  readonly hasUnsavedCompanionEdits: boolean;
  /** Refetches the stored links and resets the draft to them. */
  readonly onRefresh: () => void;
}

interface CompanionRemoteRefresh {
  /** Someone else changed the links while an edit was in progress. */
  readonly companionsStale: boolean;
  /** Discards the draft and reloads the stored links. */
  readonly refreshFromRemote: () => void;
  /**
   * Runs a save, treating the announcement it makes itself as a post-commit
   * refetch rather than as a remote conflict.
   *
   * `mayAnnounce` says whether this save can be answered by a
   * `student_companions_changed` echo at all. The backend announces only when
   * the write actually changed links — an edited list, a confirmed plan
   * extension, or a departure-plan change that trims links — while the forms
   * resubmit their whole payload on every save. A save that cannot announce
   * (a pure name or address edit, or a failed one) must not arm the echo
   * grace: the next genuinely remote announcement would be consumed as the
   * echo of a save that never made one, refreshing the view instead of warning
   * about a draft built on links that no longer exist.
   */
  readonly withOwnWrite: <T>(
    write: () => Promise<T>,
    mayAnnounce: boolean,
  ) => Promise<T>;
  /**
   * Flags the view stale from the outside — for the backend's own verdict.
   *
   * The announcement bus only covers writers in THIS tab; a save can still be
   * refused with 409 `companions_changed` because another browser replaced the
   * links. That refusal means the same thing as a remote announcement, so it
   * has to leave the form in the same state: draft kept, save blocked, "Neu
   * laden" offered.
   */
  readonly markStale: () => void;
}

export function useCompanionRemoteRefresh({
  active,
  hasUnsavedCompanionEdits,
  onRefresh,
}: CompanionRemoteRefreshOptions): CompanionRemoteRefresh {
  const [companionsStale, setCompanionsStale] = useState(false);

  // The listener is subscribed once and must not be re-registered on every
  // keystroke, so the values it reads travel through refs.
  const activeRef = useRef(active);
  const dirtyRef = useRef(hasUnsavedCompanionEdits);
  const refreshRef = useRef(onRefresh);
  const ownWriteRef = useRef(false);
  // Deadline until which an announcement is still attributed to the write that
  // just finished — see OWN_WRITE_ECHO_GRACE_MS.
  const ownWriteEchoUntilRef = useRef(0);
  // Layout effect, not a passive one: the SSE listener below runs in its own
  // task, and passive effects flush after paint — so between committing a
  // render that turned the draft dirty and the passive flush, a notification
  // could still read the PREVIOUS dirty value and refresh the draft away
  // instead of flagging it stale. A layout effect runs synchronously with the
  // commit, before any other task gets to observe these refs.
  useLayoutEffect(() => {
    activeRef.current = active;
    dirtyRef.current = hasUnsavedCompanionEdits;
    refreshRef.current = onRefresh;
  });

  // A closed modal keeps its stale flag out of the way: it reloads on open.
  useEffect(() => {
    if (!active) setCompanionsStale(false);
  }, [active]);

  useEffect(
    () =>
      subscribeStudentCompanionsChanged(() => {
        if (!activeRef.current || ownWriteRef.current) return;
        if (Date.now() < ownWriteEchoUntilRef.current) {
          // The SSE echo of our own save. Consume the grace with it so a
          // genuinely remote write arriving right after is judged normally.
          ownWriteEchoUntilRef.current = 0;
          // Refetch instead of returning: the caller starts its reload the
          // moment withOwnWrite resolves, but the response is streamed from the
          // handler BEFORE the outer tenant transaction commits (see
          // withOwnWrite below), so that reload can still read the pre-save
          // edges and record them as the new baseline. This echo is the first
          // signal that is guaranteed to be post-commit — the broadcast runs in
          // the after-commit hook — so it is exactly the point at which the
          // stored links are worth re-reading.
          //
          // Unconditional, dirty draft or not: the flag is stale here anyway
          // (it compares against the pre-save baseline the reload is about to
          // replace), and what the user "loses" is at most whatever they typed
          // in the few hundred milliseconds between their own save landing and
          // its echo. Marking stale instead would put a conflict warning on the
          // screen for the user's own change, which is the one thing this hook
          // exists to avoid.
          setCompanionsStale(false);
          refreshRef.current();
          return;
        }
        if (dirtyRef.current) setCompanionsStale(true);
        else refreshRef.current();
      }),
    [],
  );

  const refreshFromRemote = useCallback(() => {
    setCompanionsStale(false);
    refreshRef.current();
  }, []);

  const withOwnWrite = useCallback(
    async <T>(write: () => Promise<T>, mayAnnounce: boolean) => {
      ownWriteRef.current = true;
      try {
        const result = await write();
        // The write's OWN announcement can still be in flight when its
        // response is already here: the backend streams the response from the
        // handler and only then commits and runs the after-commit hook that
        // broadcasts student_companions_changed, so the SSE echo lands AFTER
        // this promise settles. Clearing the suppression alone would let the
        // form treat its own change as a remote one and refuse the save the
        // user just made. The draft only stops differing from the baseline
        // once the caller's reload lands, so the window has to outlive that
        // reload, not the fetch.
        //
        // Armed only on SUCCESS and only when the save can announce at all: a
        // failed save and a save the backend answers without a broadcast
        // produce no echo, so a grace period after them could only ever
        // swallow somebody else's genuine change.
        if (mayAnnounce) {
          ownWriteEchoUntilRef.current = Date.now() + OWN_WRITE_ECHO_GRACE_MS;
        }
        return result;
      } finally {
        ownWriteRef.current = false;
      }
    },
    [],
  );

  const markStale = useCallback(() => setCompanionsStale(true), []);

  return { companionsStale, refreshFromRemote, withOwnWrite, markStale };
}
