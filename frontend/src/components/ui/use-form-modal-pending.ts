"use client";

import { useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";

/** One running save of a `FormModal`, bound to the dialog session it started in. */
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
 * The pending contract of `FormModal` (#6533). Pass it as `pending` and run
 * every save through `run`: while it runs, `FormModal` locks the close X,
 * Escape, the backdrop and every footer action, and marks the dialog
 * `aria-busy`.
 */
export interface FormModalPending {
  readonly pending: boolean;
  /**
   * Runs one save. A second call while one is running in the same session
   * is ignored. Closing or unmounting the dialog ends the session: the lock
   * lifts for the next session, and `op.isCurrent()` turns false.
   */
  readonly run: <T>(
    task: (op: FormModalOperation) => Promise<T>,
  ) => Promise<T | undefined>;
}

/**
 * Creates the pending contract for one `FormModal`. `isOpen` is the dialog's
 * own `isOpen`; a dialog that is only mounted while open passes nothing.
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
  const [pending, setPending] = useState(false);

  // A layout effect, so a session ends in the commit that closes the dialog
  // or switches its context, before a late answer can run against the next.
  useLayoutEffect(() => {
    const switched = !Object.is(contextRef.current, context);
    contextRef.current = context;
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
