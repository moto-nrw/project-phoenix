"use client";

import { useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";

/**
 * One running save of a `FormModal` or `SlideOver`, bound to the dialog
 * session it started in.
 */
export interface FormModalOperation {
  /**
   * True while the dialog that started this operation is still the same:
   * mounted, and neither closed nor reopened since. A late answer that is no
   * longer current must not close the dialog, call parent setters that close
   * or switch dialogs, or show its result in a newer dialog; refreshing the
   * written data stays allowed (#6533).
   */
  readonly isCurrent: () => boolean;
}

/**
 * The pending contract of `FormModal` (#6533) and `SlideOver` (#6593). Pass
 * it as `pending` and run every save through `run`: while it runs, the
 * dialog locks the close X, Escape, the backdrop (on a phone also the swipe)
 * and every footer action, and marks itself `aria-busy`.
 */
export interface FormModalPending {
  readonly pending: boolean;
  /**
   * Runs one save. A second call while one is running in the same session
   * is ignored, and so is a call while the dialog is closed (a click during
   * its closing animation). Closing or unmounting the dialog ends the
   * session: the lock lifts for the next session, and `op.isCurrent()`
   * turns false.
   */
  readonly run: <T>(
    task: (op: FormModalOperation) => Promise<T>,
  ) => Promise<T | undefined>;
}

/**
 * Creates the pending contract for one `FormModal` or `SlideOver`. `isOpen`
 * is the dialog's own `isOpen` / `open`; a dialog that is only mounted while open passes nothing.
 * `context` names what the open dialog works on, as a stable primitive (an
 * id); when it changes while the dialog stays open, a new session starts as
 * on reopening.
 */
export function useFormModalPending(
  isOpen = true,
  context?: unknown,
): FormModalPending {
  const sessionRef = useRef(0);
  const runningSessionRef = useRef<number | null>(null);
  const contextRef = useRef(context);
  const openRef = useRef(isOpen);
  const [pending, setPending] = useState(false);

  // A layout effect, so a session ends in the commit that closes the dialog
  // or switches its context, before a late answer can run against the next.
  useLayoutEffect(() => {
    const switched = !Object.is(contextRef.current, context);
    contextRef.current = context;
    openRef.current = isOpen;
    if (isOpen && !switched) return;
    sessionRef.current += 1;
    runningSessionRef.current = null;
    setPending(false);
  }, [isOpen, context]);

  // Unmounting, or a Suspense boundary hiding the dialog, ends the session;
  // the lock lifts with it so a dialog shown again is never stuck busy.
  useLayoutEffect(
    () => () => {
      sessionRef.current += 1;
      runningSessionRef.current = null;
      setPending(false);
    },
    [],
  );

  const run = useCallback(
    async <T>(
      task: (op: FormModalOperation) => Promise<T>,
    ): Promise<T | undefined> => {
      // A closed dialog has no session to save in: a click that lands
      // during the closing animation must not start a request that the next
      // opening would count as its own.
      if (!openRef.current) return undefined;
      const session = sessionRef.current;
      if (runningSessionRef.current === session) return undefined;
      runningSessionRef.current = session;
      setPending(true);
      const op: FormModalOperation = {
        isCurrent: () => sessionRef.current === session,
      };
      try {
        return await task(op);
      } finally {
        if (op.isCurrent()) {
          runningSessionRef.current = null;
          setPending(false);
        }
      }
    },
    [],
  );

  return useMemo(() => ({ pending, run }), [pending, run]);
}
