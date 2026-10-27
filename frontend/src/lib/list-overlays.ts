"use client";

/**
 * Lokale Änderungen an gecachten Listen (#4988), ohne den SWR-Cache zu
 * verändern.
 *
 * Der Cache einer Liste hält nur Antworten des Servers. Eine Entscheidung,
 * die eine Zeile entfernt oder ändert, wird hier als Überlagerung für die
 * ganze Liste vermerkt: für jede Ansicht und jeden Filter derselben Schule
 * und desselben Kontos. Sie gilt nur für Daten, deren Abruf vor der
 * Entscheidung begann; jede spätere Antwort des Servers kennt die Entscheidung
 * schon und gilt unverändert.
 *
 * Weil der Cache nie verändert wird, verwirft SWR keinen laufenden Abruf, und
 * kein Wechsel zwischen Filtern kann eine Antwort verlieren oder eine Liste im
 * Ladezustand stehen lassen. Die Reihenfolge stiftet ein Zähler im Speicher
 * des Tabs; ein Neuladen der Seite beginnt ohne Überlagerungen.
 */

import { useCallback, useSyncExternalStore } from "react";

export interface ListOverlay<T> {
  /** Nach allen Abrufen vergeben, die vor der Änderung begannen. */
  readonly seq: number;
  readonly update: (items: T[]) => T[];
}

const NONE: readonly never[] = [];

let counter = 0;
const overlays = new Map<string, readonly ListOverlay<unknown>[]>();
const listeners = new Map<string, Set<() => void>>();

/** Vor jedem Abruf einer Liste: seine Nummer in der Reihenfolge. */
export function startListFetch(): number {
  counter += 1;
  return counter;
}

/** Vermerkt eine Änderung für alle Ansichten und Filter der Liste `list`. */
export function addListOverlay<T>(
  list: string,
  update: (items: T[]) => T[],
): void {
  counter += 1;
  // Auch alte Cache-Stände bleiben im Tab erreichbar. Ohne deren Entwertung
  // darf keine Überlagerung allein wegen ihres Alters oder der Anzahl wegfallen.
  const next = [
    ...(overlays.get(list) ?? NONE),
    { seq: counter, update } as ListOverlay<unknown>,
  ];
  overlays.set(list, next);
  for (const listener of listeners.get(list) ?? []) listener();
}

/**
 * Wendet die Änderungen an, die nach dem Abruf `fetchedAt` vermerkt wurden.
 * `apply` setzt eine Änderung in den Zustand der Liste um.
 */
export function applyListOverlays<T, S>(
  state: S,
  fetchedAt: number,
  list: readonly ListOverlay<T>[],
  apply: (state: S, update: (items: T[]) => T[]) => S,
): S {
  let result = state;
  for (const overlay of list) {
    if (overlay.seq > fetchedAt) result = apply(result, overlay.update);
  }
  return result;
}

/** Die Änderungen der Liste `list`; rendert neu, sobald eine dazukommt. */
export function useListOverlays<T>(list: string): readonly ListOverlay<T>[] {
  const subscribe = useCallback(
    (listener: () => void) => {
      const set = listeners.get(list) ?? new Set();
      set.add(listener);
      listeners.set(list, set);
      return () => {
        set.delete(listener);
        if (set.size === 0) listeners.delete(list);
      };
    },
    [list],
  );
  const read = useCallback(
    () => (overlays.get(list) ?? NONE) as readonly ListOverlay<T>[],
    [list],
  );
  return useSyncExternalStore(subscribe, read, () => NONE);
}
