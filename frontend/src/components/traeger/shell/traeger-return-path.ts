"use client";

import { usePathname, useSearchParams } from "next/navigation";
import { useEffect, useSyncExternalStore } from "react";
import { traegerPath } from "~/lib/traeger-url";
import { isTraegerSettingsPath } from "./traeger-nav-active";

/**
 * Der Rückweg aus den Einstellungen (#5685): die zuletzt geöffnete App-Seite
 * dieses Tabs. Kein Recht hängt daran; die Seite prüft ihre Rechte selbst.
 */
const RETURN_PATH_KEY = "traeger-settings-return-path";

function storedReturnPath(): string | null {
  try {
    const stored = globalThis.sessionStorage.getItem(RETURN_PATH_KEY);
    // Nur ein Pfad dieser Seite: "//host" wäre eine fremde Adresse.
    return stored?.startsWith("/") && !stored.startsWith("//") ? stored : null;
  } catch {
    return null;
  }
}

/** Merkt sich die aktuelle Seite, solange sie außerhalb der Einstellungen liegt. */
export function TraegerReturnPathMemory() {
  const pathname = usePathname() ?? "";
  const search = useSearchParams()?.toString() ?? "";
  useEffect(() => {
    if (!pathname || isTraegerSettingsPath(pathname)) return;
    try {
      globalThis.sessionStorage.setItem(
        RETURN_PATH_KEY,
        search ? `${pathname}?${search}` : pathname,
      );
    } catch {
      // Ohne Speicher führt der Rückweg zur Startseite.
    }
  }, [pathname, search]);
  return null;
}

const subscribeNever = () => () => undefined;

/** Das Ziel von „Zurück zur App"; ohne gemerkte Seite die Startseite. */
export function useTraegerReturnHref(): string {
  const start = traegerPath("/traeger");
  return useSyncExternalStore(
    subscribeNever,
    () => storedReturnPath() ?? start,
    () => start,
  );
}
