"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";

import {
  getRequestSharingOptions,
  type RequestSharingState,
} from "~/lib/parent-api";

type SharingOptionsLoader = (studentId: string) => Promise<RequestSharingState>;

const SharingOptionsContext = createContext<SharingOptionsLoader | null>(null);

/**
 * Holds one in-flight/settled request per child, so several sharing selectors
 * on the same page share a single GET instead of one each. A failed load is
 * dropped from the cache, so the next mount retries instead of repeating the
 * error forever.
 */
export function SharingOptionsProvider({
  children,
}: Readonly<{ children: ReactNode }>) {
  const cache = useRef(new Map<string, Promise<RequestSharingState>>());
  const load = useCallback<SharingOptionsLoader>((studentId) => {
    const cached = cache.current.get(studentId);
    if (cached) return cached;
    const pending = getRequestSharingOptions(studentId);
    cache.current.set(studentId, pending);
    pending.catch(() => cache.current.delete(studentId));
    return pending;
  }, []);
  return (
    <SharingOptionsContext.Provider value={load}>
      {children}
    </SharingOptionsContext.Provider>
  );
}

/**
 * Loads the guardians a request may be shared with. Uses the page-wide cache
 * when a provider is mounted, otherwise fetches directly, so the selector
 * works in isolation (tests, standalone dialogs) too.
 */
export function useSharingOptions(studentId: string): {
  state: RequestSharingState | null;
  error: boolean;
} {
  const contextLoad = useContext(SharingOptionsContext);
  const load = contextLoad ?? getRequestSharingOptions;
  const [state, setState] = useState<RequestSharingState | null>(null);
  const [error, setError] = useState(false);
  useEffect(() => {
    let active = true;
    setState(null);
    setError(false);
    void (async () => {
      try {
        const next = await load(studentId);
        if (active) setState(next);
      } catch {
        if (active) setError(true);
      }
    })();
    return () => {
      active = false;
    };
  }, [load, studentId]);
  return useMemo(() => ({ state, error }), [error, state]);
}
