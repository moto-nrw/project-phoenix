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
 * The generosity only holds because of two limits. The window is armed solely
 * after a save that can actually be echoed (see withOwnWrite's `mayAnnounce`) —
 * armed after every save, a pure name edit would spend it on suppressing a
 * genuinely remote change instead. And an announcement in the window is only
 * ATTRIBUTED to our save, never verified as its echo (the bus carries no
 * correlation id), so it may reset only the draft that save actually submitted
 * — not one started after it, and not one edited on while it was in flight; see
 * the listener.
 */
const OWN_WRITE_ECHO_GRACE_MS = 10_000;

interface CompanionRemoteRefreshOptions {
  /** False while the view is not showing the links (a closed modal). */
  readonly active: boolean;
  /**
   * Identity of the record whose links are shown — the student id in practice.
   *
   * A stale flag is a statement about ONE child's draft, and the editable views
   * are reused across children without unmounting (the master-detail tab stays
   * `active` and simply reloads for the next selection). Without this key the
   * warning survives the switch and blocks saving a form the user has not even
   * edited yet. Changing it resets the flag exactly like closing the view does.
   */
  readonly resetKey?: string | number;
  /** True while the draft differs from the loaded snapshot. */
  readonly hasUnsavedCompanionEdits: boolean;
  /**
   * Identity of the companion-relevant draft — the link list plus, where the
   * form submits one, the departure plan that trims it.
   *
   * The dirty flag alone cannot tell an echo whether the draft it is about to
   * reset is still the one the save submitted: a draft that was ALREADY dirty
   * when the user hit save stays dirty while they keep editing, so nothing in
   * that boolean changes when they pick another companion mid-flight. This key
   * does change, and {@link CompanionRemoteRefresh.withOwnWrite} snapshots it
   * when the write starts — an echo may only reset a draft that still matches
   * the snapshot.
   */
  readonly companionDraftKey?: string;
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
  resetKey,
  hasUnsavedCompanionEdits,
  companionDraftKey,
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
  // When the last own write settled, and when the current draft started
  // differing from the loaded snapshot (0 while it does not). A draft that was
  // born AFTER the save cannot be the one the save wrote, so the echo must not
  // reset it — see the listener below.
  const ownWriteSettledAtRef = useRef(0);
  const dirtySinceRef = useRef(0);
  // The current companion draft and the one the running save submitted. A draft
  // that has moved on from the submitted one is work the echo cannot account
  // for, whether it started before that save or after it.
  const draftKeyRef = useRef(companionDraftKey);
  const submittedDraftKeyRef = useRef<string | undefined>(undefined);
  // Layout effect, not a passive one: the SSE listener below runs in its own
  // task, and passive effects flush after paint — so between committing a
  // render that turned the draft dirty and the passive flush, a notification
  // could still read the PREVIOUS dirty value and refresh the draft away
  // instead of flagging it stale. A layout effect runs synchronously with the
  // commit, before any other task gets to observe these refs.
  useLayoutEffect(() => {
    activeRef.current = active;
    if (hasUnsavedCompanionEdits && !dirtyRef.current) {
      dirtySinceRef.current = Date.now();
    } else if (!hasUnsavedCompanionEdits) {
      dirtySinceRef.current = 0;
    }
    dirtyRef.current = hasUnsavedCompanionEdits;
    draftKeyRef.current = companionDraftKey;
    refreshRef.current = onRefresh;
  });

  // A closed modal keeps its stale flag out of the way: it reloads on open —
  // and so does a view that switched to another child, which loads that child's
  // links from scratch. Both leave nothing the old flag could still describe.
  // The echo grace goes with it: it was armed by a save on the PREVIOUS record,
  // so leaving it running would let it answer for a write it cannot be the echo
  // of.
  useEffect(() => {
    if (active) return;
    setCompanionsStale(false);
    // Disarm the grace with the flag. A save whose echo has not landed by the
    // time the view closes will never be answered here, so leaving the deadline
    // running would let the NEXT genuinely remote announcement — possibly for a
    // draft started after the view was reopened — be consumed as that save's
    // echo, refreshing silently instead of warning.
    ownWriteEchoUntilRef.current = 0;
    ownWriteSettledAtRef.current = 0;
    submittedDraftKeyRef.current = undefined;
  }, [active]);

  useEffect(() => {
    setCompanionsStale(false);
    ownWriteEchoUntilRef.current = 0;
    ownWriteSettledAtRef.current = 0;
    submittedDraftKeyRef.current = undefined;
  }, [resetKey]);

  useEffect(
    () =>
      subscribeStudentCompanionsChanged(() => {
        if (!activeRef.current || ownWriteRef.current) return;
        if (Date.now() < ownWriteEchoUntilRef.current) {
          // The SSE echo of our own save. Consume the grace with it so a
          // genuinely remote write arriving right after is judged normally.
          ownWriteEchoUntilRef.current = 0;
          // ...but only the draft the save actually wrote may be reset by it.
          // The bus carries no correlation id (the announcement is untargeted
          // on purpose — links are symmetric), so an event landing in this
          // window is ATTRIBUTED to our save rather than verified as its echo.
          // A draft that started differing only after the save settled is
          // therefore work this echo cannot account for: resetting it would
          // throw away edits the user made after their save, and a foreign
          // announcement arriving first would do it for somebody else's write.
          // Keep it. The baseline then stays the possibly pre-commit list, but
          // that costs nothing silently: the save carries the fingerprint of
          // exactly that list, so the backend refuses it with 409
          // `companions_changed` (→ markStale) instead of overwriting anyone.
          //
          // The same holds for a draft that was ALREADY dirty when the user
          // saved and that they kept editing while the request was in flight —
          // both forms leave the picker interactive and disable only the save
          // button. Its dirty timestamp is older than the save, so only the
          // draft key catches it: it no longer matches the one the write
          // submitted, so the echo would replace edits it never carried.
          if (
            dirtyRef.current &&
            (dirtySinceRef.current > ownWriteSettledAtRef.current ||
              draftKeyRef.current !== submittedDraftKeyRef.current)
          ) {
            return;
          }
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
      // The draft as it goes out. Anything the user changes from here on is
      // work this write does not carry, so its echo must not reset it — see the
      // listener.
      submittedDraftKeyRef.current = draftKeyRef.current;
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
          const settledAt = Date.now();
          ownWriteSettledAtRef.current = settledAt;
          ownWriteEchoUntilRef.current = settledAt + OWN_WRITE_ECHO_GRACE_MS;
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
