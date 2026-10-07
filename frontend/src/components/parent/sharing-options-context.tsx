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
import { useTranslations } from "next-intl";

import type { FormError } from "~/components/ui/form-error";
import { useApiLoadError } from "~/contexts/ToastContext";
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
 *
 * A failed load goes through the shared error path (#2518): `error` carries
 * the localized message with retry, `failed` tells the selector that no
 * choice is possible right now.
 */
export function useSharingOptions(studentId: string): {
  state: RequestSharingState | null;
  failed: boolean;
  error: FormError | null;
} {
  const t = useTranslations("parentRequestSharing");
  const contextLoad = useContext(SharingOptionsContext);
  const load = contextLoad ?? getRequestSharingOptions;
  const [state, setState] = useState<RequestSharingState | null>(null);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const { error, show, clear } = useApiLoadError();
  useEffect(() => {
    let active = true;
    setState(null);
    setFailed(false);
    clear();
    void (async () => {
      try {
        const next = await load(studentId);
        if (active) setState(next);
      } catch (err) {
        if (!active) return;
        setFailed(true);
        void show(err, {
          object: t("errorObjectRecipients"),
          messageSuffix: t("optionsHint"),
          retry: () => setAttempt((current) => current + 1),
        });
      }
    })();
    return () => {
      active = false;
    };
    // `attempt` re-runs the load after a retry; the provider has dropped the
    // failed request from its cache by then.
  }, [attempt, clear, load, show, studentId, t]);
  return useMemo(() => ({ state, failed, error }), [error, failed, state]);
}
