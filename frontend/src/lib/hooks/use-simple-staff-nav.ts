"use client";

import type { Session } from "next-auth";
import { useCallback } from "react";

import { leadsSchool } from "~/lib/auth-utils";
import { useLocalStorageValue } from "~/lib/hooks/use-local-storage-value";
import {
  useSimpleStaffNavigationSetting,
  useTenantSlugSafe,
} from "~/lib/tenant-context";

/** Der Umschalter unten in der Seitenleiste und im Menü „Mehr“. */
export const SIMPLE_STAFF_NAV_SHOW_ALL_LABEL = "Alle Seiten anzeigen";
export const SIMPLE_STAFF_NAV_SHOW_SIMPLE_LABEL = "Weniger Seiten anzeigen";

const STORAGE_PREFIX = "simple-staff-nav-show-all";
const SHOW_ALL = "1";

function writeShowAll(key: string, showAll: boolean) {
  try {
    if (showAll) {
      globalThis.localStorage.setItem(key, SHOW_ALL);
    } else {
      globalThis.localStorage.removeItem(key);
    }
  } catch {
    // Ohne Speicher (privater Modus, volle Quota) gilt die Wahl nur, bis die
    // Seite neu lädt.
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
  const showAll = useLocalStorageValue(key, available) === SHOW_ALL;

  const showAllPages = useCallback(() => writeShowAll(key, true), [key]);
  const showSimplePages = useCallback(() => writeShowAll(key, false), [key]);

  return {
    available,
    active: available && !showAll,
    showAllPages,
    showSimplePages,
  };
}
