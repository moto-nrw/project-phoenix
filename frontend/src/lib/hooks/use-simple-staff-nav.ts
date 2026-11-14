"use client";

import type { Session } from "next-auth";
import { useCallback, useSyncExternalStore } from "react";

import { leadsSchool } from "~/lib/auth-utils";
import {
  useSimpleStaffNavigationSetting,
  useTenantSlugSafe,
} from "~/lib/tenant-context";

/** Der Umschalter unten in der Seitenleiste und im Menü „Mehr“. */
const SIMPLE_STAFF_NAV_SHOW_ALL_LABEL = "Alle Seiten anzeigen";
const SIMPLE_STAFF_NAV_SHOW_SIMPLE_LABEL = "Weniger Seiten anzeigen";
const SIMPLE_STAFF_NAV_SHOW_ALL_TOOLTIP =
  "Zeigt im Menü alle Seiten, die Sie öffnen dürfen.";
const SIMPLE_STAFF_NAV_SHOW_SIMPLE_TOOLTIP =
  "Zeigt im Menü nur die Seiten für den Betreuungsalltag.";

const STORAGE_PREFIX = "simple-staff-nav-show-all";
const SHOW_ALL = "1";

// Failed writes are newer than the persisted value and must be shared by
// every menu in this document, including menus mounted after the choice.
const memoryValues = new Map<string, boolean>();
const getServerSnapshot = () => false;

function readShowAll(key: string): boolean {
  const memory = memoryValues.get(key);
  if (memory !== undefined) return memory;
  try {
    return globalThis.localStorage.getItem(key) === SHOW_ALL;
  } catch {
    return false;
  }
}

function subscribe(onStoreChange: () => void) {
  globalThis.addEventListener("storage", onStoreChange);
  return () => globalThis.removeEventListener("storage", onStoreChange);
}

function writeShowAll(key: string, showAll: boolean) {
  try {
    if (showAll) {
      globalThis.localStorage.setItem(key, SHOW_ALL);
    } else {
      globalThis.localStorage.removeItem(key);
    }
    memoryValues.delete(key);
  } catch {
    // Ohne Speicher (privater Modus, volle Quota) gilt die Wahl nur, bis die
    // Seite neu lädt.
    memoryValues.set(key, showAll);
  }
  // `storage` feuert nur in anderen Tabs. Seitenleiste und mobile Leiste
  // derselben Seite sollen sofort folgen, deshalb auch hier.
  globalThis.dispatchEvent(new StorageEvent("storage", { key }));
}

export interface SimpleStaffNav {
  /** Die Schule hat die einfache Seitenleiste an, und die Person leitet sie nicht. */
  readonly available: boolean;
  /** Die Person sieht gerade die einfache Seitenleiste. */
  readonly active: boolean;
  readonly showAllPages: () => void;
  readonly showSimplePages: () => void;
  /** Der Umschalter unten: wechselt in die jeweils andere Ansicht. */
  readonly toggle: () => void;
  readonly toggleLabel: string;
  /** Was der Umschalter bewirkt (#6643). */
  readonly toggleTooltip: string;
}

/**
 * Ob eine Person die einfache Seitenleiste sieht (#6956).
 *
 * Die Schule schaltet sie ein (`operations.simple_staff_navigation`); sie
 * gilt für alle, die die Schule nicht leiten (`leadsSchool`, am Recht
 * festgemacht wie Hilfe und Startseite). Jede Person kann „Alle Seiten
 * anzeigen“ wählen. Das merkt sich der Browser je Schule und Konto, damit
 * eine Kollegin am selben iPad ihre eigene Ansicht behält.
 *
 * Die Wahl ändert nur die Navigation, nie ein Recht. Der erste Render kennt
 * den Speicher nicht und zeigt die einfache Ansicht; das ist die Ansicht,
 * die die Schule gewählt hat.
 */
export function useSimpleStaffNav(session: Session | null): SimpleStaffNav {
  const schoolSetting = useSimpleStaffNavigationSetting();
  const tenantSlug = useTenantSlugSafe();
  const accountId = session?.user?.id;
  const available =
    schoolSetting && Boolean(accountId) && !leadsSchool(session);
  const key = `${STORAGE_PREFIX}:${tenantSlug ?? ""}:${accountId ?? ""}`;
  const getSnapshot = useCallback(
    () => available && readShowAll(key),
    [available, key],
  );
  const showAll = useSyncExternalStore(
    subscribe,
    getSnapshot,
    getServerSnapshot,
  );

  const showAllPages = useCallback(() => writeShowAll(key, true), [key]);
  const showSimplePages = useCallback(() => writeShowAll(key, false), [key]);

  const active = available && !showAll;
  return {
    available,
    active,
    showAllPages,
    showSimplePages,
    toggle: active ? showAllPages : showSimplePages,
    toggleLabel: active
      ? SIMPLE_STAFF_NAV_SHOW_ALL_LABEL
      : SIMPLE_STAFF_NAV_SHOW_SIMPLE_LABEL,
    toggleTooltip: active
      ? SIMPLE_STAFF_NAV_SHOW_ALL_TOOLTIP
      : SIMPLE_STAFF_NAV_SHOW_SIMPLE_TOOLTIP,
  };
}
