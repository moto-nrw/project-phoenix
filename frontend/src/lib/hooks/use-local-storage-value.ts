"use client";

import { useCallback, useSyncExternalStore } from "react";

const getServerSnapshot = () => null;

/**
 * Reads a localStorage key without making the server and first client render
 * disagree. Cross-document storage events keep an already-mounted consumer in
 * sync when another tab changes the value.
 */
export function useLocalStorageValue(
  key: string,
  enabled = true,
): string | null {
  const subscribe = useCallback(
    (onStoreChange: () => void) => {
      if (!enabled) return () => undefined;
      const handleStorage = (event: StorageEvent) => {
        if (event.key === null || event.key === key) onStoreChange();
      };
      globalThis.addEventListener("storage", handleStorage);
      return () => globalThis.removeEventListener("storage", handleStorage);
    },
    [enabled, key],
  );

  const getSnapshot = useCallback(() => {
    if (!enabled) return null;
    try {
      return globalThis.localStorage.getItem(key);
    } catch {
      return null;
    }
  }, [enabled, key]);

  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
