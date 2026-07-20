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
      ownWriteRef.current = false;
    }
  }, []);

  const markStale = useCallback(() => setCompanionsStale(true), []);

  return { companionsStale, refreshFromRemote, withOwnWrite, markStale };
}
