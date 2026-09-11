"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import {
  getStaffNavGroupForPathname,
  STAFF_NAV_DEFAULT_OPEN_GROUPS,
  STAFF_NAV_GROUPS,
  type StaffNavGroupKey,
} from "~/lib/staff-navigation";

const STORAGE_KEY = "sidebar-open-groups";

const KNOWN_GROUP_KEYS = new Set<string>(
  STAFF_NAV_GROUPS.map((group) => group.key),
);

function isGroupKey(value: unknown): value is StaffNavGroupKey {
  return typeof value === "string" && KNOWN_GROUP_KEYS.has(value);
}

function readStoredGroups(): readonly StaffNavGroupKey[] | null {
  try {
    const raw = globalThis.localStorage.getItem(STORAGE_KEY);
    if (raw === null) return null;
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return null;
    return parsed.filter(isGroupKey);
  } catch {
    return null;
  }
}

function writeStoredGroups(groups: readonly StaffNavGroupKey[]) {
  try {
    globalThis.localStorage.setItem(STORAGE_KEY, JSON.stringify(groups));
  } catch {
    // Ohne Speicher (privater Modus, volle Quota) gilt der Zustand nur für
    // diese Sitzung.
  }
}

function withGroup(
  groups: readonly StaffNavGroupKey[],
  group: StaffNavGroupKey | null,
): readonly StaffNavGroupKey[] {
  if (!group || groups.includes(group)) return groups;
  return [...groups, group];
}

/**
 * Welche Gruppen der Seitenleiste offen stehen (#2826).
 *
 * Mehrere Gruppen können gleichzeitig offen sein; jede klappt unabhängig.
 * Beim ersten Besuch steht nur der Tagesbetrieb offen. Was die Person
 * auf- oder zuklappt, merkt sich der Browser (`sidebar-open-groups`). Die
 * Gruppe der aktuellen Seite öffnet sich von selbst, ohne die anderen zu
 * schließen; ein Seitenwechsel schließt nichts — sonst müsste man nach
 * jedem Klick wieder aufklappen, was man gerade gebraucht hat.
 *
 * Der erste Render kennt den Speicher nicht (Server und Client müssen
 * dasselbe Bild liefern), deshalb wird der gespeicherte Stand erst nach dem
 * Einhängen übernommen — dasselbe Muster wie useSidebarAccordion.
 *
 * Geschrieben wird nur bei einer echten Änderung (Klick, Auto-Aufklappen,
 * Übernahme des Speichers), nie aus einem Effekt, der den Zustand
 * beobachtet: der liefe beim Einhängen mit dem Standard los und überschriebe
 * den gespeicherten Stand, bevor er gelesen ist (Reacts doppeltes Einhängen
 * im Entwicklungsmodus macht genau das sichtbar).
 */
export function useSidebarGroups(pathname: string, fromParam?: string | null) {
  const currentGroup = getStaffNavGroupForPathname(pathname, fromParam);
  const [openGroups, setOpenGroups] = useState<readonly StaffNavGroupKey[]>(
    () => withGroup(STAFF_NAV_DEFAULT_OPEN_GROUPS, currentGroup),
  );
  // Der jeweils letzte Stand, auch zwischen zwei Renderings: die Effekte
  // unten dürfen nicht auf einem veralteten `openGroups` aufsetzen.
  const latest = useRef(openGroups);

  const apply = useCallback((next: readonly StaffNavGroupKey[]) => {
    latest.current = next;
    setOpenGroups(next);
    writeStoredGroups(next);
  }, []);

  useEffect(() => {
    const stored = readStoredGroups();
    if (!stored) return;
    // Der gespeicherte Stand ersetzt den Standard; die Gruppe der aktuellen
    // Seite bleibt in jedem Fall offen.
    apply(withGroup(stored, getStaffNavGroupForPathname(pathname, fromParam)));
    // Nur einmal nach dem Einhängen.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!currentGroup || latest.current.includes(currentGroup)) return;
    apply(withGroup(latest.current, currentGroup));
  }, [currentGroup, apply]);

  const toggleGroup = useCallback(
    (group: StaffNavGroupKey) => {
      const current = latest.current;
      apply(
        current.includes(group)
          ? current.filter((key) => key !== group)
          : [...current, group],
      );
    },
    [apply],
  );

  const isGroupOpen = useCallback(
    (group: StaffNavGroupKey) => openGroups.includes(group),
    [openGroups],
  );

  return { openGroups, isGroupOpen, toggleGroup };
}
