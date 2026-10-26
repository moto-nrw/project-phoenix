"use client";

import { useMemo } from "react";
import { useSession } from "next-auth/react";
import {
  mayWritePersonalStorage,
  personalStorageOwner,
} from "./personal-storage";

/**
 * Was sich die Seitenleiste merkt, damit ein Klick auf „Gruppen“ oder
 * „Aktuelle Aufsicht“ dort weitermacht, wo die Person war: Gruppe und Raum
 * samt Namen, die laufende Aufsicht und die zuletzt offene Seite der
 * Datenverwaltung.
 */
export const SIDEBAR_MEMORY_ENTRIES = [
  "sidebar-last-group",
  "sidebar-last-group-name",
  "sidebar-last-room",
  "sidebar-last-room-name",
  "supervision-last-session",
  "sidebar-last-database",
] as const;

export type SidebarMemoryEntry = (typeof SIDEBAR_MEMORY_ENTRIES)[number];

export interface SidebarMemory {
  /** Schule und Konto, für die dieses Gedächtnis schreibt. */
  readonly owner: string | null;
  /** Schlüssel im localStorage; null, solange niemand angemeldet ist. */
  readonly key: (entry: SidebarMemoryEntry) => string | null;
  readonly read: (entry: SidebarMemoryEntry) => string | null;
  readonly write: (entry: SidebarMemoryEntry, value: string) => void;
  readonly remove: (entry: SidebarMemoryEntry) => void;
}

/**
 * Schlüssel je Schule und Konto (#5000): Auf einem geteilten Gerät übernimmt
 * ein anderes Konto oder dieselbe Person an einer anderen Schule keine
 * Vorauswahl. Die Einträge stehen im Inventar von personal-storage.ts und
 * verschwinden beim Logout.
 */
export function sidebarMemoryKey(
  entry: SidebarMemoryEntry,
  owner: string,
): string {
  return `${entry}:${owner}`;
}

function sidebarMemory(owner: string | null): SidebarMemory {
  const key = (entry: SidebarMemoryEntry) =>
    owner === null ? null : sidebarMemoryKey(entry, owner);
  return {
    owner,
    key,
    read: (entry) => {
      const storageKey = key(entry);
      if (storageKey === null) return null;
      try {
        return globalThis.localStorage.getItem(storageKey);
      } catch {
        // Gesperrter Speicher: die Seite startet ohne gemerkte Auswahl.
        return null;
      }
    },
    write: (entry, value) => {
      const storageKey = key(entry);
      // Nach einem Logout oder Kontowechsel schreibt das Gedächtnis des
      // vorigen Kontos nicht mehr, auch nicht aus einer späten Antwort oder
      // einem anderen Tab (#5000).
      if (storageKey === null || !mayWritePersonalStorage(owner)) return;
      try {
        globalThis.localStorage.setItem(storageKey, value);
      } catch {
        // Ohne Speicher merkt sich die Seitenleiste nur nichts.
      }
    },
    remove: (entry) => {
      const storageKey = key(entry);
      if (storageKey === null) return;
      try {
        globalThis.localStorage.removeItem(storageKey);
      } catch {
        // Gesperrter Speicher hält ohnehin nichts fest.
      }
    },
  };
}

/** Das Gedächtnis der Seitenleiste für das angemeldete Konto. */
export function useSidebarMemory(): SidebarMemory {
  const { data: session } = useSession();
  const owner = personalStorageOwner(session?.user);
  return useMemo(() => sidebarMemory(owner), [owner]);
}
