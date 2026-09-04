"use client";

import { useLinkStatus } from "next/link";
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useSyncExternalStore,
  type ReactNode,
} from "react";

/**
 * Fortschrittsanzeige für Seitenwechsel (#2828).
 *
 * Ohne Ladehülle bleibt beim Seitenwechsel die aktuelle Seite stehen, bis die
 * neue bereit ist. Das ist ruhig, aber ohne Rückmeldung: ein Klick, und für
 * einen Moment passiert sichtbar nichts. Der Balken schließt genau diese
 * Lücke — er ersetzt keine Seite, er liegt als 3 Pixel hohe Linie über dem
 * Kopfbereich.
 *
 * Woher der Zustand kommt: `useLinkStatus` meldet für jeden `NavLink`, ob
 * seine Navigation noch aussteht. Für vorgeladene Ziele (NavLink lädt bei
 * Hover, Fokus oder Touch-Beginn vor) überspringt Next diesen Zustand ganz —
 * dann erscheint der Balken gar nicht erst, weil der Wechsel sofort passiert.
 *
 * Warum ein eigener Store statt `useState`: der Anbieter umschließt die
 * gesamte Hülle. Ein State-Wechsel dort würde Kopfzeile, Seitenleiste und
 * Inhalt bei jedem Klick neu rendern. So rendert nur der Balken selbst neu.
 */

interface NavigationProgressStore {
  readonly subscribe: (onChange: () => void) => () => void;
  readonly isPending: () => boolean;
  readonly start: () => void;
  readonly end: () => void;
}

function createStore(): NavigationProgressStore {
  let pending = 0;
  const listeners = new Set<() => void>();
  const notify = () => {
    for (const listener of listeners) listener();
  };
  return {
    subscribe: (onChange) => {
      listeners.add(onChange);
      return () => {
        listeners.delete(onChange);
      };
    },
    isPending: () => pending > 0,
    start: () => {
      pending += 1;
      notify();
    },
    end: () => {
      pending = Math.max(0, pending - 1);
      notify();
    },
  };
}

const NavigationProgressContext = createContext<NavigationProgressStore | null>(
  null,
);

/**
 * Umschließt die Hülle eines Portals. Außerhalb davon melden `NavLink`s
 * nichts und der Balken erscheint nie — Tests und Stories brauchen den
 * Anbieter deshalb nicht.
 */
export function NavigationProgressProvider({
  children,
}: {
  readonly children: ReactNode;
}) {
  const store = useMemo(createStore, []);
  return (
    <NavigationProgressContext.Provider value={store}>
      {children}
    </NavigationProgressContext.Provider>
  );
}

/**
 * Meldet den ausstehenden Seitenwechsel eines Links. Rendert nichts und muss
 * innerhalb eines `next/link` stehen — `NavLink` setzt sie dort ein.
 */
export function NavigationProgressReporter() {
  const store = useContext(NavigationProgressContext);
  const { pending } = useLinkStatus();

  useEffect(() => {
    if (!store || !pending) return;
    store.start();
    return store.end;
  }, [store, pending]);

  return null;
}

const NOT_PENDING = () => false;
const NO_SUBSCRIPTION = () => () => undefined;

/**
 * Der Balken selbst. Er liegt fest am oberen Rand, verschiebt nichts und
 * erscheint erst nach 150 ms — kurze Wechsel bleiben dadurch unsichtbar
 * ruhig, statt kurz aufzublitzen.
 */
export function NavigationProgressBar() {
  const store = useContext(NavigationProgressContext);
  const pending = useSyncExternalStore(
    store?.subscribe ?? NO_SUBSCRIPTION,
    store?.isPending ?? NOT_PENDING,
    NOT_PENDING,
  );

  return (
    <>
      {/* Dauerhaft im Baum, damit Screenreader die Änderung überhaupt
          vorlesen; ein erst beim Wechsel eingehängter Bereich wird nicht
          angesagt. */}
      <span role="status" aria-live="polite" className="sr-only">
        {pending ? "Seite wird geladen" : ""}
      </span>
      {pending ? (
        <div
          data-testid="navigation-progress"
          className="moto-nav-progress"
          aria-hidden="true"
        />
      ) : null}
    </>
  );
}
