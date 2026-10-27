"use client";

import {
  createContext,
  useContext,
  useMemo,
  useRef,
  type ReactNode,
} from "react";

import {
  getChildCareSchedule,
  getChildFeatures,
  type ChildCareSchedule,
  type ChildFeatures,
} from "~/lib/parent-api";

interface ChildRequests {
  features: (studentId: string) => Promise<ChildFeatures>;
  careSchedule: (studentId: string) => Promise<ChildCareSchedule>;
}

const DIRECT_REQUESTS: ChildRequests = {
  features: (studentId) => getChildFeatures(studentId),
  careSchedule: (studentId) => getChildCareSchedule(studentId),
};

const ChildRequestsContext = createContext<ChildRequests | null>(null);

/**
 * Lets the sections of one child page share a GET that is already running.
 * Care status, master data and the care section each need the feature flags,
 * two of them the care schedule; without this every mount and every refresh
 * event would fetch them once per section.
 *
 * Only running requests are shared. A settled one is dropped right away, so
 * every later load (retry, refresh event, focus) fetches fresh data.
 */
export function ChildRequestsProvider({
  children,
}: Readonly<{ children: ReactNode }>) {
  const running = useRef(new Map<string, Promise<unknown>>());
  const requests = useMemo<ChildRequests>(() => {
    function share<T>(key: string, load: () => Promise<T>): Promise<T> {
      const pending = running.current.get(key);
      if (pending) return pending as Promise<T>;
      const next = load();
      running.current.set(key, next);
      const drop = () => {
        if (running.current.get(key) === next) running.current.delete(key);
      };
      void next.then(drop, drop);
      return next;
    }
    return {
      features: (studentId) =>
        share(`features:${studentId}`, () => getChildFeatures(studentId)),
      careSchedule: (studentId) =>
        share(`care-schedule:${studentId}`, () =>
          getChildCareSchedule(studentId),
        ),
    };
  }, []);
  return (
    <ChildRequestsContext.Provider value={requests}>
      {children}
    </ChildRequestsContext.Provider>
  );
}

/**
 * The child requests of the surrounding page, or direct fetches outside a
 * provider (tests, the conversation view).
 */
export function useChildRequests(): ChildRequests {
  return useContext(ChildRequestsContext) ?? DIRECT_REQUESTS;
}
