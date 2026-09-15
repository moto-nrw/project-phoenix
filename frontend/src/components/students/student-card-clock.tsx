"use client";

// components/students/student-card-clock.tsx
// Minute clock for the arrival/pickup rows of a whole card list (#2975).

import { createContext, useContext, type ReactNode } from "react";

/**
 * The time rows colour themselves by how far the planned time is away, so they
 * need a clock that ticks every minute. Handing that clock down as a prop makes
 * the tick a prop change on every card, which defeats `React.memo` and
 * re-renders all 100 Kinderkarten once a minute. Reading it from context keeps
 * the tick inside the rows: the cards skip, only the time rows re-render.
 *
 * A page that renders one card, or passes `now` explicitly, needs no provider.
 * Deliberately its own module so the many test files that mock
 * `~/components/students/student-card` do not have to know about it.
 */
const StudentCardClockContext = createContext<Date | null>(null);

export function StudentCardClockProvider({
  now,
  children,
}: Readonly<{ now: Date; children: ReactNode }>) {
  return (
    <StudentCardClockContext.Provider value={now}>
      {children}
    </StudentCardClockContext.Provider>
  );
}

/**
 * An explicit `now` prop always wins; without one the row follows the list's
 * clock. Falling back to the wall clock keeps a stray row honest rather than
 * silently freezing it at some historical time.
 */
export function useRowClock(explicitNow: Date | undefined): Date {
  const listClock = useContext(StudentCardClockContext);
  return explicitNow ?? listClock ?? new Date();
}
