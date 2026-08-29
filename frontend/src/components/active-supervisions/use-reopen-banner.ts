"use client";

import { useCallback, useEffect, useState } from "react";

const REOPEN_STORAGE_KEY = "timetable-reopenable-instance";
const REOPEN_WINDOW_MS = 5 * 60 * 1000;

function readStoredReopenBanner(): {
  instanceId: string;
  expiresAt: number;
} | null {
  const raw = window.sessionStorage.getItem(REOPEN_STORAGE_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as {
      instanceId?: string;
      expiresAt?: string | number;
    };
    if (!parsed.instanceId || parsed.expiresAt == null) {
      window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
      return null;
    }
    const expiresAt =
      typeof parsed.expiresAt === "number"
        ? parsed.expiresAt
        : Date.parse(parsed.expiresAt);
    if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) {
      window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
      return null;
    }
    return { instanceId: parsed.instanceId, expiresAt };
  } catch {
    window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
    return null;
  }
}

/**
 * Tracks the recently completed timetable instance that can still be
 * reopened. The window survives a page reload via sessionStorage and expires
 * itself: after `reopenUntil` (fallback: five minutes) the banner disappears
 * without a render from outside.
 */
export function useReopenBanner(): {
  reopenableInstanceId: string | null;
  /**
   * Remember a just-completed instance. `reopenUntil` is the backend's
   * ISO deadline; an absent or invalid value falls back to now + 5 minutes,
   * an already-expired one clears the banner instead.
   */
  rememberReopenable: (
    instanceId: string,
    reopenUntil: string | null | undefined,
  ) => void;
  clearReopenable: () => void;
} {
  const [storedReopen, setStoredReopen] = useState<{
    instanceId: string;
    expiresAt: number;
  } | null>(null);

  useEffect(() => {
    setStoredReopen(readStoredReopenBanner());
  }, []);

  useEffect(() => {
    if (!storedReopen) return;
    const remainingMs = storedReopen.expiresAt - Date.now();
    if (remainingMs <= 0) {
      window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
      setStoredReopen(null);
      return;
    }
    const timeoutId = window.setTimeout(() => {
      window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
      setStoredReopen(null);
    }, remainingMs);
    return () => window.clearTimeout(timeoutId);
  }, [storedReopen]);

  const clearReopenable = useCallback(() => {
    window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
    setStoredReopen(null);
  }, []);

  const rememberReopenable = useCallback(
    (instanceId: string, reopenUntil: string | null | undefined) => {
      const expiresAt = reopenUntil
        ? Date.parse(reopenUntil)
        : Date.now() + REOPEN_WINDOW_MS;
      if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) {
        window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
        setStoredReopen(null);
        return;
      }
      window.sessionStorage.setItem(
        REOPEN_STORAGE_KEY,
        JSON.stringify({ instanceId, expiresAt }),
      );
      setStoredReopen({ instanceId, expiresAt });
    },
    [],
  );

  return {
    reopenableInstanceId: storedReopen?.instanceId ?? null,
    rememberReopenable,
    clearReopenable,
  };
}
