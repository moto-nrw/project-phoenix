"use client";

import { useCallback, useEffect, useRef, useState } from "react";
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
 * A write made by the caller itself announces on the same bus, so it is
 * suppressed via {@link CompanionRemoteRefresh.withOwnWrite}: the caller
 * reloads after its own save anyway, and marking itself stale would leave the
 * user staring at a conflict warning for their own change.
 */
/**
 * How long after a own save its announcement is still treated as the echo of
 * that save rather than as a remote edit.
 *
 * Generous on purpose: swallowing one genuinely remote announcement in this
 * window costs nothing that matters — the caller reloads the stored links after
 * its own save anyway, and a save built on a snapshot that a remote write has
 * replaced is still refused by the backend's fingerprint check (409
 * `companions_changed` → markStale). Being too STRICT is what costs the user
 * their work: the form would block the save they just completed.
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
  /** Runs a save, ignoring the announcement it makes itself. */
  readonly withOwnWrite: <T>(write: () => Promise<T>) => Promise<T>;
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
  useEffect(() => {
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

  const withOwnWrite = useCallback(async <T>(write: () => Promise<T>) => {
    ownWriteRef.current = true;
    try {
      return await write();
    } finally {
      // The write's OWN announcement can still be in flight when its response
      // is already here: the backend streams the response from the handler and
      // only then commits and runs the after-commit hook that broadcasts
      // student_companions_changed, so the SSE echo lands AFTER this promise
      // settles. Clearing the suppression here alone would let the form treat
      // its own change as a remote one and refuse the save the user just made.
      // The draft only stops differing from the baseline once the caller's
      // reload lands, so the window has to outlive that reload, not the fetch.
      ownWriteEchoUntilRef.current = Date.now() + OWN_WRITE_ECHO_GRACE_MS;
      ownWriteRef.current = false;
    }
  }, []);

  const markStale = useCallback(() => setCompanionsStale(true), []);

  return { companionsStale, refreshFromRemote, withOwnWrite, markStale };
}
