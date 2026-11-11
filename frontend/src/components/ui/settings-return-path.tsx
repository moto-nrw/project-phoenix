"use client";

import { usePathname, useSearchParams } from "next/navigation";
import { useEffect, useSyncExternalStore } from "react";
import { isInternalPath } from "./navigation-url";

/**
 * Der Rückweg aus den Einstellungen (#5685, #5750): die zuletzt geöffnete
 * App-Seite dieses Tabs. Jedes Portal hat seinen eigenen Schlüssel, der in
 * `PERSONAL_STORAGE_PREFIXES` steht: Filter in der Adresse können
 * personenbezogen sein. Kein Recht hängt daran; die Seite prüft ihre Rechte
 * selbst.
 */
function storedReturnPath(storageKey: string): string | null {
  try {
    const stored = globalThis.sessionStorage.getItem(storageKey);
    return stored !== null && isInternalPath(stored) ? stored : null;
  } catch {
    return null;
  }
}

/** Merkt sich die aktuelle Seite, solange sie außerhalb der Einstellungen liegt. */
export function SettingsReturnPathMemory({
  storageKey,
  isSettingsPath,
}: {
  readonly storageKey: string;
  readonly isSettingsPath: (pathname: string) => boolean;
}) {
  const pathname = usePathname() ?? "";
  const search = useSearchParams()?.toString() ?? "";
  useEffect(() => {
    if (!pathname || isSettingsPath(pathname)) return;
    try {
      globalThis.sessionStorage.setItem(
        storageKey,
        search ? `${pathname}?${search}` : pathname,
      );
    } catch {
      // Ohne Speicher führt der Rückweg zur Startseite.
    }
  }, [isSettingsPath, pathname, search, storageKey]);
  return null;
}

const subscribeNever = () => () => undefined;

/** Das Ziel von „Zurück zur App"; ohne gemerkte Seite `start`. */
export function useSettingsReturnHref(
  storageKey: string,
  start: string,
): string {
  return useSyncExternalStore(
    subscribeNever,
    () => storedReturnPath(storageKey) ?? start,
    () => start,
  );
}
