"use client";

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  type ReactNode,
} from "react";

import {
  getChildCareSchedule,
  getChildFeatures,
  type Child,
  type ChildCareSchedule,
  type ChildFeatures,
} from "~/lib/parent-api";

import { useParentFeatures } from "~/lib/hooks/use-parent-resources";

const SharedFeaturesContext = createContext<ReturnType<
  typeof useParentFeatures
> | null>(null);

export function SharedParentFeaturesProvider({
  children,
  linkedChildren,
}: Readonly<{
  children: ReactNode;
  linkedChildren: readonly Child[] | undefined;
}>) {
  const resource = useParentFeatures(linkedChildren);
  return (
    <SharedFeaturesContext.Provider value={resource}>
      {children}
    </SharedFeaturesContext.Provider>
  );
}

export function useSharedParentFeatures() {
  return useContext(SharedFeaturesContext);
}

interface ChildRequests {
  features: (studentId: string) => Promise<ChildFeatures>;
  careSchedule: (studentId: string) => Promise<ChildCareSchedule>;
}

const DIRECT_REQUESTS: ChildRequests = {
  features: (studentId) => getChildFeatures(studentId),
  careSchedule: (studentId) => getChildCareSchedule(studentId),
};

const ChildRequestsContext = createContext<ChildRequests | null>(null);

interface RunningRequest {
  promise: Promise<unknown>;
  invalidate: () => void;
}

/**
 * Lets the sections of one child page share a GET that is already running.
 * Care status, master data and the care section each need the feature flags,
 * two of them the care schedule; without this every mount and every refresh
 * event would fetch them once per section.
 *
 * Only running requests are shared. A settled one is dropped right away, so
 * every later load (retry, refresh event, focus) fetches fresh data. If a change
 * notification or focus arrives during a request, all callers wait for a fresh
 * read after it settles instead of installing the pre-change result.
 */
export function ChildRequestsProvider({
  children,
}: Readonly<{ children: ReactNode }>) {
  const running = useRef(new Map<string, RunningRequest>());
  useEffect(() => {
    const invalidate = (event: Event) => {
      // In der Capture-Phase kommt auch der Fokus jedes Eingabefelds an. Die
      // Abschnitte laden nur bei Fokus des Fensters neu, also gilt nur der.
      if (event.type === "focus" && event.target instanceof Node) return;
      const studentId = (event as CustomEvent<{ studentId?: string | null }>)
        .detail?.studentId;
      for (const [key, request] of running.current) {
        if (
          !studentId ||
          key === `features:${studentId}` ||
          key === `care-schedule:${studentId}`
        ) {
          request.invalidate();
        }
      }
    };
    // Capture runs before the sections' refresh listeners, regardless of the
    // order React mounted their effects.
    window.addEventListener("parent-conversation-refresh", invalidate, true);
    window.addEventListener("focus", invalidate, true);
    return () => {
      window.removeEventListener(
        "parent-conversation-refresh",
        invalidate,
        true,
      );
      window.removeEventListener("focus", invalidate, true);
    };
  }, []);
  const requests = useMemo<ChildRequests>(() => {
    function share<T>(key: string, load: () => Promise<T>): Promise<T> {
      const pending = running.current.get(key);
      if (pending) return pending.promise as Promise<T>;
      let stale = false;
      const loadFresh = async (): Promise<T> => {
        for (;;) {
          stale = false;
          try {
            const result = await load();
            if (!stale) return result;
          } catch (error) {
            // A notification supersedes errors too; otherwise preserve the
            // existing rejection path for every waiting section.
            if (!stale) throw error;
          }
        }
      };
      const next = loadFresh();
      running.current.set(key, {
        promise: next,
        invalidate: () => {
          stale = true;
        },
      });
      const drop = () => {
        if (running.current.get(key)?.promise === next)
          running.current.delete(key);
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
